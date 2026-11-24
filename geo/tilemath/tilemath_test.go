// 测试向量来自 @mapbox/tilebelt 实测输出。
package tilemath

import (
	"math"
	"testing"
)

const tolerance = 1e-6

func TestLonLatToTile(t *testing.T) {
	cases := []struct {
		lng, lat float64
		z        int
		wantX    int
		wantY    int
	}{
		{113.63, 34.75, 18, 213814, 104056}, // 郑州
		{116.404, 39.915, 16, 53958, 24830}, // 北京
		{121.4737, 31.2304, 3, 6, 3},        // 上海
	}
	for _, c := range cases {
		x, y := LonLatToTile(c.lng, c.lat, c.z)
		if x != c.wantX || y != c.wantY {
			t.Errorf("LonLatToTile(%v, %v, %d) = (%d, %d)，want (%d, %d)",
				c.lng, c.lat, c.z, x, y, c.wantX, c.wantY)
		}
	}
}

func TestTMSRoundTrip(t *testing.T) {
	for _, z := range []int{3, 10, 18} {
		for _, y := range []int{0, 5, 100} {
			if got := TileToTMS(TMSToTile(y, z), z); got != y {
				t.Errorf("z=%d y=%d 往返转换得到 %d", z, y, got)
			}
		}
	}
}

func TestTileBBox(t *testing.T) {
	bbox := TileBBox(6, 3, 3)
	want := [4]float64{90.0, 0.0, 135.0, 40.979898}
	for i := range bbox {
		if math.Abs(bbox[i]-want[i]) > 1e-5 {
			t.Errorf("TileBBox(6, 3, 3)[%d] = %v，want %v", i, bbox[i], want[i])
		}
	}
}

func TestPixelToLonLatTMS(t *testing.T) {
	// tilebelt tileToBBOX([213861, 262143-158132, 18]) 内插值参照
	lng, lat := PixelToLonLatTMS(Tile{Z: 18, X: 213861, Y: 158132}, 100, 150)
	wantLng, wantLat := 113.6939263343811, 34.800739274508594
	if math.Abs(lng-wantLng) > tolerance || math.Abs(lat-wantLat) > tolerance {
		t.Errorf("PixelToLonLatTMS = (%v, %v)，want (%v, %v)", lng, lat, wantLng, wantLat)
	}
}

func TestTilesFromBBox(t *testing.T) {
	// 以两个对角相邻瓦片的中心为角点（外扩微小 ε），应恰好覆盖 2×2 共 4 个瓦片
	sw := TileBBox(6, 3, 3)
	ne := TileBBox(7, 2, 3)
	const eps = 1e-7
	bbox := [4]float64{
		(sw[0]+sw[2])/2 - eps, (sw[1]+sw[3])/2 - eps,
		(ne[0]+ne[2])/2 + eps, (ne[1]+ne[3])/2 + eps,
	}

	tiles := TilesFromBBox(bbox, 3)
	if len(tiles) != 4 {
		t.Fatalf("TilesFromBBox 应产出 4 个瓦片，得到 %d 个", len(tiles))
	}
	for _, t2 := range tiles {
		if t2.X < 6 || t2.X > 7 || TMSToTile(t2.Y, 3) < 2 || TMSToTile(t2.Y, 3) > 3 {
			t.Errorf("产出范围外瓦片: %+v", t2)
		}
	}
}
