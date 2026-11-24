package vector

import (
	"os"
	"testing"

	"github.com/luo-geng/go-framework/geo/tilemath"
)

// TestMVTDecode_RoundTrip tmvt → Layerize → EncodeTile → ParseMVTTile 往返：
// 要素数量一致，且能取回 class/style_id 属性。
func TestMVTDecode_RoundTrip(t *testing.T) {
	raw, err := os.ReadFile("testdata/tile_z16_x53454_y39521.tmvt")
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewClassifier()
	if err != nil {
		t.Fatal(err)
	}
	feats, err := ParseTileJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(feats) == 0 {
		t.Fatal("样本瓦片无要素")
	}

	tile := tilemath.Tile{Z: 16, X: 53454, Y: 39521}
	data, err := EncodeTile(Layerize(feats, tile.Z, c))
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseMVTTile(data, tile, ParseOptions{CRS: "gcj02"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(feats) {
		t.Fatalf("往返要素数不符: got=%d want=%d", len(got), len(feats))
	}

	// 属性应保持（抽查首个有名字的要素）
	var named *OutFeature
	for i := range got {
		if got[i].Name != "" {
			named = &got[i]
			break
		}
	}
	if named == nil {
		t.Fatal("往返后未找到带 name 的要素")
	}
	if named.Class == "" || named.Group == "" {
		t.Fatalf("属性丢失: class=%q group=%q", named.Class, named.Group)
	}
	if named.Geom == nil {
		t.Fatal("几何为空")
	}
}
