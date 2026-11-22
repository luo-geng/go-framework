package gpkg_test

import (
	"encoding/binary"
	"os"
	"testing"

	gpkg "github.com/luo-geng/go-framework/gis/gpkg"
)

// TestWriter_GpkgFileIdentification 校验生成的文件符合 GeoPackage 规范标识：
// 文件头 68-71 字节为 "GPKG"（application_id），60-63 字节为 user_version（10201）。
// 参照 go-spatial/geom encoding/gpkg 的 initialSQL（PRAGMA application_id/user_version）。
func TestWriter_GpkgFileIdentification(t *testing.T) {
	tmpFile := "/tmp/test_ident.gpkg"
	defer os.Remove(tmpFile)

	w, err := gpkg.NewWriter(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	header := make([]byte, 100)
	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Read(header); err != nil {
		t.Fatal(err)
	}

	if got := string(header[68:72]); got != "GPKG" {
		t.Errorf("application_id = %q，want %q", got, "GPKG")
	}
	if got := binary.BigEndian.Uint32(header[60:64]); got != 10201 {
		t.Errorf("user_version = %d，want 10201", got)
	}
}
