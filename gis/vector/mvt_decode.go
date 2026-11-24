package vector

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/paulmach/orb"
	orbmvt "github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/twpayne/go-geom"

	"github.com/luo-geng/go-framework/gis/gcj02"
	"github.com/luo-geng/go-framework/gis/tilemath"
)

// ParseMVTTile 解析标准 .mvt protobuf，转换为输出要素。
//
// 适用于本包 Layerize/EncodeTile 产出的 .mvt（属性 class/type/style_id/name），
// 也兼容任意标准 MVT（缺失属性按几何类型推断）。
// tile 坐标经 orb 投影回 GCJ02 经纬度（腾讯数据约定），opts.CRS="wgs84" 时再转 WGS84。
// data 为 gzip 压缩（mbtiles 惯例）或裸 protobuf 均可，自动识别。
func ParseMVTTile(data []byte, t tilemath.Tile, opts ParseOptions, pd *PointDeduper, gd *GeomDeduper) ([]OutFeature, error) {
	raw, err := gunzipMaybe(data)
	if err != nil {
		return nil, err
	}
	layers, err := orbmvt.Unmarshal(raw)
	if err != nil {
		return nil, fmt.Errorf("MVT 解码失败: %w", err)
	}

	mt := maptile.Tile{X: uint32(t.X), Y: uint32(t.Y), Z: maptile.Zoom(t.Z)}

	var out []OutFeature
	for _, layer := range layers {
		// 投影到 GCJ02 经纬度（ orb 投影为球面 mercator 逆变换，腾讯数据即 GCJ02 经纬度）
		layer.ProjectToWGS84(mt)
		for _, f := range layer.Features {
			of, keep, err := orbFeatureToOut(f, layer.Name, t, opts, pd, gd)
			if err != nil {
				return nil, err
			}
			if keep {
				out = append(out, of)
			}
		}
	}
	return out, nil
}

// gunzipMaybe gzip 魔数识别并解压；非 gzip 原样返回
func gunzipMaybe(data []byte) ([]byte, error) {
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// orbFeatureToOut orb 要素（已投影为经纬度）→ OutFeature
func orbFeatureToOut(f *geojson.Feature, layerName string, t tilemath.Tile, opts ParseOptions, pd *PointDeduper, gd *GeomDeduper) (OutFeature, bool, error) {
	g, ok := orbToGeom(f.Geometry)
	if !ok {
		return OutFeature{}, false, nil
	}

	gt := GeomType(propString(f.Properties, "type"))
	if gt == "" {
		gt = geomTypeOfOrb(f.Geometry)
	}
	if opts.Types != nil && !opts.Types[gt] {
		return OutFeature{}, false, nil
	}

	// ROI 过滤（ROI 为 GCJ02，orb 投影结果同为 GCJ02，可比）
	bbox := geomBBox(g)
	if opts.ROI != nil {
		if p, isPoint := g.(*geom.Point); isPoint {
			if !opts.ROI.ContainsPoint(p.X(), p.Y()) {
				return OutFeature{}, false, nil
			}
		} else if !opts.ROI.IntersectsBBox(bbox) {
			return OutFeature{}, false, nil
		}
	}

	lng, lat := bbox[0], bbox[1]
	if p, isPoint := g.(*geom.Point); isPoint {
		lng, lat = p.X(), p.Y()
		if pd != nil && !pd.TryAdd(lng, lat) {
			return OutFeature{}, false, nil
		}
	} else if gd != nil {
		// 以整要素扁平坐标为哈希输入，避免同名不同几何被误去重
		if !gd.TryAdd(propInt(f.Properties, "style_id"), f.Properties["name"], nil, [][]float64{g.FlatCoords()}) {
			return OutFeature{}, false, nil
		}
	}

	// GCJ02 → WGS84（opts.CRS 要求时）
	if opts.CRS == "wgs84" {
		g = transformGeom(g, gcj02.GCJ02ToWGS84)
	}

	styleID := propInt(f.Properties, "style_id")
	return OutFeature{
		Geom:    g,
		Name:    propString(f.Properties, "name"),
		Class:   propString(f.Properties, "class"),
		Group:   propOr(f.Properties, "group", layerName),
		StyleID: styleID,
		Zoom:    t.Z, TileZ: t.Z, TileX: t.X, TileY: t.Y,
	}, true, nil
}

// orbToGeom orb 几何 → go-geom；不支持的类型返回 false
func orbToGeom(g orb.Geometry) (geom.T, bool) {
	toFlat := func(p []orb.Point) []float64 {
		flat := make([]float64, 0, len(p)*2)
		for _, q := range p {
			flat = append(flat, q[0], q[1])
		}
		return flat
	}

	switch v := g.(type) {
	case orb.Point:
		return geom.NewPointFlat(geom.XY, []float64{v[0], v[1]}), true
	case orb.MultiPoint:
		return geom.NewMultiPointFlat(geom.XY, toFlat(v)), len(v) > 0
	case orb.LineString:
		return geom.NewLineStringFlat(geom.XY, toFlat(v)), true
	case orb.MultiLineString:
		var flat []float64
		var ends []int
		for _, ls := range v {
			flat = append(flat, toFlat(ls)...)
			ends = append(ends, len(flat))
		}
		return geom.NewMultiLineStringFlat(geom.XY, flat, ends), true
	case orb.Polygon:
		var flat []float64
		var ends []int
		for _, ring := range v {
			flat = append(flat, toFlat(ring)...)
			ends = append(ends, len(flat))
		}
		return geom.NewPolygonFlat(geom.XY, flat, ends), true
	case orb.MultiPolygon:
		var flats []float64
		var endss [][]int
		for _, p := range v {
			var ends []int
			base := len(flats)
			for _, ring := range p {
				flats = append(flats, toFlat(ring)...)
				ends = append(ends, len(flats))
			}
			for i := range ends {
				ends[i] += base
			}
			endss = append(endss, ends)
		}
		return geom.NewMultiPolygonFlat(geom.XY, flats, endss), true
	}
	return nil, false
}

// geomTypeOfOrb 按 orb 几何类型推断 GeomType
func geomTypeOfOrb(g orb.Geometry) GeomType {
	switch g.(type) {
	case orb.Point, orb.MultiPoint:
		return Point
	case orb.LineString, orb.MultiLineString:
		return Line
	case orb.Polygon, orb.MultiPolygon:
		return Region
	}
	return ""
}

// transformGeom 对几何全部坐标做 lng/lat 变换
func transformGeom(g geom.T, fn func(lng, lat float64) (float64, float64)) geom.T {
	switch v := g.(type) {
	case *geom.Point:
		return geom.NewPointFlat(geom.XY, transformFlat(v.FlatCoords(), fn))
	case *geom.LineString:
		return geom.NewLineStringFlat(geom.XY, transformFlat(v.FlatCoords(), fn))
	case *geom.MultiLineString:
		return geom.NewMultiLineStringFlat(geom.XY, transformFlat(v.FlatCoords(), fn), v.Ends())
	case *geom.Polygon:
		return geom.NewPolygonFlat(geom.XY, transformFlat(v.FlatCoords(), fn), v.Ends())
	case *geom.MultiPolygon:
		return geom.NewMultiPolygonFlat(geom.XY, transformFlat(v.FlatCoords(), fn), v.Endss())
	}
	return g
}

// transformFlat 原地变换扁平坐标对
func transformFlat(flat []float64, fn func(lng, lat float64) (float64, float64)) []float64 {
	out := make([]float64, len(flat))
	for i := 0; i+1 < len(flat); i += 2 {
		out[i], out[i+1] = fn(flat[i], flat[i+1])
	}
	return out
}

// geomBBox 几何包围盒 [minLng, minLat, maxLng, maxLat]
func geomBBox(g geom.T) [4]float64 {
	flat := g.FlatCoords()
	bbox := [4]float64{180, 90, -180, -90}
	for i := 0; i+1 < len(flat); i += 2 {
		bbox[0] = min(bbox[0], flat[i])
		bbox[1] = min(bbox[1], flat[i+1])
		bbox[2] = max(bbox[2], flat[i])
		bbox[3] = max(bbox[3], flat[i+1])
	}
	return bbox
}

// propString 字符串属性提取（非字符串视为空）
func propString(props map[string]any, key string) string {
	if s, ok := props[key].(string); ok {
		return s
	}
	return ""
}

// propOr 取字符串属性，缺失时回退默认值
func propOr(props map[string]any, key, fallback string) string {
	if s := propString(props, key); s != "" {
		return s
	}
	return fallback
}

// propInt 数值属性提取（浮点/整型兼容）
func propInt(props map[string]any, key string) int {
	switch v := props[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}
