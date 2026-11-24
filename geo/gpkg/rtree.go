package gpkg

import (
	"encoding/binary"
	"fmt"
	"math"
)

// BuildRTreeIndex 从既有要素的 GPKG 二进制头 envelope 回填 rtree_<table>_<geom> 虚拟表。
//
// 说明：OGC 规范的空间索引触发器依赖 ST_MinX/ST_MaxX 等 spatialite 函数，
// 而 modernc.org/sqlite（纯 Go，无 cgo）无法加载 spatialite，
// 因此采用批量回填方式：逐行读取几何 blob，在 Go 侧解析 envelope 写入 rtree。
// 对已关闭写入的数据（如下载归档）只需调用一次。
func (w *Writer) BuildRTreeIndex(tableName, geomCol string) error {
	rtree := fmt.Sprintf("rtree_%s_%s", tableName, geomCol)

	rows, err := w.db.Query(fmt.Sprintf(`SELECT fid, %s FROM %s`, geomCol, tableName))
	if err != nil {
		return fmt.Errorf("读取要素失败: %w", err)
	}
	defer rows.Close()

	type entry struct {
		fid                    int64
		minX, maxX, minY, maxY float64
	}
	var entries []entry

	for rows.Next() {
		var fid int64
		var blob []byte
		if err := rows.Scan(&fid, &blob); err != nil {
			return err
		}
		env, err := envelopeFromGpkgBlob(blob)
		if err != nil {
			return fmt.Errorf("fid=%d envelope 解析失败: %w", fid, err)
		}
		entries = append(entries, entry{fid: fid, minX: env[0], maxX: env[1], minY: env[2], maxY: env[3]})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// 先清空再回填，保证幂等
	if _, err := w.db.Exec(fmt.Sprintf(`DELETE FROM %s`, rtree)); err != nil {
		return err
	}
	stmt, err := w.db.Prepare(fmt.Sprintf(`INSERT OR REPLACE INTO %s (id, minX, maxX, minY, maxY) VALUES (?,?,?,?,?)`, rtree))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.fid, e.minX, e.maxX, e.minY, e.maxY); err != nil {
			return err
		}
	}
	return nil
}

// envelopeFromGpkgBlob 解析 GPKG 二进制头的 XY envelope：[minX, maxX, minY, maxY]
//
// 头布局：magic(2B "GP") + version(1B) + flags(1B) + srs_id(4B) + envelope
// envelope 内容与长度由 flags 的 bit1-3 指示（0=无, 1=XY, 2=XYZ, 3=XYM, 4=XYZM），
// 均为小端 float64（flags bit0=1 小端；本库写入端固定小端）。
func envelopeFromGpkgBlob(blob []byte) ([4]float64, error) {
	var env [4]float64

	if len(blob) < 8 {
		return env, fmt.Errorf("blob 过短（%d 字节），非 GPKG 几何", len(blob))
	}
	if blob[0] != 0x47 || blob[1] != 0x50 {
		return env, fmt.Errorf("缺少 GP magic，非 GPKG 几何")
	}
	flags := blob[3]
	if flags&0x01 == 0 {
		return env, fmt.Errorf("不支持大端 GPKG 几何")
	}

	envIndicator := (flags >> 1) & 0x07
	// envelope 起始偏移固定为 8；各指示位对应的 float64 个数
	envFloats := map[byte]int{0: 0, 1: 4, 2: 6, 3: 6, 4: 8}[envIndicator]
	if envFloats == 0 && envIndicator != 0 {
		return env, fmt.Errorf("非法 envelope 指示位: %d", envIndicator)
	}
	if envFloats == 0 {
		// 无 envelope：按空包围盒处理
		return [4]float64{math.MaxFloat64, -math.MaxFloat64, math.MaxFloat64, -math.MaxFloat64}, nil
	}
	if len(blob) < 8+envFloats*8 {
		return env, fmt.Errorf("blob 长度 %d 不足 envelope（%d 个 float64）", len(blob), envFloats)
	}

	offset := 8
	read := func(i int) float64 {
		return math.Float64frombits(binary.LittleEndian.Uint64(blob[offset+i*8:]))
	}
	// GPKG envelope 顺序：minX, maxX, minY, maxY(, minZ, maxZ...)。xy(1) 与 xyzm(4) 前 4 个一致；
	// xyz(2)/xym(3) 顺序为 minX,maxX,minY,maxY,minZ/M,maxZ/M —— 前 4 个仍是 XY。
	env[0] = read(0)
	env[1] = read(1)
	env[2] = read(2)
	env[3] = read(3)
	return env, nil
}
