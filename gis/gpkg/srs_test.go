package gpkg

import (
	"os"
	"strings"
	"testing"
)

// TestWriter_EnsureSRS 校验 KnownSRS 预置的写入与幂等性。
func TestWriter_EnsureSRS(t *testing.T) {
	tmpFile := "/tmp/test_ensure_srs.gpkg"
	defer os.Remove(tmpFile)

	w, err := NewWriter(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// 写入 3857 与 4490
	if err := w.EnsureSRS(3857, 4490); err != nil {
		t.Fatal(err)
	}

	for _, id := range []int{3857, 4490} {
		var count int
		err := w.db.QueryRow(`SELECT COUNT(*) FROM gpkg_spatial_ref_sys WHERE srs_id = ?`, id).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("srs_id=%d 应有 1 条记录，得到 %d", id, count)
		}
	}

	// 重复写入应幂等（INSERT OR IGNORE）
	if err := w.EnsureSRS(3857); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM gpkg_spatial_ref_sys WHERE srs_id = 3857`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("重复 EnsureSRS(3857) 后应有 1 条记录，得到 %d", count)
	}

	// 默认已含 4326
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM gpkg_spatial_ref_sys WHERE srs_id = 4326`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("默认应已含 srs_id=4326，得到 %d 条", count)
	}

	// 未知 SRS 应报错
	if err := w.EnsureSRS(999999); err == nil {
		t.Error("EnsureSRS(999999) 应返回未知 SRS 错误")
	}
}

// TestKnownSRS_Definition 校验预置 SRS 定义包含关键要素（ spheroid / authority ）。
func TestKnownSRS_Definition(t *testing.T) {
	checks := map[int32]string{
		4326: "WGS 84",
		3857: "Pseudo-Mercator",
		4490: "CGCS2000",
	}
	for id, kw := range checks {
		srs, ok := KnownSRS[id]
		if !ok {
			t.Errorf("KnownSRS 缺少 %d", id)
			continue
		}
		if srs.SRSID != int(id) || srs.AuthSRID != int(id) {
			t.Errorf("KnownSRS[%d] 的 SRSID/AuthSRID 应为 %d，得到 %d/%d", id, id, srs.SRSID, srs.AuthSRID)
		}
		if !strings.Contains(srs.Definition, kw) {
			t.Errorf("KnownSRS[%d] 定义应包含 %q", id, kw)
		}
	}
}
