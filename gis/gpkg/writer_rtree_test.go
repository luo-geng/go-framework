package gpkg_test

import (
	"database/sql"
	"os"
	"testing"

	gpkg "github.com/luo-geng/go-framework/gis/gpkg"
	"github.com/twpayne/go-geom"
)

// TestWriter_BuildRTreeIndex 校验 BuildRTreeIndex 能从 GPKG 二进制头的 envelope
// 填充 rtree 虚拟表（不依赖 spatialite 的 ST_MinX 系列函数，纯 Go 解析）。
func TestWriter_BuildRTreeIndex(t *testing.T) {
	tmpFile := "/tmp/test_rtree.gpkg"
	defer os.Remove(tmpFile)

	srsID := 4326
	w, err := gpkg.NewWriter(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.CreateTable(gpkg.Table{
		Name:           "roads",
		GeometryColumn: "geom",
		GeometryType:   gpkg.GeomLineString,
		SRSID:          &srsID,
	}); err != nil {
		t.Fatal(err)
	}

	features := []gpkg.Feature{
		{ID: 1, Geometry: geom.NewLineStringFlat(geom.XY, []float64{113.6, 34.7, 113.65, 34.75})},
		{ID: 2, Geometry: geom.NewLineStringFlat(geom.XY, []float64{113.7, 34.8, 113.75, 34.85})},
	}
	if err := w.InsertFeatures("roads", "geom", features); err != nil {
		t.Fatal(err)
	}
	if err := w.BuildRTreeIndex("roads", "geom"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 两条要素的 envelope 都应在 rtree 中
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM rtree_roads_geom`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("rtree 记录数 = %d，want 2", count)
	}

	// 空间查询验证：rtree 返回落在窗口内的 fid
	var fid int
	err = db.QueryRow(`SELECT id FROM rtree_roads_geom WHERE minX >= 113.69 AND maxX <= 113.76 LIMIT 1`).Scan(&fid)
	if err != nil {
		t.Fatalf("rtree 窗口查询失败: %v", err)
	}
	if fid != 2 {
		t.Errorf("窗口查询命中 fid = %d，want 2", fid)
	}
}
