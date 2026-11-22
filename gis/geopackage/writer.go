package geopackage

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/wkb"
	_ "modernc.org/sqlite"
)

// NewWriter 创建 GeoPackage 写入器
func NewWriter(dbPath string) (*Writer, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite failed: %w", err)
	}

	w := &Writer{dbPath: dbPath, db: db}
	if err := w.initMetadata(); err != nil {
		db.Close()
		return nil, err
	}

	return w, nil
}

// initMetadata 初始化 GeoPackage 元数据表（OGC 标准）
func (w *Writer) initMetadata() error {
	// gpkg_spatial_ref_sys - 空间参考系统表
	_, err := w.db.Exec(`
	CREATE TABLE IF NOT EXISTS gpkg_spatial_ref_sys (
		srs_name TEXT NOT NULL,
		srs_id INTEGER NOT NULL PRIMARY KEY,
		organization TEXT NOT NULL,
		organization_coordsys_id INTEGER NOT NULL,
		definition TEXT NOT NULL,
		description TEXT
	)`)
	if err != nil {
		return fmt.Errorf("create gpkg_spatial_ref_sys failed: %w", err)
	}

	// gpkg_contents - 内容表
	_, err = w.db.Exec(`
	CREATE TABLE IF NOT EXISTS gpkg_contents (
		table_name TEXT NOT NULL PRIMARY KEY,
		data_type TEXT NOT NULL,
		identifier TEXT UNIQUE,
		description TEXT DEFAULT '',
		last_change DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
		min_x DOUBLE,
		min_y DOUBLE,
		max_x DOUBLE,
		max_y DOUBLE,
		srs_id INTEGER,
		FOREIGN KEY (srs_id) REFERENCES gpkg_spatial_ref_sys(srs_id)
	)`)
	if err != nil {
		return fmt.Errorf("create gpkg_contents failed: %w", err)
	}

	// gpkg_geometry_columns - 几何列定义表
	_, err = w.db.Exec(`
	CREATE TABLE IF NOT EXISTS gpkg_geometry_columns (
		table_name TEXT NOT NULL,
		column_name TEXT NOT NULL,
		geometry_type_name TEXT NOT NULL,
		srs_id INTEGER NOT NULL,
		z TINYINT NOT NULL DEFAULT 0,
		m TINYINT NOT NULL DEFAULT 0,
		PRIMARY KEY (table_name, column_name),
		FOREIGN KEY (table_name) REFERENCES gpkg_contents(table_name),
		FOREIGN KEY (srs_id) REFERENCES gpkg_spatial_ref_sys(srs_id)
	)`)
	if err != nil {
		return fmt.Errorf("create gpkg_geometry_columns failed: %w", err)
	}

	// 插入 EPSG:4326 (WGS84)
	_, err = w.db.Exec(`
	INSERT OR IGNORE INTO gpkg_spatial_ref_sys
	(srs_name, srs_id, organization, organization_coordsys_id, definition, description)
	VALUES
	('WGS 84 geodetic', 4326, 'EPSG', 4326, 'GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]]', 'WGS84 Latitude/Longitude')
	`)
	if err != nil {
		return fmt.Errorf("insert EPSG:4326 failed: %w", err)
	}

	return nil
}

// AddSRS 添加自定义空间参考
func (w *Writer) AddSRS(srs SpatialRefSys) error {
	_, err := w.db.Exec(`
	INSERT OR REPLACE INTO gpkg_spatial_ref_sys
	(srs_name, srs_id, organization, organization_coordsys_id, definition, description)
	VALUES (?, ?, ?, ?, ?, ?)`,
		fmt.Sprintf("%s:%d", srs.AuthName, srs.AuthSRID),
		srs.SRSID,
		srs.AuthName,
		srs.AuthSRID,
		srs.Definition,
		"",
	)
	return err
}

// CreateTable 创建要素表
func (w *Writer) CreateTable(table Table) error {
	// 1. 创建用户要素表
	colSQL := "fid INTEGER PRIMARY KEY AUTOINCREMENT,\n"
	colSQL += fmt.Sprintf("  %s BLOB,\n", table.GeometryColumn)
	for _, col := range table.AttributeColumns {
		colSQL += fmt.Sprintf("  %s %s", col.Name, col.Type)
		if col.NotNull {
			colSQL += " NOT NULL"
		}
		colSQL += ",\n"
	}
	colSQL = colSQL[:len(colSQL)-2] // 移除最后一个逗号

	createSQL := fmt.Sprintf("CREATE TABLE %s (%s)", table.Name, colSQL)
	_, err := w.db.Exec(createSQL)
	if err != nil {
		return fmt.Errorf("create feature table failed: %w", err)
	}

	// 2. 注册到 gpkg_contents
	_, err = w.db.Exec(`
	INSERT INTO gpkg_contents
	(table_name, data_type, identifier, description, srs_id)
	VALUES (?, 'features', ?, ?, ?)`,
		table.Name, table.Name, fmt.Sprintf("%s features", table.Name), table.SRSID,
	)
	if err != nil {
		return fmt.Errorf("register to gpkg_contents failed: %w", err)
	}

	// 3. 注册几何列到 gpkg_geometry_columns
	zFlag := 0
	if hasZ(table.GeometryType) {
		zFlag = 1
	}
	_, err = w.db.Exec(`
	INSERT INTO gpkg_geometry_columns
	(table_name, column_name, geometry_type_name, srs_id, z, m)
	VALUES (?, ?, ?, ?, ?, 0)`,
		table.Name, table.GeometryColumn, string(table.GeometryType), table.SRSID, zFlag,
	)
	if err != nil {
		return fmt.Errorf("register geometry column failed: %w", err)
	}

	// 4. 创建空间索引（GeoPackage 标准使用虚拟表）
	_, err = w.db.Exec(fmt.Sprintf(`
	CREATE VIRTUAL TABLE rtree_%s_%s USING rtree(
		id, minX, maxX, minY, maxY
	)`, table.Name, table.GeometryColumn))
	if err != nil {
		return fmt.Errorf("create spatial index failed: %w", err)
	}

	return nil
}

// InsertFeatures 批量插入要素，几何按实际类型编码为 GPKG 二进制（WKB + 标准头）
func (w *Writer) InsertFeatures(tableName string, geomCol string, features []Feature) error {
	if len(features) == 0 {
		return nil
	}

	// 获取属性列名（以第一条要素为准，调用方保证属性集一致）
	attrNames := make([]string, 0, len(features[0].Attributes))
	for k := range features[0].Attributes {
		attrNames = append(attrNames, k)
	}

	colNames := "fid, " + geomCol
	placeholders := "?, ?"
	for _, k := range attrNames {
		colNames += ", " + k
		placeholders += ", ?"
	}

	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, colNames, placeholders))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, feat := range features {
		gpkgGeom, err := encodeGpkgGeometry(feat.Geometry, 4326)
		if err != nil {
			return fmt.Errorf("encode geometry (fid=%d) failed: %w", feat.ID, err)
		}

		args := make([]any, 0, 2+len(attrNames))
		args = append(args, feat.ID, gpkgGeom)
		for _, k := range attrNames {
			args = append(args, feat.Attributes[k])
		}

		if _, err := stmt.Exec(args...); err != nil {
			return fmt.Errorf("insert feature %d failed: %w", feat.ID, err)
		}
	}

	return tx.Commit()
}

// encodeGpkgGeometry 将 go-geom 几何编码为 GeoPackage 标准二进制：
// magic(0x4750) + version(0x00) + flags + srs_id + envelope + WKB
// flags: bit0=小端；bit1-3=envelope 类型（1=XY，2=XYZ）
func encodeGpkgGeometry(g geom.T, srsID int) ([]byte, error) {
	wkbBytes, err := wkb.Marshal(g, binary.LittleEndian)
	if err != nil {
		return nil, err
	}

	hasZ := g.Layout().ZIndex() != -1
	envIndicator := uint8(1) // XY envelope
	if hasZ {
		envIndicator = 2 // XYZ envelope
	}
	flags := byte(0x01) | byte(envIndicator<<1)

	// envelope 内容按维度取几何包围盒；空几何时写 0
	bounds := g.Bounds()
	envVals := []float64{bounds.Min(0), bounds.Max(0), bounds.Min(1), bounds.Max(1)}
	if hasZ {
		envVals = append(envVals, bounds.Min(2), bounds.Max(2))
	}

	header := make([]byte, 0, 8+len(envVals)*8)
	header = append(header, 0x47, 0x50, 0x00, flags)
	header = binary.LittleEndian.AppendUint32(header, uint32(srsID))
	for _, v := range envVals {
		header = binary.LittleEndian.AppendUint64(header, math.Float64bits(v))
	}

	return append(header, wkbBytes...), nil
}

// Close 关闭数据库
func (w *Writer) Close() error {
	return w.db.Close()
}

// hasZ 判断几何类型是否包含 Z 值
func hasZ(gt GeometryType) bool {
	switch gt {
	case GeomPointZ, GeomLineStringZ, GeomPolygonZ, GeomMultiPointZ:
		return true
	default:
		return false
	}
}
