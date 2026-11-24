// 解码腾讯矢量瓦片。
//
// 编码管线（逆向自腾讯前端 JS，见 docs/瓦片格式解析.md）：
// base64（去 padding、滤非法字符）→ 每2字节一组做3轮自定义异或解扰
// → UTF-8 → 截到最后一个 '}' → JSON。
//
// 要素坐标为瓦片内差分编码的像素坐标，本包负责还原差分并合并 Multi 分段。
package vector

import (
	"bytes"
	"encoding/base64"
	"encoding/json/v2"
	"fmt"
	"strings"
)

// Segment 一段扁平像素坐标 [x0,y0,x1,y1,...]；Multi 几何由多个 Segment 组成
type Segment []int

// Feature 一个解码后的要素
type Feature struct {
	StyleID  int
	Segments []Segment
	// Label 要素名称；面要素可能为数字 4；可能为空
	Label any
	// Sign 同类要素（同面/同线）的分段标识；无则为空
	Sign any
}

// ParseTile 解码瓦片 data 字段（base64 字符串），返回还原差分并合并 Multi 后的要素列表。
// ret != 0、无 features 或空瓦片返回空列表，不视为错误。
//
// 注意：.tmvt 文件存的是 ParseTileJSON 接受的解码后二进制，本函数仅用于
// 直接处理线上响应的 base64 字符串。
func ParseTile(data string) ([]Feature, error) {
	raw, err := DecodeRawData(data)
	if err != nil {
		return nil, err
	}
	return ParseTileJSON(raw)
}

// DecodeRawData 线上 base64 → 解码后 JSON 二进制（.tmvt 文件内容格式）
func DecodeRawData(data string) ([]byte, error) {
	return descrambleBase64(data)
}

// ParseTileJSON 解析解码后的瓦片 JSON 二进制（.tmvt 文件内容），
// 返回还原差分并合并 Multi 后的要素列表。
// ret != 0、无 features 或空瓦片返回空列表，不视为错误。
func ParseTileJSON(raw []byte) ([]Feature, error) {
	// 截到最后一个 '}'，容忍尾部脏字节
	end := bytes.LastIndexByte(raw, '}')
	if end < 0 {
		return nil, fmt.Errorf("解码结果中未找到 JSON 结尾: %q", preview(raw))
	}

	var tile struct {
		Ret      int   `json:"ret"`
		Features []any `json:"features"`
	}
	if err := json.Unmarshal(raw[:end+1], &tile); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	if tile.Ret != 0 || len(tile.Features) == 0 {
		return nil, nil
	}
	return parseFeatures(tile.Features)
}

// descrambleBase64 base64 解码 + 自定义异或解扰
func descrambleBase64(data string) ([]byte, error) {
	// JS 实现为去两次尾部 '='，等价于 TrimSuffix
	clean := filterBase64Chars(strings.TrimSuffix(strings.TrimSuffix(data, "="), "="))
	encoded, err := base64.RawStdEncoding.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("base64 解码失败: %w", err)
	}
	return descramble(encoded), nil
}

// filterBase64Chars 过滤非法字符（复刻 JS 的 replace(/[^A-Za-z0-9+/=]/g, ”)）
func filterBase64Chars(s string) string {
	if strings.IndexFunc(s, func(r rune) bool {
		return !('A' <= r && r <= 'Z' || 'a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == '+' || r == '/')
	}) < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if 'A' <= r && r <= 'Z' || 'a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == '+' || r == '/' {
			b.WriteByte(byte(r))
		}
	}
	return b.String()
}

// descramble 逐2字节做3轮异或解扰（Go byte 即 uint8，移位天然截断，与 JS & 255 语义一致）
func descramble(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i += 2 {
		b1 := b[i]
		var b2 byte
		if i+1 < len(b) {
			b2 = b[i+1]
		}
		for range 3 {
			t := b2
			b2 = b1 ^ ((t >> 6) | (t << 2)) ^ (((t >> 7) | (t << 1)) & ((t >> 5) | (t << 3)))
			b1 = t
		}
		out = append(out, b1, b2)
	}
	return out
}

// parseFeatures 解析要素数组、还原差分坐标并合并 Multi 分段
func parseFeatures(raws []any) ([]Feature, error) {
	feats := make([]Feature, 0, len(raws))
	for _, raw := range raws {
		arr, ok := raw.([]any)
		if !ok || len(arr) < 2 {
			return nil, fmt.Errorf("要素结构异常: %v", raw)
		}

		styleID, ok := toInt(arr[0])
		if !ok {
			return nil, fmt.Errorf("style_id 解析失败: %v", arr[0])
		}

		segs, err := parseSegments(arr[1])
		if err != nil {
			return nil, fmt.Errorf("style_id=%d 坐标解析失败: %w", styleID, err)
		}

		feat := Feature{
			StyleID:  styleID,
			Segments: segs,
		}
		if len(arr) > 2 {
			feat.Label = arr[2]
		}
		if len(arr) > 3 {
			feat.Sign = arr[3]
		}
		feats = append(feats, feat)
	}
	return mergeMulti(feats), nil
}

// parseSegments 解析 pixed 字段：数字数组为单段，嵌套数组为多段
func parseSegments(raw any) ([]Segment, error) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("坐标字段非数组: %v", raw)
	}
	if len(arr) == 0 {
		return nil, nil
	}

	// 首元素是数组 → 嵌套多段
	if _, nested := arr[0].([]any); nested {
		segs := make([]Segment, 0, len(arr))
		for _, item := range arr {
			flat, ok := item.([]any)
			if !ok {
				return nil, fmt.Errorf("多段坐标中混入非数组元素: %v", item)
			}
			seg, err := toIntSlice(flat)
			if err != nil {
				return nil, err
			}
			segs = append(segs, restoreDelta(seg))
		}
		return segs, nil
	}

	seg, err := toIntSlice(arr)
	if err != nil {
		return nil, err
	}
	return []Segment{restoreDelta(seg)}, nil
}

// toIntSlice JSON 数字数组 → int 切片
func toIntSlice(arr []any) (Segment, error) {
	out := make(Segment, len(arr))
	for i, v := range arr {
		n, ok := toInt(v)
		if !ok {
			return nil, fmt.Errorf("坐标非数字: %v", v)
		}
		out[i] = n
	}
	return out, nil
}

// toInt JSON 数字 → int
func toInt(v any) (int, bool) {
	n, ok := v.(float64)
	if !ok {
		return 0, false
	}
	return int(n), true
}

// restoreDelta 差分还原：相邻点前缀累加（输入为 [dx0,dy0,dx1,dy1,...]）
func restoreDelta(coords Segment) Segment {
	out := make(Segment, len(coords))
	copy(out, coords)
	for i := 2; i < len(out); i += 2 {
		out[i] += out[i-2]
		out[i+1] += out[i-1]
	}
	return out
}

// mergeMulti 合并 Multi 几何：label 非字符串且 !=1 且 style_id 与前一条相同的相邻要素并入前一条。
// 这与腾讯渲染逻辑一致——多段属于同一几何的不同分段。
func mergeMulti(feats []Feature) []Feature {
	merged := make([]Feature, 0, len(feats))
	var prev *Feature

	for _, f := range feats {
		if prev != nil && mergeable(f.Label) && f.StyleID == prev.StyleID {
			prev.Segments = append(prev.Segments, f.Segments...)
			continue
		}
		merged = append(merged, f)
		prev = &merged[len(merged)-1]
	}
	return merged
}

// mergeable 判断 label 是否满足合并条件（非字符串且 !=1）
func mergeable(label any) bool {
	switch v := label.(type) {
	case nil:
		return true
	case string:
		return v == "" // JSON 空字符串视为无名称，与 undefined 同义处理
	case float64:
		return v != 1
	}
	return true
}

func preview(b []byte) string {
	const max = 64
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
