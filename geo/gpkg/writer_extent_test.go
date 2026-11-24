package gpkg_test

import (
	"database/sql"
	"os"
	"testing"

	gpkg "github.com/luo-geng/go-framework/geo/gpkg"
	"github.com/twpayne/go-geom"
)

// TestWriter_InsertFeatures_MaintainsContentsExtent 校验批量插入后
// gpkg_contents 中该表的 min_x/min_y/max_x/max_y 为已插入几何的联合包围盒。
// 参照 go-spatial/geom encoding/gpkg 的 UpdateGeometryExtent 行为。
func TestWriter_InsertFeatures_MaintainsContentsExtent(t *testing.T) {
	tmpFile := "/tmp/test_extent.gpkg"
	defer os.Remove(tmpFile)

	srsID := 4326
	w, err := gpkg.NewWriter(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if err := w.CreateTable(gpkg.Table{
		Name:           "pois",
		GeometryColumn: "geom",
		GeometryType:   gpkg.GeomPoint,
		SRSID:          &srsID,
	}); err != nil {
		t.Fatal(err)
	}

	// 两批插入，第二批包围盒更大：extent 应取联合而非覆盖
	batch1 := []gpkg.Feature{
		{ID: 1, Geometry: geom.NewPointFlat(geom.XY, []float64{113.6, 34.7})},
		{ID: 2, Geometry: geom.NewPointFlat(geom.XY, []float64{113.7, 34.8})},
	}
	batch2 := []gpkg.Feature{
		{ID: 3, Geometry: geom.NewPointFlat(geom.XY, []float64{113.5, 34.6})},
	}
	if err := w.InsertFeatures("pois", "geom", batch1); err != nil {
		t.Fatal(err)
	}
	if err := w.InsertFeatures("pois", "geom", batch2); err != nil {
		t.Fatal(err)
	}

	// 关闭后独立读取，确保数据已提交
	w.Close()

	db, err := sql.Open("sqlite", tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var minX, minY, maxX, maxY float64
	err = db.QueryRow(`SELECT min_x, min_y, max_x, max_y FROM gpkg_contents WHERE table_name = 'pois'`).
		Scan(&minX, &minY, &maxX, &maxY)
	if err != nil {
		t.Fatal(err)
	}

	if minX != 113.5 || minY != 34.6 || maxX != 113.7 || maxY != 34.8 {
		t.Errorf("extent = (%v, %v, %v, %v)，want (113.5, 34.6, 113.7, 34.8)",
			minX, minY, maxX, maxY)
	}
}
