// 将解析结果写入 GeoPackage（基于 github.com/luo-geng/go-framework/geo/gpkg）。
package vector

import (
	"errors"
	"fmt"
	"os"
	"strings"

	geopkg "github.com/luo-geng/go-framework/geo/gpkg"
	"github.com/twpayne/go-geom"
)

// 每批提交的要素数（控制事务大小与内存）
const batchSize = 1000

// 统一属性列（所有表一致，简化建表）
var attributeColumns = []geopkg.Column{
	{Name: "name", Type: "TEXT"},
	{Name: "class", Type: "TEXT"},
	{Name: "group_name", Type: "TEXT"},
	{Name: "style_id", Type: "INTEGER"},
	{Name: "zoom", Type: "INTEGER"},
	{Name: "tile_z", Type: "INTEGER"},
	{Name: "tile_x", Type: "INTEGER"},
	{Name: "tile_y", Type: "INTEGER"},
}

// Writer GPKG 输出器；表按 splitKey 惰性创建
type GPKGWriter struct {
	w        *geopkg.Writer
	created  map[string]bool
	buffers  map[string][]geopkg.Feature
	fid      int64
	splitKey func(OutFeature) string
}

// New 创建输出器；splitKey 决定建表粒度（type/group/class）。
// 已存在的同名文件会被删除重建，保证输出干净。
func NewGPKGWriter(path string, splitKey func(OutFeature) string) (*GPKGWriter, error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	w, err := geopkg.NewWriter(path)
	if err != nil {
		return nil, err
	}
	return &GPKGWriter{
		w:        w,
		created:  make(map[string]bool),
		buffers:  make(map[string][]geopkg.Feature),
		splitKey: splitKey,
	}, nil
}

// Write 写入一批要素（内部按表缓冲，满 batchSize 自动提交）
func (w *GPKGWriter) Write(feats []OutFeature) error {
	for _, f := range feats {
		table := sanitizeName(w.splitKey(f))
		if err := w.ensureTable(table, f.Geom); err != nil {
			return err
		}

		w.fid++
		w.buffers[table] = append(w.buffers[table], geopkg.Feature{
			ID:       w.fid,
			Geometry: f.Geom,
			Attributes: map[string]any{
				"name":       f.Name,
				"class":      f.Class,
				"group_name": f.Group,
				"style_id":   f.StyleID,
				"zoom":       f.Zoom,
				"tile_z":     f.TileZ,
				"tile_x":     f.TileX,
				"tile_y":     f.TileY,
			},
		})

		if len(w.buffers[table]) >= batchSize {
			if err := w.flushTable(table); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close 提交剩余缓冲、为各表回填空间索引（rtree）并关闭
func (w *GPKGWriter) Close() error {
	for table := range w.buffers {
		if err := w.flushTable(table); err != nil {
			return err
		}
	}
	// 所有要素落库后统一回填 rtree（解析 envelope 批量插入，幂等）
	for table := range w.created {
		if err := w.w.BuildRTreeIndex(table, "geom"); err != nil {
			return fmt.Errorf("构建空间索引失败 %s: %w", table, err)
		}
	}
	return w.w.Close()
}

// ensureTable 首次写入某表时创建（几何类型取该表首个要素的类型）
func (w *GPKGWriter) ensureTable(name string, g geom.T) error {
	if w.created[name] {
		return nil
	}
	srsID := 4326
	err := w.w.CreateTable(geopkg.Table{
		Name:             name,
		GeometryColumn:   "geom",
		GeometryType:     geometryTypeOf(g),
		SRSID:            &srsID,
		AttributeColumns: attributeColumns,
	})
	if err != nil {
		return fmt.Errorf("创建表 %s 失败: %w", name, err)
	}
	w.created[name] = true
	return nil
}

// flushTable 提交某表缓冲
func (w *GPKGWriter) flushTable(name string) error {
	buf := w.buffers[name]
	if len(buf) == 0 {
		return nil
	}
	if err := w.w.InsertFeatures(name, "geom", buf); err != nil {
		return err
	}
	w.buffers[name] = buf[:0]
	return nil
}

// geometryTypeOf go-geom 几何 → GPKG 类型名
func geometryTypeOf(g geom.T) geopkg.GeometryType {
	switch g.(type) {
	case *geom.Point:
		return geopkg.GeomPoint
	case *geom.LineString:
		return geopkg.GeomLineString
	case *geom.Polygon:
		return geopkg.GeomPolygon
	case *geom.MultiLineString:
		return geopkg.GeomLineString // GPKG 单表多几何类型以首要素为准
	case *geom.MultiPolygon:
		return geopkg.GeomPolygon
	}
	return geopkg.GeomPoint
}

// sanitizeName 表名安全化：非字母数字下划线 → 下划线
func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
