package geopackage

import (
	"database/sql"

	"github.com/twpayne/go-geom"
)

// GeometryType GeoPackage 几何类型
type GeometryType string

const (
	GeomPoint       GeometryType = "POINT"
	GeomPointZ      GeometryType = "POINTZ"
	GeomLineString  GeometryType = "LINESTRING"
	GeomLineStringZ GeometryType = "LINESTRINGZ"
	GeomPolygon     GeometryType = "POLYGON"
	GeomPolygonZ    GeometryType = "POLYGONZ"
	GeomMultiPoint  GeometryType = "MULTIPOINT"
	GeomMultiPointZ GeometryType = "MULTIPOINTZ"
)

// SpatialRefSys 空间参考系统
type SpatialRefSys struct {
	SRSID      int
	AuthName   string
	AuthSRID   int
	SRSText    string
	Definition string
}

// Column 数据表列定义
type Column struct {
	Name         string
	Type         string // INTEGER, REAL, TEXT, BLOB
	IsPrimaryKey bool
	NotNull      bool
}

// Table 要素表定义
type Table struct {
	Name             string
	GeometryColumn   string
	GeometryType     GeometryType
	SRSID            *int
	AttributeColumns []Column
}

// Feature 单个要素
type Feature struct {
	ID         int64
	Geometry   geom.T
	Attributes map[string]any
}

// Writer GeoPackage 写入器
type Writer struct {
	dbPath string
	db     *sql.DB
}

// Reader GeoPackage 读取器
type Reader struct {
	dbPath string
	db     *sql.DB
}
