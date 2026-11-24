package vector

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseTile_Golden 与 node 版 tile_parser.mjs 输出逐 feature 对拍。
// 样本为真实抓取的瓦片（testdata/*.tmvt），若腾讯服务端格式变化此测试会立即暴露。
func TestParseTile_Golden(t *testing.T) {
	tiles, err := filepath.Glob("testdata/*.tmvt")
	if err != nil || len(tiles) == 0 {
		t.Fatalf("缺少黄金测试样本: %v", err)
	}

	for _, tilePath := range tiles {
		t.Run(filepath.Base(tilePath), func(t *testing.T) {
			raw, err := os.ReadFile(tilePath)
			if err != nil {
				t.Fatal(err)
			}

			// .tmvt 为解码后的 JSON 二进制
			got, err := ParseTileJSON(raw)
			if err != nil {
				t.Fatalf("ParseTileJSON 失败: %v", err)
			}

			expRaw, err := os.ReadFile(strings.TrimSuffix(tilePath, ".tmvt") + ".expected.json")
			if err != nil {
				t.Fatalf("读取基准失败: %v", err)
			}
			var expected []any
			if err := json.Unmarshal(expRaw, &expected); err != nil {
				t.Fatalf("基准 JSON 解析失败: %v", err)
			}

			if len(got) != len(expected) {
				t.Fatalf("feature 数量不符: got=%d want=%d", len(got), len(expected))
			}
			for i, exp := range expected {
				assertFeatureEqual(t, i, exp, got[i])
			}
		})
	}
}

// assertFeatureEqual 对比单条要素：[style_id, pixed, label, sign]
func assertFeatureEqual(t *testing.T, i int, exp any, got Feature) {
	t.Helper()

	arr, ok := exp.([]any)
	if !ok || len(arr) < 2 {
		t.Fatalf("feature[%d] 基准结构异常: %v", i, exp)
	}

	styleID, _ := arr[0].(float64)
	if int(styleID) != got.StyleID {
		t.Errorf("feature[%d] style_id: got=%d want=%v", i, got.StyleID, arr[0])
	}

	// pixed：单段扁平数组或多段嵌套数组
	expSegs := toSegments(t, i, arr[1])
	if len(expSegs) != len(got.Segments) {
		t.Fatalf("feature[%d] 段数: got=%d want=%d", i, len(got.Segments), len(expSegs))
	}
	for j := range expSegs {
		if fmt.Sprint(got.Segments[j]) != fmt.Sprint(expSegs[j]) {
			t.Errorf("feature[%d] seg[%d]:\n got=%v\nwant=%v", i, j, got.Segments[j], expSegs[j])
		}
	}

	// label / sign
	if len(arr) > 2 && !scalarEqual(arr[2], got.Label) {
		t.Errorf("feature[%d] label: got=%v want=%v", i, got.Label, arr[2])
	}
	if len(arr) > 3 && !scalarEqual(arr[3], got.Sign) {
		t.Errorf("feature[%d] sign: got=%v want=%v", i, got.Sign, arr[3])
	}
}

// toSegments 基准 pixed → [][]float64
func toSegments(t *testing.T, i int, raw any) [][]float64 {
	t.Helper()

	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("feature[%d] pixed 非数组: %v", i, raw)
	}
	if len(arr) == 0 {
		return nil
	}

	flatten := func(a []any) []float64 {
		out := make([]float64, len(a))
		for k, v := range a {
			n, ok := v.(float64)
			if !ok {
				t.Fatalf("feature[%d] 坐标非数字: %v", i, v)
			}
			out[k] = n
		}
		return out
	}

	if _, nested := arr[0].([]any); nested {
		segs := make([][]float64, len(arr))
		for j, item := range arr {
			segs[j] = flatten(item.([]any))
		}
		return segs
	}
	return [][]float64{flatten(arr)}
}

// scalarEqual JSON 标量对比（int 与 float64 视为相等）
func scalarEqual(want, got any) bool {
	if want == nil || got == nil {
		return want == nil && got == nil
	}
	if w, ok := want.(float64); ok {
		g, ok := got.(float64)
		return ok && w == g
	}
	return fmt.Sprint(got) == fmt.Sprint(want)
}
