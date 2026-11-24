package tilemath

import (
	"strconv"
	"strings"
)

// QuadkeyToXYZ 将 Bing Maps QuadKey 转换为 XYZ 瓦片坐标。
// QuadKey 是四叉树编码，每个字符代表一个象限 (0-3)。
func QuadkeyToXYZ(quadkey string) (x, y, z int) {
	z = len(quadkey) // 缩放级别
	for i := 0; i < z; i++ {
		mask := 1 << (z - i - 1)
		switch quadkey[i] {
		case '0': // 左上
			// x=0, y=0
		case '1': // 右上
			x |= mask
		case '2': // 左下
			y |= mask
		case '3': // 右下
			x |= mask
			y |= mask
		}
	}
	return x, y, z
}

// XYZToQuadkey 将 XYZ 瓦片坐标转换为 Bing Maps QuadKey。
// 从最高级别开始，每 bit 确定一个象限。
func XYZToQuadkey(x, y, z int) string {
	var b strings.Builder
	b.Grow(z)
	for i := z; i > 0; i-- {
		mask := 1 << (i - 1)
		digit := 0
		if (x & mask) != 0 {
			digit += 1
		}
		if (y & mask) != 0 {
			digit += 2
		}
		b.WriteString(strconv.Itoa(digit))
	}
	return b.String()
}
