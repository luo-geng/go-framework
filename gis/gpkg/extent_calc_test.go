package gpkg

import (
	"math"
	"os"
	"testing"

	"github.com/twpayne/go-geom"
)

// TestWriter_CalculateGeometryExtent 校验从既有几何数据计算全表 extent。
// 对照 go-spatial/geom 的 CalculateGeometryExtent（其依赖 spatialite ST_MinX，
// 本库改为 Go 侧解析 GPKG 二进制 envelope，modernc sqlite 无 cgo 可用）。
func TestWriter_CalculateGeometryExtent(t *testing.T) {
	tmpFile := "/tmp/test_calc_extent.gpkg"
	defer os.Remove(tmpFile)

	w, err := NewWriter(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	srsID := 4326
	table := Table{
		Name:           "pois",
		GeometryColumn: "geom",
		GeometryType:   GeomPoint,
		SRSID:          &srsID,
		AttributeColumns: []Column{
			{Name: "name", Type: "TEXT"},
		},
	}
	if err := w.CreateTable(table); err != nil {
		t.Fatal(err)
	}

	// 两个点：(116.39, 39.91) 与 (121.47, 31.23)
	features := []Feature{
		{ID: 1, Geometry: geom.NewPointFlat(geom.XY, []float64{116.39, 39.91}), Attributes: map[string]any{"name": "a"}},
		{ID: 2, Geometry: geom.NewPointFlat(geom.XY, []float64{121.47, 31.23}), Attributes: map[string]any{"name": "b"}},
	}
	if err := w.InsertFeatures("pois", "geom", features); err != nil {
		t.Fatal(err)
	}

	extent, err := w.CalculateGeometryExtent("pois", "geom")
	if err != nil {
		t.Fatal(err)
	}
	// 返回 [minLng, minLat, maxLng, maxLat]（与 ROI/GeoJSON bbox 约定一致）
	want := [4]float64{116.39, 31.23, 121.47, 39.91}
	for i := range extent {
		if math.Abs(extent[i]-want[i]) > 1e-9 {
			t.Errorf("extent[%d] = %v，want %v（整体 %v）", i, extent[i], want[i], extent)
		}
	}

	// 空表应报错
	empty := Table{
		Name:           "empty_t",
		GeometryColumn: "geom",
		GeometryType:   GeomPoint,
		SRSID:          &srsID,
	}
	if err := w.CreateTable(empty); err != nil {
		t.Fatal(err)
	}
	if _, err := w.CalculateGeometryExtent("empty_t", "geom"); err == nil {
		t.Error("空表应返回错误")
	}
}
