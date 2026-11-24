package tilegrid

import (
	"testing"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

// worldFC 覆盖全球的多边形 FeatureCollection
func worldFC() *geojson.FeatureCollection {
	p := orb.Polygon{orb.Ring{
		orb.Point{-180, -90}, orb.Point{180, -90}, orb.Point{180, 90}, orb.Point{-180, 90}, orb.Point{-180, -90},
	}}
	return geojson.NewFeatureCollection().Append(geojson.NewFeature(p))
}

// mercatorWorldFC 覆盖 Web Mercator 全球范围的多边形。
// 不使用 ±180/±90 的 worldFC：orb/tilecover 会沿对向子午线补相邻瓦片（z0 多出 1 片），
// 缩小到 ±179/±85 后覆盖结果恰为 z0:1 + z1:4。
func mercatorWorldFC() *geojson.FeatureCollection {
	p := orb.Polygon{orb.Ring{
		orb.Point{-179, -85}, orb.Point{179, -85}, orb.Point{179, 85}, orb.Point{-179, 85}, orb.Point{-179, -85},
	}}
	return geojson.NewFeatureCollection().Append(geojson.NewFeature(p))
}

// xianFC 西安钟楼附近小多边形
func xianFC() *geojson.FeatureCollection {
	p := orb.Polygon{orb.Ring{
		orb.Point{108.94, 34.25}, orb.Point{108.95, 34.25}, orb.Point{108.95, 34.27}, orb.Point{108.94, 34.27}, orb.Point{108.94, 34.25},
	}}
	return geojson.NewFeatureCollection().Append(geojson.NewFeature(p))
}

// TestTiles_WholeWorld 全球多边形在低级别的瓦片数（各矩阵定义的黄金值）
func TestTiles_WholeWorld(t *testing.T) {
	cases := []struct {
		crs      int
		fc       *geojson.FeatureCollection
		minZ     int
		maxZ     int
		wantTile int // 全部级别瓦片总数
	}{
		{3857, mercatorWorldFC(), 0, 1, 1 + 4}, // z0: 1, z1: 4
		{4326, worldFC(), 0, 1, 1 + 4},         // z0: 1x1, z1: 2x2
		{4490, worldFC(), 0, 1, 1 + 2},         // z0: 1x1, z1: 2x1（4490 矩阵高度 2^(z-1)）
	}
	for _, c := range cases {
		tiles, err := Tiles(c.fc, c.crs, c.minZ, c.maxZ)
		if err != nil {
			t.Fatalf("crs=%d: %v", c.crs, err)
		}
		if len(tiles) != c.wantTile {
			t.Errorf("crs=%d: got %d tiles, want %d", c.crs, len(tiles), c.wantTile)
		}
		assertSortedUnique(t, tiles)
	}
}

// TestTiles_SmallBBox 小范围多边形：瓦片必须落在范围内且唯一有序
func TestTiles_SmallBBox(t *testing.T) {
	// 同一 bbox 在不同矩阵下 Y 范围不同（z16 西安：3857 高约 2^16 行、4326 也是 2^16 行但跨 180°、4490 仅 2^15 行）
	cases := []struct {
		crs  int
		minY int
		maxY int
	}{
		{3857, 26090, 26150},
		{4326, 20280, 20310},
		{4490, 10140, 10160},
	}
	for _, c := range cases {
		tiles, err := Tiles(xianFC(), c.crs, 16, 16)
		if err != nil {
			t.Fatalf("crs=%d: %v", c.crs, err)
		}
		if len(tiles) == 0 {
			t.Fatalf("crs=%d: 未计算到瓦片", c.crs)
		}
		assertSortedUnique(t, tiles)
		for _, tl := range tiles {
			if tl.Z != 16 {
				t.Fatalf("crs=%d: z 不符: %+v", c.crs, tl)
			}
			// 西安 bbox 在 z16 的瓦片范围（粗略校验，防跑出范围）；X 方向三者一致
			if tl.X < 52580 || tl.X > 52640 || tl.Y < c.minY || tl.Y > c.maxY {
				t.Errorf("crs=%d: 瓦片越界: %+v", c.crs, tl)
			}
		}
	}
}

// TestTiles_Dedupe 重叠 feature 不产出重复瓦片
func TestTiles_Dedupe(t *testing.T) {
	fc := worldFC()
	fc = fc.Append(geojson.NewFeature(
		orb.Polygon{orb.Ring{
			orb.Point{0, 0}, orb.Point{90, 0}, orb.Point{90, 66.5}, orb.Point{0, 66.5}, orb.Point{0, 0},
		}}),
	)
	tiles, err := Tiles(fc, 4326, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tiles) != 4 { // z1 全球 2x2=4，重叠部分去重
		t.Fatalf("去重后应为 4 个瓦片，got %d", len(tiles))
	}
	assertSortedUnique(t, tiles)
}

// TestTiles_Errors 非法 crs / 不支持的几何类型报错
func TestTiles_Errors(t *testing.T) {
	if _, err := Tiles(worldFC(), 9999, 0, 1); err == nil {
		t.Error("非法 crs 应报错")
	}
	pointFC := geojson.NewFeatureCollection().Append(geojson.NewFeature(orb.Point{108.94, 34.26}))
	if _, err := Tiles(pointFC, 4326, 1, 1); err == nil {
		t.Error("4326 不支持 Point 几何应报错")
	}
}

// assertSortedUnique 瓦片按 (z,x,y) 升序且无重复
func assertSortedUnique(t *testing.T, tiles []Tile) {
	t.Helper()
	for i := 1; i < len(tiles); i++ {
		a, b := tiles[i-1], tiles[i]
		if a.Z > b.Z || (a.Z == b.Z && (a.X > b.X || (a.X == b.X && a.Y >= b.Y))) {
			t.Fatalf("未排序或有重复: %+v 后接 %+v", a, b)
		}
	}
}
