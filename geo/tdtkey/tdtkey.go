// Package tdtkey 天地图瓦片加密索引（pk 参数）计算。
// 逐字符复刻逆向来源的 JS 参考实现（tdt-util.js），randomKey 恒为 "9876"。
package tdtkey

import (
	"strconv"
	"strings"
)

// XYZToTdtKey 将 XYZ 瓦片坐标转换为天地图加密索引 (pk)。
// 核心步骤：构建二进制象限序列（类似 QuadKey 但多一位）、反转、
// 对索引 9,8,7,6 位置的值 +1，最后映射为字母并按重复计数压缩。
func XYZToTdtKey(x, y, z int) string {
	var bitStr strings.Builder
	// 构建二进制象限序列（a 从 z 到 0，比 QuadKey 多最高一位）
	for a := z; a >= 0; a-- {
		mask := 1 << a
		var bits int
		if (x & mask) != 0 {
			bits |= 1
		}
		if (y & mask) != 0 {
			bits |= 2
		}
		bitStr.WriteString(strconv.Itoa(bits))
	}

	reversed := reverseString(bitStr.String())
	modified := bumpIndexes(reversed)
	return encryptDigits(modified)
}

// reverseString 反转字符串
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// bumpIndexes 对索引 9,8,7,6 位置的值 +1（天地图特有算法）
func bumpIndexes(s string) string {
	runes := []rune(s)
	for _, idx := range []int{9, 8, 7, 6} {
		if idx >= len(runes) {
			continue
		}
		num, _ := strconv.Atoi(string(runes[idx]))
		runes[idx] = []rune(strconv.Itoa(num + 1))[0]
	}
	return string(runes)
}

// encryptDigits 将 0-3 的数字映射为 a/d/g/j（值 +1 后可为 m），连续重复以 字符+计数 压缩。
// 逐字符复刻 JS 的 digitToEncrypt：重复时计数数字原位替换，
// add==1 时追加，add>10 时左移一位（兼容多位计数）。
func encryptDigits(digit string) string {
	s := ""
	compress := ""
	add := 0

	for _, char := range digit {
		num, _ := strconv.Atoi(string(char))
		current := string(rune(97 + 3*num)) // 0->a, 1->d, 2->g, 3->j

		if current == compress {
			add++
			last := len(s) - 1
			if add == 1 {
				last++ // 首次重复：计数追加在末位之后
			} else if add > 10 {
				last-- // 两位计数：替换两个数字位（JS slice(0,-1) 语义）
			}
			if last < 0 {
				last = 0
			}
			s = s[:last] + strconv.Itoa(add)
		} else {
			s += current
			compress = current
			add = 0
		}
	}

	return s
}
