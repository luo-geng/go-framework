// 提供 style_id + zoom → 几何类型与 stylename 的分类能力。
//
// 样式表（styles.json）逆向自腾讯前端，约 4751 条，按 5 类组织：
// regionStyles / lineStyles / pointStyles / labelStyles / arrowStyles。
// 注意：每个 style_id 对应按 zoom 分段的数组，不同 z 段 stylename 可能不同，
// 必须按 minScale<=z<=maxScale 选段。
package vector

import (
	_ "embed"
	"encoding/json/v2"
	"fmt"
	"strings"
)

//go:embed styles.json
var stylesJSON []byte

// GeomType 几何类型（由 style_id 高 4 位掩码决定）
type GeomType string

const (
	Point  GeomType = "point"
	Line   GeomType = "line"
	Region GeomType = "region"
	Label  GeomType = "label"
	Arrow  GeomType = "arrow"
)

// typeMask 类型掩码：style_id & typeMask 得到类型段
const typeMask = 983040

// StyleEntry 单条样式记录。
// 渲染字段为可选指针（点/标注类无 color_fill/width 等字段）；
// 颜色为 Java ARGB signed int（如 -13421773 → #ff333333），由 stylegen 转换。
type StyleEntry struct {
	MinScale        int      `json:"minScaleVal"`
	MaxScale        int      `json:"maxScaleVal"`
	Stylename       string   `json:"stylename"`
	ColorFill       *int64   `json:"color_fill"`
	ColorSide       *int64   `json:"color_side"`
	Width           *float64 `json:"width"`
	SideWidth       *float64 `json:"sidewidth"`
	FontSize        *float64 `json:"font_size"`
	FontColor       *int64   `json:"font_color"`
	BackgroundColor *int64   `json:"background_color"`
	HasDash         *int     `json:"has_dash"`
}

// Classifier 样式分类器
type Classifier struct {
	point  map[int][]StyleEntry
	line   map[int][]StyleEntry
	region map[int][]StyleEntry
	label  map[int][]StyleEntry
	arrow  map[int][]StyleEntry
}

// StylesFile styles.json 顶层结构
type StylesFile struct {
	Region map[int][]StyleEntry `json:"regionStyles"`
	Line   map[int][]StyleEntry `json:"lineStyles"`
	Point  map[int][]StyleEntry `json:"pointStyles"`
	Label  map[int][]StyleEntry `json:"labelStyles"`
	Arrow  map[int][]StyleEntry `json:"arrowStyles"`
}

// New 加载内嵌样式表，构造分类器
func NewClassifier() (*Classifier, error) {
	var sf StylesFile
	if err := json.Unmarshal(stylesJSON, &sf); err != nil {
		return nil, fmt.Errorf("样式表解析失败: %w", err)
	}
	return &Classifier{
		point:  sf.Point,
		line:   sf.Line,
		region: sf.Region,
		label:  sf.Label,
		arrow:  sf.Arrow,
	}, nil
}

// Styles 返回完整样式表（供 stylegen 等消费渲染属性；勿修改返回值内容）
func (c *Classifier) Styles() StylesFile {
	return StylesFile{Region: c.region, Line: c.line, Point: c.point, Label: c.label, Arrow: c.arrow}
}

// GeomTypeOf 由 style_id 得到几何类型段
func GeomTypeOf(styleID int) GeomType {
	switch styleID & typeMask {
	case 65536:
		return Point
	case 131072:
		return Line
	case 196608:
		return Region
	case 262144:
		return Label
	case 327680:
		return Arrow
	}
	return ""
}

// Classify 返回 (几何类型, stylename, 是否命中样式表)。未命中时 ok=false，调用方应记 unknown。
func (c *Classifier) Classify(styleID, zoom int) (GeomType, string, bool) {
	gt := GeomTypeOf(styleID)
	var table map[int][]StyleEntry
	switch gt {
	case Point:
		table = c.point
	case Line:
		table = c.line
	case Region:
		table = c.region
	case Label:
		table = c.label
	case Arrow:
		table = c.arrow
	}

	entries, ok := table[styleID]
	if !ok {
		return gt, "", false
	}
	for _, e := range entries {
		if e.MinScale <= zoom && zoom <= e.MaxScale {
			return gt, e.Stylename, true
		}
	}
	return gt, "", false
}

// Group 要素大类（在 stylename 之上再归组，便于按主题使用数据）
type Group string

const (
	GroupAdmin    Group = "admin"    // 行政区划/功能区（admarea/aoi/community*）
	GroupRoad     Group = "road"     // 道路（road*/b_road*）
	GroupRailway  Group = "railway"  // 铁路/轨道交通（railway*/sr_*/subway*）
	GroupWater    Group = "water"    // 水系（water*/river*/sea*）
	GroupGreen    Group = "green"    // 绿地（green*/park*）
	GroupBuilding Group = "building" // 建筑物（building*/dasha）
	GroupPOI      Group = "poi"      // 兴趣点（兜底）
	GroupOther    Group = "other"    // 其他（线/面/箭头兜底）
)

// groupRules 前缀匹配规则，先匹配先命中
var groupRules = []struct {
	prefix string
	group  Group
}{
	{"adm", GroupAdmin},
	{"b_adm", GroupAdmin},
	{"admlm", GroupAdmin},
	{"aoi", GroupAdmin},
	{"community", GroupAdmin},
	{"railway", GroupRailway},
	{"b_railway", GroupRailway},
	{"sr_", GroupRailway},
	{"subway", GroupRailway},
	{"road", GroupRoad},
	{"b_road", GroupRoad},
	{"water", GroupWater},
	{"river", GroupWater},
	{"sea", GroupWater},
	{"green", GroupGreen},
	{"park", GroupGreen},
	{"building", GroupBuilding},
	{"dasha", GroupBuilding},
}

// GroupOf 返回 stylename 归属的大类。
// unknown（未命中样式表的）由调用方按几何类型兜底为 poi/other。
func GroupOf(stylename string) Group {
	for _, r := range groupRules {
		if strings.HasPrefix(stylename, r.prefix) {
			return r.group
		}
	}
	return ""
}
