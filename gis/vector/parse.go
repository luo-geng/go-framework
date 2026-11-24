package vector

import (
	"fmt"
	"math"

	"github.com/luo-geng/go-framework/gis/gcj02"
	"github.com/luo-geng/go-framework/gis/tilemath"
	"github.com/twpayne/go-geom"
)

// OutFeature 解析输出要素（坐标为 GCJ02 或 WGS84，由 Options.CRS 决定）
type OutFeature struct {
	Geom                               geom.T
	Name, Class, Group                 string
	StyleID, Zoom, TileZ, TileX, TileY int
}

// Options 解析选项
type ParseOptions struct {
	// Types 为 nil 表示输出全部类型
	Types   map[GeomType]bool
	CRS     string // "gcj02"（默认）或 "wgs84"
	ROI     *ROI   // nil 不做空间过滤
	SplitBy string // 表/文件分组粒度：type|group|class
}

// ParseTileRaw 解析单个 .tmvt 文件（解码后的 JSON 二进制），返回输出要素与未命中样式表的 style_id 列表。
// 同一批瓦片应共享 pd/gd 去重器与分类器。
func ParseTileRaw(raw []byte, t tilemath.Tile, opts ParseOptions, c *Classifier, pd *PointDeduper, gd *GeomDeduper) ([]OutFeature, []int, error) {
	feats, err := ParseTileJSON(raw)
	if err != nil {
		return nil, nil, err
	}

	var out []OutFeature
	var unknown []int

	for _, f := range feats {
		gt, stylename, ok := c.Classify(f.StyleID, t.Z)
		if !ok {
			unknown = append(unknown, f.StyleID)
			stylename = fmt.Sprintf("unknown_%d", f.StyleID)
		}
		if opts.Types != nil && !opts.Types[gt] {
			continue
		}

		group := string(GroupOf(stylename))
		if group == "" {
			if gt == Point || gt == Label {
				group = string(GroupPOI)
			} else {
				group = string(GroupOther)
			}
		}

		of, keep := buildOutFeature(f, gt, t, opts, pd, gd)
		if !keep {
			continue
		}
		of.Class = stylename
		of.Group = group
		out = append(out, of)
	}
	return out, unknown, nil
}

// buildOutFeature 构建输出要素（坐标转换 + 去重 + 空间过滤 + 几何构造）
func buildOutFeature(f Feature, gt GeomType, t tilemath.Tile, opts ParseOptions, pd *PointDeduper, gd *GeomDeduper) (OutFeature, bool) {
	of := OutFeature{
		Name:    labelText(f.Label),
		StyleID: f.StyleID,
		Zoom:    t.Z, TileZ: t.Z, TileX: t.X, TileY: t.Y,
	}

	switch gt {
	case Point, Label:
		if len(f.Segments) == 0 || len(f.Segments[0]) < 2 {
			return of, false
		}
		lng, lat := pixelToCRS(t, f.Segments[0][0], f.Segments[0][1], opts.CRS)
		if opts.ROI != nil && !opts.ROI.ContainsPoint(lng, lat) {
			return of, false
		}
		if pd != nil && !pd.TryAdd(lng, lat) {
			return of, false
		}
		of.Geom = geom.NewPointFlat(geom.XY, []float64{lng, lat})

	case Line, Arrow:
		segs := toLonLatSegs(f.Segments, t, opts.CRS)
		if len(segs) == 0 {
			return of, false
		}
		if opts.ROI != nil && !opts.ROI.IntersectsBBox(segsBBox(segs)) {
			return of, false
		}
		if gd != nil && !gd.TryAdd(f.StyleID, f.Label, f.Sign, segs) {
			return of, false
		}
		if len(segs) == 1 {
			of.Geom = geom.NewLineStringFlat(geom.XY, segs[0])
		} else {
			flat, ends := flattenSegs(segs)
			of.Geom = geom.NewMultiLineStringFlat(geom.XY, flat, ends)
		}

	case Region:
		rings := toLonLatSegs(f.Segments, t, opts.CRS)
		if len(rings) == 0 {
			return of, false
		}
		if opts.ROI != nil && !opts.ROI.IntersectsBBox(segsBBox(rings)) {
			return of, false
		}
		if gd != nil && !gd.TryAdd(f.StyleID, f.Label, f.Sign, rings) {
			return of, false
		}
		// 每段视为独立面的外环；闭合环（OGC 要求首末点相同）
		if len(rings) == 1 {
			flat, ends := closeRings(rings)
			of.Geom = geom.NewPolygonFlat(geom.XY, flat, ends)
		} else {
			var flats []float64
			var endss [][]int // 每个多边形一组环的绝对结束索引（go-geom 要求跨多边形累计）
			for _, r := range rings {
				flat, ringEnds := closeRings([][]float64{r})
				base := len(flats)
				flats = append(flats, flat...)
				absEnds := make([]int, len(ringEnds))
				for j, e := range ringEnds {
					absEnds[j] = base + e
				}
				endss = append(endss, absEnds)
			}
			of.Geom = geom.NewMultiPolygonFlat(geom.XY, flats, endss)
		}
	}
	return of, true
}

// pixelToCRS 瓦片像素坐标 → 目标坐标系经纬度
func pixelToCRS(t tilemath.Tile, px, py int, crs string) (float64, float64) {
	lng, lat := tilemath.PixelToLonLatTMS(t, float64(px), float64(py))
	if crs == "wgs84" {
		return gcj02.GCJ02ToWGS84(lng, lat)
	}
	return lng, lat
}

// toLonLatSegs 全部像素段 → 经纬度段
func toLonLatSegs(segs []Segment, t tilemath.Tile, crs string) [][]float64 {
	out := make([][]float64, 0, len(segs))
	for _, seg := range segs {
		if len(seg) < 2 {
			continue
		}
		flat := make([]float64, 0, len(seg))
		for i := 0; i+1 < len(seg); i += 2 {
			lng, lat := pixelToCRS(t, seg[i], seg[i+1], crs)
			flat = append(flat, lng, lat)
		}
		out = append(out, flat)
	}
	return out
}

// flattenSegs 多段 → (扁平坐标, 各段结束索引)
func flattenSegs(segs [][]float64) ([]float64, []int) {
	var flat []float64
	var ends []int
	for _, s := range segs {
		flat = append(flat, s...)
		ends = append(ends, len(flat))
	}
	return flat, ends
}

// closeRings 闭合所有环并返回 go-geom Polygon 参数
func closeRings(rings [][]float64) ([]float64, []int) {
	var flat []float64
	var ends []int
	for _, r := range rings {
		if len(r) >= 4 && (r[0] != r[len(r)-2] || r[1] != r[len(r)-1]) {
			r = append(r, r[0], r[1])
		}
		flat = append(flat, r...)
		ends = append(ends, len(flat))
	}
	return flat, ends
}

// segsBBox 多段坐标包围盒 [minLng, minLat, maxLng, maxLat]
func segsBBox(segs [][]float64) [4]float64 {
	bbox := [4]float64{math.MaxFloat64, math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64}
	for _, s := range segs {
		for i := 0; i+1 < len(s); i += 2 {
			bbox[0] = min(bbox[0], s[i])
			bbox[1] = min(bbox[1], s[i+1])
			bbox[2] = max(bbox[2], s[i])
			bbox[3] = max(bbox[3], s[i+1])
		}
	}
	return bbox
}

// labelText 提取要素名称
func labelText(label any) string {
	if s, ok := label.(string); ok {
		return s
	}
	return ""
}
