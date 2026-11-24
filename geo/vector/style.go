// 由腾讯样式表（classifier 内嵌 styles.json）生成 MapLibre GL
// style.json（spec v8），与 internal/mvt 产出的 .mvt 配套：
//
//   - .mvt layer 名 = 要素大类（admin/road/water/...），要素属性 class=stylename
//   - 本包按 stylename 聚合样式分段（跨 style_id 同名合并），生成
//     region→fill、line/arrow→line、point→circle、label→symbol 四类图层
//   - 颜色为 Java ARGB signed int，转为 #rrggbbaa（MapLibre 支持 8 位 hex）
//   - zoom 分段（minScaleVal/maxScaleVal）转为 step 表达式
//
// 已知取舍：
//   - 不还原图标（POI 用中性圆点）、不还原虚线/描边/箭头专用符号
//   - 标注为单一 symbol 图层（固定字号/色），不逐 class 生成
package vector

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	"encoding/json/jsontext"
	"encoding/json/v2"
)

// defaultGlyphs MapLibre 官方演示 glyphs（仅用于标注渲染，可替换）
const defaultGlyphs = "https://demotiles.maplibre.org/font/{fontstack}/{range}.pbf"

// sourceID 固定的矢量源 id
const sourceID = "tencent"

// zoomMax 样式分段可能出现的最大层级（腾讯数据 3-18，留裕量）
const zoomMax = 22

// groupOrder 同类型图层内的大类排序权重（先画大面要素，线在其上）
var groupOrder = map[string]int{
	"admin":    0,
	"green":    1,
	"water":    2,
	"building": 3,
	"railway":  4,
	"road":     5,
	"poi":      6,
	"other":    7,
}

// Options 生成参数
type StyleOptions struct {
	// TilesURL .mvt 瓦片 URL 模板，必须含 {z}/{x}/{y}（如 "tiles/mvt/{z}/{x}/{y}.mvt"）
	TilesURL string
	// MinZoom/MaxZoom 数据源层级范围（下载时的 --zoom 范围）
	MinZoom int
	MaxZoom int
	// Glyphs glyphs URL 模板；空则用 MapLibre 演示 glyphs
	Glyphs string
}

// source/layer/styleRoot 对应 MapLibre style spec v8 的子集
type source struct {
	Type    string   `json:"type"`
	Tiles   []string `json:"tiles"`
	MinZoom int      `json:"minzoom"`
	MaxZoom int      `json:"maxzoom"`
}

type layer struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Source      string         `json:"source"`
	SourceLayer string         `json:"source-layer"`
	Filter      []any          `json:"filter,omitempty"`
	Layout      map[string]any `json:"layout,omitempty"`
	Paint       map[string]any `json:"paint,omitempty"`
}

type styleRoot struct {
	Version int               `json:"version"`
	Name    string            `json:"name"`
	Glyphs  string            `json:"glyphs"`
	Sources map[string]source `json:"sources"`
	Layers  []layer           `json:"layers"`
}

// Generate 生成 style.json 字节
func Generate(c *Classifier, opts StyleOptions) ([]byte, error) {
	if opts.TilesURL == "" {
		return nil, fmt.Errorf("TilesURL 为空")
	}
	sf := c.Styles()

	layers := fillLayers(sf.Region)
	layers = append(layers, lineLayers(sf.Line)...)
	layers = append(layers, lineLayers(sf.Arrow)...)
	layers = append(layers, pointLayer(), labelLayer())

	root := styleRoot{
		Version: 8,
		Name:    "tencent-vt",
		Glyphs:  cmp.Or(opts.Glyphs, defaultGlyphs),
		Sources: map[string]source{
			sourceID: {
				Type:    "vector",
				Tiles:   []string{opts.TilesURL},
				MinZoom: opts.MinZoom,
				MaxZoom: opts.MaxZoom,
			},
		},
		Layers: layers,
	}
	return json.Marshal(root, jsontext.WithIndent("  "), jsontext.Multiline(true))
}

// fillLayers region 样式 → fill 图层（有填充色的 class 才生成）
func fillLayers(region map[int][]StyleEntry) []layer {
	colors := aggregate(region, func(e StyleEntry) *int64 { return e.ColorFill })
	outlines := aggregate(region, func(e StyleEntry) *int64 { return e.ColorSide })

	layers := make([]layer, 0, len(colors))
	for name := range colors {
		paint := map[string]any{"fill-color": colorSteps(colors[name])}
		if c, ok := outlines[name]; ok {
			paint["fill-outline-color"] = colorSteps(c)
		}
		layers = append(layers, layer{
			ID:          "fill-" + name,
			Type:        "fill",
			Source:      sourceID,
			SourceLayer: groupOf(name),
			Filter:      []any{"==", "class", name},
			Paint:       paint,
		})
	}
	sortByGroup(layers)
	return layers
}

// lineLayers line/arrow 样式 → line 图层（全部分段线宽 <=0 的类不可见，跳过）
func lineLayers(t map[int][]StyleEntry) []layer {
	colors := aggregate(t, func(e StyleEntry) *int64 { return e.ColorFill })
	widths := aggregateF(t, func(e StyleEntry) *float64 { return e.Width })

	layers := make([]layer, 0, len(widths))
	for name, w := range widths {
		if allNonPositive(w) {
			continue
		}
		layers = append(layers, layer{
			ID:          "line-" + name,
			Type:        "line",
			Source:      sourceID,
			SourceLayer: groupOf(name),
			Filter:      []any{"==", "class", name},
			Paint: map[string]any{
				"line-color": colorSteps(colors[name]),
				"line-width": widthSteps(w),
			},
		})
	}
	sortByGroup(layers)
	return layers
}

// pointLayer 单一中性圆点图层（不还原官方图标）
func pointLayer() layer {
	return layer{
		ID:          "point-all",
		Type:        "circle",
		Source:      sourceID,
		SourceLayer: "poi",
		Filter:      []any{"==", "type", "point"},
		Paint: map[string]any{
			"circle-radius":  2.5,
			"circle-color":   "#4d4d4d",
			"circle-opacity": 0.7,
		},
	}
}

// labelLayer 单一标注图层（固定字号/色；class 级字样式差异不展开）
func labelLayer() layer {
	return layer{
		ID:          "label-all",
		Type:        "symbol",
		Source:      sourceID,
		SourceLayer: "poi",
		Filter:      []any{"==", "type", "label"},
		Layout: map[string]any{
			"text-field": []any{"get", "name"},
			"text-font":  []any{"Open Sans Semibold"},
			"text-size":  12,
		},
		Paint: map[string]any{
			"text-color":      "#333333",
			"text-halo-color": "#ffffff",
			"text-halo-width": 1,
		},
	}
}

// groupOf stylename → 大类（与 mvt.Layerize 同一套规则）
func groupOf(stylename string) string {
	g := GroupOf(stylename)
	if g == "" {
		return string(GroupOther)
	}
	return string(g)
}

// sortByGroup 同类型图层按大类权重稳定排序（权重相同保持原名序，需先按名排序再排序组）
func sortByGroup(layers []layer) {
	slices.SortFunc(layers, func(a, b layer) int {
		if c := groupOrder[a.SourceLayer] - groupOrder[b.SourceLayer]; c != 0 {
			return c
		}
		return cmpCompare(a.ID, b.ID)
	})
}

// aggregate 按 stylename 聚合 zoom→颜色（同 zoom 跨 style_id 冲突时后遍历者胜出，
// styles.json 中同名段的渲染属性一致，冲突实际不发生）
func aggregate(t map[int][]StyleEntry, pick func(StyleEntry) *int64) map[string]map[int]string {
	out := map[string]map[int]string{}
	for _, entries := range t {
		for _, e := range entries {
			v := pick(e)
			if v == nil {
				continue
			}
			if out[e.Stylename] == nil {
				out[e.Stylename] = map[int]string{}
			}
			for z := max(e.MinScale, 0); z <= min(e.MaxScale, zoomMax); z++ {
				out[e.Stylename][z] = argbHex(*v)
			}
		}
	}
	return out
}

// aggregateF 按 stylename 聚合 zoom→浮点值（width 等）
func aggregateF(t map[int][]StyleEntry, pick func(StyleEntry) *float64) map[string]map[int]float64 {
	out := map[string]map[int]float64{}
	for _, entries := range t {
		for _, e := range entries {
			v := pick(e)
			if v == nil {
				continue
			}
			if out[e.Stylename] == nil {
				out[e.Stylename] = map[int]float64{}
			}
			for z := max(e.MinScale, 0); z <= min(e.MaxScale, zoomMax); z++ {
				out[e.Stylename][z] = *v
			}
		}
	}
	return out
}

// colorSteps zoom→颜色 转 MapLibre step 表达式（值变化处才设断点）
func colorSteps(m map[int]string) []any {
	if len(m) == 0 {
		return []any{"literal", "#cccccc"}
	}
	return steps(m, func(v string) any { return v })
}

// widthSteps zoom→线宽 转 step 表达式
func widthSteps(m map[int]float64) []any {
	if len(m) == 0 {
		return []any{"literal", 1}
	}
	return steps(m, func(v float64) any { return v })
}

// steps 通用构造 ["step",["zoom"],v0,z1,v1,...]，断点取在值发生变化的那一级 zoom
func steps[V comparable](m map[int]V, conv func(V) any) []any {
	zooms := slices.Sorted(maps.Keys(m))
	expr := []any{"step", []string{"zoom"}, conv(m[zooms[0]])}
	prev := m[zooms[0]]
	for _, z := range zooms[1:] {
		if m[z] != prev {
			expr = append(expr, z, conv(m[z]))
			prev = m[z]
		}
	}
	return expr
}

// allNonPositive 全部层级线宽均 <= 0
func allNonPositive(m map[int]float64) bool {
	for _, w := range m {
		if w > 0 {
			return false
		}
	}
	return true
}

// argbHex Java ARGB signed int → #rrggbbaa
func argbHex(v int64) string {
	return fmt.Sprintf("#%08x", uint32(int32(v)))
}

// cmpCompare 字符串比较（避免引入 cmp 包仅为一个调用）
func cmpCompare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
