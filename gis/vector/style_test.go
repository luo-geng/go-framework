package vector

import (
	"encoding/json/v2"
	"slices"
	"strings"
	"testing"
)

func TestArgbHex(t *testing.T) {
	cases := map[int64]string{
		-920858:    "#fff1f2e6", // 实测面填充色样本
		-13421773:  "#ff333333", // 实测字体色样本
		0:          "#00000000",
		-1:         "#ffffffff",
		-8686631:   "#ff7b73d9", // 实测道路色样本
		2147483647: "#7fffffff",
	}
	for in, want := range cases {
		if got := argbHex(in); got != want {
			t.Errorf("argbHex(%d) = %s, want %s", in, got, want)
		}
	}
}

// findClass 在样式表中找第一个匹配前缀的 stylename
func findClass(t *testing.T, c *Classifier, table func(StylesFile) map[int][]StyleEntry, prefix string) string {
	t.Helper()
	for _, entries := range table(c.Styles()) {
		for _, e := range entries {
			if strings.HasPrefix(e.Stylename, prefix) {
				return e.Stylename
			}
		}
	}
	return ""
}

func TestGenerate(t *testing.T) {
	c, err := NewClassifier()
	if err != nil {
		t.Fatal(err)
	}
	data, err := Generate(c, StyleOptions{
		TilesURL: "tiles/mvt/{z}/{x}/{y}.mvt",
		MinZoom:  16,
		MaxZoom:  18,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 结构校验：合法 JSON、version 8、source 配置正确
	var root struct {
		Version int `json:"version"`
		Sources map[string]struct {
			Type    string   `json:"type"`
			Tiles   []string `json:"tiles"`
			MinZoom int      `json:"minzoom"`
			MaxZoom int      `json:"maxzoom"`
		} `json:"sources"`
		Layers []struct {
			ID          string         `json:"id"`
			Type        string         `json:"type"`
			SourceLayer string         `json:"source-layer"`
			Paint       map[string]any `json:"paint"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("style.json 不是合法 JSON: %v", err)
	}
	if root.Version != 8 {
		t.Errorf("version = %d, want 8", root.Version)
	}
	src, ok := root.Sources[sourceID]
	if !ok || src.Type != "vector" {
		t.Fatalf("source %s 缺失或类型错误: %+v", sourceID, root.Sources)
	}
	if src.Tiles[0] != "tiles/mvt/{z}/{x}/{y}.mvt" || src.MinZoom != 16 || src.MaxZoom != 18 {
		t.Errorf("source 配置错误: %+v", src)
	}

	// 图层类型序：fill 在前，line 居中，circle/symbol 收尾
	typeOrder := map[string]int{"fill": 0, "line": 1, "circle": 2, "symbol": 3}
	last := -1
	for _, l := range root.Layers {
		if typeOrder[l.Type] < last {
			t.Errorf("图层类型顺序错误: %s(%s) 出现在更上层类型之后", l.ID, l.Type)
		}
		last = typeOrder[l.Type]
	}

	// fill 层：water 系 class 存在且颜色为 step 表达式
	water := findClass(t, c, func(s StylesFile) map[int][]StyleEntry { return s.Region }, "water")
	if water == "" {
		t.Fatal("样式表中未找到 water 系 region")
	}
	idx := slices.IndexFunc(root.Layers, func(l struct {
		ID          string         `json:"id"`
		Type        string         `json:"type"`
		SourceLayer string         `json:"source-layer"`
		Paint       map[string]any `json:"paint"`
	}) bool {
		return l.ID == "fill-"+water
	})
	if idx < 0 {
		t.Fatalf("未生成 fill-%s 图层", water)
	}
	fillColor, ok := root.Layers[idx].Paint["fill-color"].([]any)
	if !ok || fillColor[0] != "step" {
		t.Errorf("fill-color = %v, want step 表达式", root.Layers[idx].Paint["fill-color"])
	}

	// line 层：road 系 class 存在且带 line-width
	road := findClass(t, c, func(s StylesFile) map[int][]StyleEntry { return s.Line }, "road")
	if road == "" {
		t.Fatal("样式表中未找到 road 系 line")
	}
	roadIdx := slices.IndexFunc(root.Layers, func(l struct {
		ID          string         `json:"id"`
		Type        string         `json:"type"`
		SourceLayer string         `json:"source-layer"`
		Paint       map[string]any `json:"paint"`
	}) bool {
		return l.ID == "line-"+road
	})
	if roadIdx < 0 {
		t.Fatalf("未生成 line-%s 图层", road)
	}
	if _, ok := root.Layers[roadIdx].Paint["line-width"]; !ok {
		t.Errorf("line-%s 缺 line-width", road)
	}
}
