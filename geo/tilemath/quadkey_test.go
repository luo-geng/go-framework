package tilemath

import "testing"

func TestQuadkeyRoundTrip(t *testing.T) {
	cases := []struct {
		x, y, z int
		want    string
	}{
		{0, 0, 0, ""},
		{1, 1, 1, "3"},
		{3, 5, 3, "213"},
		{49, 853, 10, "2202130203"},
	}
	for _, c := range cases {
		if got := XYZToQuadkey(c.x, c.y, c.z); got != c.want {
			t.Errorf("XYZToQuadkey(%d,%d,%d) = %q, want %q", c.x, c.y, c.z, got, c.want)
		}
		x, y, z := QuadkeyToXYZ(c.want)
		if x != c.x || y != c.y || z != c.z {
			t.Errorf("QuadkeyToXYZ(%q) = %d,%d,%d, want %d,%d,%d", c.want, x, y, z, c.x, c.y, c.z)
		}
	}
}
