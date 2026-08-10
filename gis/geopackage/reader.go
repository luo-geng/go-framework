package geopackage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// NewReader 创建 GeoPackage 读取器
func NewReader(dbPath string) (*Reader, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite failed: %w", err)
	}
	return &Reader{dbPath: dbPath, db: db}, nil
}

// ListTables 列出所有要素表
func (r *Reader) ListTables() ([]string, error) {
	rows, err := r.db.Query(`SELECT table_name FROM gpkg_contents WHERE data_type = 'features'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return tables, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// ReadFeatures 读取要素（简化版，只读取几何）
func (r *Reader) ReadFeatures(tableName string, limit int) ([]Feature, error) {
	// 获取几何列名
	var geomCol string
	err := r.db.QueryRow(`SELECT column_name FROM gpkg_geometry_columns WHERE table_name = ?`, tableName).Scan(&geomCol)
	if err != nil {
		return nil, fmt.Errorf("get geometry column failed: %w", err)
	}

	rows, err := r.db.Query(fmt.Sprintf(`SELECT fid, %s FROM %s LIMIT ?`, geomCol, tableName), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var features []Feature
	for rows.Next() {
		var id int64
		var geomBlob []byte
		err := rows.Scan(&id, &geomBlob)
		if err != nil {
			return nil, err
		}

		// TODO: 解码 WKB（暂时跳过，只读取要素 ID）
		features = append(features, Feature{ID: id})
	}

	return features, rows.Err()
}

// Close 关闭数据库
func (r *Reader) Close() error {
	return r.db.Close()
}
