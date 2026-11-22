package gpkg_test

import (
	"fmt"
	"os"
	"testing"

	gpkg "github.com/luo-geng/go-framework/gis/gpkg"
	"github.com/twpayne/go-geom"
)

func TestWriter_PolygonZ(t *testing.T) {
	tmpFile := "/tmp/test_polygonz.gpkg"
	defer os.Remove(tmpFile)

	srsID := 4326

	// 创建写入器
	w, err := gpkg.NewWriter(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// 定义表结构
	table := gpkg.Table{
		Name:           "flood_triangles",
		GeometryColumn: "geom",
		GeometryType:   gpkg.GeomPolygonZ,
		SRSID:          &srsID,
		AttributeColumns: []gpkg.Column{
			{Name: "tri_id", Type: "INTEGER"},
			{Name: "water_level", Type: "REAL"},
			{Name: "water_depth", Type: "REAL"},
		},
	}

	if err := w.CreateTable(table); err != nil {
		t.Fatal(err)
	}

	// 插入一个 PolygonZ 三角形
	features := []gpkg.Feature{
		{
			ID: 1,
			Geometry: geom.NewPolygonFlat(geom.XYZ, []float64{
				116.0, 39.0, 100.5, // 点1 lng, lat, z
				116.1, 39.0, 101.0, // 点2
				116.1, 39.1, 100.8, // 点3
				116.0, 39.0, 100.5, // 闭合
			}, []int{4}),
			Attributes: map[string]any{
				"tri_id":      0,
				"water_level": 100.8,
				"water_depth": 2.5,
			},
		},
	}

	if err := w.InsertFeatures("flood_triangles", "geom", features); err != nil {
		t.Fatal(err)
	}

	fmt.Println("GeoPackage PolygonZ 写入成功:", tmpFile)
}
