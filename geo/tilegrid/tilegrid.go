// Package tilegrid GeoJSON 范围 → 瓦片网格枚举。
//
// 支持三种瓦片矩阵定义：
//   - EPSG:3857 (Web Mercator)：orb/tilecover 覆盖计算
//   - EPSG:4326 (WGS84 经纬度)：2^z × 2^z 矩阵，扫描线填充
//   - EPSG:4490 (CGCS2000 经纬度)：2^z × 2^(z-1) 矩阵（天地图 c 方案），扫描线填充
//
// 结果为按 (z,x,y) 升序去重后的瓦片列表。
package tilegrid

import (
	"fmt"
	"sort"

	"github.com/paulmach/orb/geojson"

	"github.com/luo-geng/go-framework/geo/tilemath"
)

// Tile 瓦片坐标（tilemath.Tile 的别名，便于调用方使用）
type Tile = tilemath.Tile

// Tiles 按坐标系枚举 GeoJSON 覆盖的瓦片。
// crs: 3857 / 4490 / 4326，其余报错；minZ/maxZ 为缩放级别范围（含）。
// 4326/4490 仅支持 Polygon/MultiPolygon；3857 支持任意几何类型。
func Tiles(fc *geojson.FeatureCollection, crs, minZ, maxZ int) ([]Tile, error) {
	if fc == nil || len(fc.Features) == 0 {
		return nil, fmt.Errorf("tilegrid: FeatureCollection 为空")
	}
	if minZ < 0 || maxZ < minZ {
		return nil, fmt.Errorf("tilegrid: 非法 zoom 范围 %d-%d", minZ, maxZ)
	}
	var set map[Tile]struct{}
	var err error
	switch crs {
	case 3857:
		set, err = mercatorTiles(fc, minZ, maxZ)
	case 4326:
		set, err = plateCarreeTiles(fc, minZ, maxZ, false)
	case 4490:
		set, err = plateCarreeTiles(fc, minZ, maxZ, true)
	default:
		return nil, fmt.Errorf("tilegrid: 不支持的坐标系 %d（可选 3857/4490/4326）", crs)
	}
	if err != nil {
		return nil, err
	}
	return sortedTiles(set), nil
}

// sortedTiles 集合转按 (z,x,y) 升序的切片
func sortedTiles(set map[Tile]struct{}) []Tile {
	tiles := make([]Tile, 0, len(set))
	for t := range set {
		tiles = append(tiles, t)
	}
	sort.Slice(tiles, func(i, j int) bool {
		a, b := tiles[i], tiles[j]
		if a.Z != b.Z {
			return a.Z < b.Z
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.Y < b.Y
	})
	return tiles
}
