// 提供流式 GeoJSON/GeoJSONL 输出。
package vector

import (
	"bufio"
	"encoding/json/v2"
	"fmt"
	"os"

	"github.com/twpayne/go-geom/encoding/geojson"
)

// Writer 流式 GeoJSON 写出器（内存不驻留全量要素）
type GeoJSONWriter struct {
	file     *os.File
	buf      *bufio.Writer
	geojsonl bool
	first    bool
	written  int64
	split    bool
}

// New 创建输出器；geojsonl=true 时每个要素一行（GeoJSONL）
func NewGeoJSONWriter(path string, geojsonl bool) (*GeoJSONWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := &GeoJSONWriter{file: f, buf: bufio.NewWriterSize(f, 64*1024), geojsonl: geojsonl, first: true}
	if !geojsonl {
		if _, err := w.buf.WriteString(`{"type":"FeatureCollection","features":[`); err != nil {
			return nil, err
		}
	}
	return w, nil
}

// Write 写出一批要素
func (w *GeoJSONWriter) Write(feats []OutFeature) error {
	for _, f := range feats {
		if err := w.writeFeature(f); err != nil {
			return err
		}
	}
	return nil
}

// writeFeature 单个要素 → GeoJSON Feature
func (w *GeoJSONWriter) writeFeature(f OutFeature) error {
	geomJSON, err := geojson.Marshal(f.Geom)
	if err != nil {
		return fmt.Errorf("几何编码失败: %w", err)
	}

	feat := map[string]any{
		"type":       "Feature",
		"geometry":   rawJSON(geomJSON),
		"properties": properties(f),
	}
	data, err := json.Marshal(feat)
	if err != nil {
		return err
	}

	if !w.geojsonl && !w.first {
		if err := w.buf.WriteByte(','); err != nil {
			return err
		}
	}
	w.first = false
	if w.geojsonl {
		data = append(data, '\n')
	}
	if _, err := w.buf.Write(data); err != nil {
		return err
	}
	w.written++
	return nil
}

// Close 收尾并关闭
func (w *GeoJSONWriter) Close() error {
	if !w.geojsonl {
		if _, err := w.buf.WriteString("]}"); err != nil {
			return err
		}
	}
	if err := w.buf.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}

// Count 已写出要素数
func (w *GeoJSONWriter) Count() int64 {
	return w.written
}

// properties 要素属性表
func properties(f OutFeature) map[string]any {
	return map[string]any{
		"name":     f.Name,
		"class":    f.Class,
		"group":    f.Group,
		"style_id": f.StyleID,
		"zoom":     f.Zoom,
		"tile_z":   f.TileZ,
		"tile_x":   f.TileX,
		"tile_y":   f.TileY,
	}
}

// rawJSON 已编码 JSON 片段的原样包装（实现 v1 Marshaler 接口，json/v2 兼容）
type rawJSON []byte

func (r rawJSON) MarshalJSON() ([]byte, error) { return r, nil }
