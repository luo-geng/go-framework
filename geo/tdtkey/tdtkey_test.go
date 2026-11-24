package tdtkey

import "testing"

// 黄金值来自逆向来源的 JS 参考实现（tdt-util.js 的 shapeshift，randomKey 恒为 "9876"）
func TestXYZToTdtKey_Golden(t *testing.T) {
	cases := []struct {
		x, y, z int
		want    string
	}{
		{6, 3, 3, "gjda"},
		{53454, 26046, 16, "aj2g2mjdgadgjda"},
		{0, 0, 0, "a"},
		{1, 1, 1, "ja"},
		{123, 456, 10, "d1ajd1mj1da"},
		{65535, 32768, 16, "d5g3d4ja"},
		{7, 7, 3, "j2a"},
	}
	for _, c := range cases {
		if got := XYZToTdtKey(c.x, c.y, c.z); got != c.want {
			t.Errorf("XYZToTdtKey(%d,%d,%d) = %q, want %q", c.x, c.y, c.z, got, c.want)
		}
	}
}
