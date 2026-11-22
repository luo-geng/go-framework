package gpkg

import (
	"fmt"
	"math"
)

// CalculateGeometryExtent 从既有几何数据计算全表 extent，返回 [minX, minY, maxX, maxY]。
// go-spatial/geom 同名函数依赖 spatialite 的 ST_MinX 触发器（cgo），
// 本库改为 Go 侧解析 GPKG 二进制 envelope（envelopeFromGpkgBlob），
// 逐行取并集，兼容 modernc.org/sqlite 纯 Go 驱动。
func (w *Writer) CalculateGeometryExtent(tableName, geomCol string) ([4]float64, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE %s IS NOT NULL`, geomCol, tableName, geomCol)
	rows, err := w.db.Query(query)
	if err != nil {
		return [4]float64{}, fmt.Errorf("查询几何数据失败 %s.%s: %w", tableName, geomCol, err)
	}
	defer rows.Close()

	// 初始化为空 extent：(+Inf, +Inf, -Inf, -Inf)，逐行并集收缩
	extent := [4]float64{math.MaxFloat64, math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64}
	found := false
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			return [4]float64{}, err
		}
		env, err := envelopeFromGpkgBlob(blob) // [minX, maxX, minY, maxY]
		if err != nil {
			return [4]float64{}, fmt.Errorf("解析 envelope 失败: %w", err)
		}
		extent[0] = min(extent[0], env[0])
		extent[1] = min(extent[1], env[2])
		extent[2] = max(extent[2], env[1])
		extent[3] = max(extent[3], env[3])
		found = true
	}
	if err := rows.Err(); err != nil {
		return [4]float64{}, err
	}
	if !found {
		return [4]float64{}, fmt.Errorf("表 %s 无几何数据，无法计算 extent", tableName)
	}
	return extent, nil
}
