package geopackage

import (
	"database/sql"
	"fmt"
	"math"

	"github.com/twpayne/go-geom"
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

// InsertFeatures 批量插入要素
func (w *Writer) InsertFeatures(tableName string, geomCol string, features []Feature) error {
	if len(features) == 0 {
		return nil
	}

	// 构建插入 SQL
	firstFeat := features[0]
	colNames := "fid, " + geomCol
	placeholders := "?, ?"

	// 获取属性列名
	attrNames := make([]string, 0, len(firstFeat.Attributes))
	for k := range firstFeat.Attributes {
		attrNames = append(attrNames, k)
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
		// 编码 WKB
		poly := feat.Geometry.(*geom.Polygon)
		wkbBytes := marshalPolygonZ(poly)

		// 计算包围盒
		coords := poly.Coords()[0] // 外环
		minX, maxX := coords[0][0], coords[0][0]
		minY, maxY := coords[0][1], coords[0][1]
		for _, pt := range coords {
			if pt[0] < minX {
				minX = pt[0]
			}
			if pt[0] > maxX {
				maxX = pt[0]
			}
			if pt[1] < minY {
				minY = pt[1]
			}
			if pt[1] > maxY {
				maxY = pt[1]
			}
		}

		// GeoPackage 标准几何头：
		// magic (0x4750) + version (0x00) + flags + srs_id (4326) + envelope
		// flags: 0x23 = 00100011 (little endian + 2D envelope + has Z)
		gpkgHeader := make([]byte, 0, 40)
		gpkgHeader = append(gpkgHeader, 0x47, 0x50)                // magic
		gpkgHeader = append(gpkgHeader, 0x00)                        // version
		gpkgHeader = append(gpkgHeader, 0x23)                        // flags
		gpkgHeader = putUint32(gpkgHeader, 4326)                     // srs_id
		gpkgHeader = putFloat64(gpkgHeader, minX)                    // min_x
		gpkgHeader = putFloat64(gpkgHeader, maxX)                    // max_x
		gpkgHeader = putFloat64(gpkgHeader, minY)                    // min_y
		gpkgHeader = putFloat64(gpkgHeader, maxY)                    // max_y

		gpkgGeom := append(gpkgHeader, wkbBytes...)

		// 构建参数
		args := []interface{}{feat.ID, gpkgGeom}
		for _, k := range attrNames {
			args = append(args, feat.Attributes[k])
		}

		_, err = stmt.Exec(args...)
		if err != nil {
			return fmt.Errorf("insert feature %d failed: %w", feat.ID, err)
		}
	}

	return tx.Commit()
}

// Close 关闭数据库
func (w *Writer) Close() error {
	return w.db.Close()
}

// marshalPolygonZ 将 Polygon 编码为 WKB PolygonZ 格式
func marshalPolygonZ(poly *geom.Polygon) []byte {
	coords := poly.Coords()
	if len(coords) == 0 {
		return nil
	}

	buf := make([]byte, 0, 2048)

	// WKB 头：byte order (1 = little endian)
	buf = append(buf, 1)

	// WKB 类型：PolygonZ = 1003 (OGC SFS 1.2 标准)
	buf = putUint32(buf, 1003)

	// 环数量
	buf = putUint32(buf, uint32(len(coords)))

	// 每个环
	for _, ring := range coords {
		// 点数量
		buf = putUint32(buf, uint32(len(ring)))
		// 每个点的坐标 (x, y, z) - WKB 要求使用 float64 (8字节)
		for _, pt := range ring {
			buf = putFloat64(buf, pt[0])
			buf = putFloat64(buf, pt[1])
			buf = putFloat64(buf, pt[2])
		}
	}

	return buf
}

// putFloat64 以小端序写入 float64 (WKB 规范要求 8 字节)
func putFloat64(b []byte, v float64) []byte {
	bits := math.Float64bits(v)
	return append(b,
		byte(bits),
		byte(bits>>8),
		byte(bits>>16),
		byte(bits>>24),
		byte(bits>>32),
		byte(bits>>40),
		byte(bits>>48),
		byte(bits>>56),
	)
}

// putUint32 以小端序写入 uint32
func putUint32(b []byte, v uint32) []byte {
	return append(b,
		byte(v),
		byte(v>>8),
		byte(v>>16),
		byte(v>>24),
	)
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
