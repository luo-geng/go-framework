// Package tilemath 提供 EPSG:3857 Web Mercator 瓦片切片与瓦片内像素坐标换算。
// 逻辑等价于原 JS 实现（@mapbox/tilebelt + pixel_to_lat_lng_tms）。
package tilemath

import "math"

const (
	// mercatorMaxLat 是 Web Mercator 有效纬度上限（±85.05112878°），超出会导致切片越界
	mercatorMaxLat = 85.05112878
	// tilePixelSize 单瓦片像素边长
	tilePixelSize = 256
)

// Tile 表示一个瓦片索引；Y 一律为 TMS 坐标（腾讯接口使用 TMS）
type Tile struct {
	Z int
	X int
	Y int
}

// LonLatToTile 经纬度 → XYZ 瓦片索引（lat 会被截断到 Mercator 有效范围）
func LonLatToTile(lng, lat float64, z int) (x, y int) {
	lat = clamp(lat, -mercatorMaxLat, mercatorMaxLat)
	n := math.Exp2(float64(z))
	x = int(math.Floor((lng + 180.0) / 360.0 * n))
	y = int(math.Floor((1.0 - math.Log(math.Tan(lat*math.Pi/180.0)+1.0/math.Cos(lat*math.Pi/180.0))/math.Pi) / 2.0 * n))
	return x, y
}

// TileToTMS XYZ y → TMS y
func TileToTMS(y, z int) int {
	return int(math.Exp2(float64(z))) - 1 - y
}

// TMSToTile TMS y → XYZ y
func TMSToTile(yTMS, z int) int {
	return int(math.Exp2(float64(z))) - 1 - yTMS
}

// TileBBox 返回 XYZ 瓦片的经纬度包围盒 [minLng, minLat, maxLng, maxLat]
func TileBBox(x, yXYZ, z int) [4]float64 {
	n := math.Exp2(float64(z))

	minLng := float64(x)/n*360.0 - 180.0
	maxLng := float64(x+1)/n*360.0 - 180.0

	// atan/sinh 结果弧度转角度
	maxLat := mercatorToLat(1 - 2*float64(yXYZ)/n)
	minLat := mercatorToLat(1 - 2*float64(yXYZ+1)/n)

	return [4]float64{minLng, minLat, maxLng, maxLat}
}

// mercatorToLat Web Mercator y 参数（[-1,1]）→ 纬度（度）
func mercatorToLat(y float64) float64 {
	return math.Atan(math.Sinh(math.Pi*y)) * 180.0 / math.Pi
}

// PixelToLonLatTMS 将 TMS 瓦片内的像素坐标转换为 GCJ02 经纬度。
// 等价于：先取瓦片 bbox，再在 bbox 内做线性插值（y 轴向下，故纬度从上往下递减）。
func PixelToLonLatTMS(t Tile, px, py float64) (lng, lat float64) {
	bbox := TileBBox(t.X, TMSToTile(t.Y, t.Z), t.Z)
	minLng, minLat, maxLng, maxLat := bbox[0], bbox[1], bbox[2], bbox[3]

	lng = minLng + px/tilePixelSize*(maxLng-minLng)
	lat = maxLat - py/tilePixelSize*(maxLat-minLat)
	return lng, lat
}

// TilesFromBBox 返回覆盖 WGS84 包围盒 [minLng, minLat, maxLng, maxLat] 的全部 TMS 瓦片。
// bbox 需已由调用方转为 GCJ02（与本包输出的经纬度同坐标系）。
func TilesFromBBox(bbox [4]float64, z int) []Tile {
	swX, swY := LonLatToTile(bbox[0], bbox[1], z)
	neX, neY := LonLatToTile(bbox[2], bbox[3], z)

	var tiles []Tile
	for x := swX; x <= neX; x++ {
		for y := neY; y <= swY; y++ {
			tiles = append(tiles, Tile{Z: z, X: x, Y: TileToTMS(y, z)})
		}
	}
	return tiles
}

func clamp(v, lo, hi float64) float64 {
	return min(max(v, lo), hi)
}
