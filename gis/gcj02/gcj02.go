// Package gcj02 实现 WGS84 与 GCJ02（火星坐标系）互转。
// 公式移植自 gcoord（腾讯/高德底图使用 GCJ02，输入范围约定 WGS84）。
package gcj02

import "math"

const (
	// 长半轴与偏心率平方（CGCS2000/Krasovsky 近似）
	a  = 6378245.0
	ee = 0.00669342162296594323
)

// outOfChina 判断坐标是否在中国境外（境外不做偏移）
func outOfChina(lng, lat float64) bool {
	return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271
}

// transformLat 纬度偏移量多项式
func transformLat(lng, lat float64) float64 {
	ret := -100.0 + 2.0*lng + 3.0*lat + 0.2*lat*lat + 0.1*lng*lat + 0.2*math.Sqrt(math.Abs(lng))
	ret += (20.0*math.Sin(6.0*lng*math.Pi) + 20.0*math.Sin(2.0*lng*math.Pi)) * 2.0 / 3.0
	ret += (20.0*math.Sin(lat*math.Pi) + 40.0*math.Sin(lat/3.0*math.Pi)) * 2.0 / 3.0
	ret += (160.0*math.Sin(lat/12.0*math.Pi) + 320*math.Sin(lat*math.Pi/30.0)) * 2.0 / 3.0
	return ret
}

// transformLng 经度偏移量多项式
func transformLng(lng, lat float64) float64 {
	ret := 300.0 + lng + 2.0*lat + 0.1*lng*lng + 0.1*lng*lat + 0.1*math.Sqrt(math.Abs(lng))
	ret += (20.0*math.Sin(6.0*lng*math.Pi) + 20.0*math.Sin(2.0*lng*math.Pi)) * 2.0 / 3.0
	ret += (20.0*math.Sin(lng*math.Pi) + 40.0*math.Sin(lng/3.0*math.Pi)) * 2.0 / 3.0
	ret += (150.0*math.Sin(lng/12.0*math.Pi) + 300.0*math.Sin(lng/30.0*math.Pi)) * 2.0 / 3.0
	return ret
}

// delta 计算某点的 GCJ02 偏移量（经纬度各分量的增量）
func delta(lng, lat float64) (dLng, dLat float64) {
	dLat = transformLat(lng-105.0, lat-35.0)
	dLng = transformLng(lng-105.0, lat-35.0)

	radLat := lat / 180.0 * math.Pi
	magic := math.Sin(radLat)
	magic = 1 - ee*magic*magic
	sqrtMagic := math.Sqrt(magic)

	dLat = dLat * 180.0 / ((a * (1.0 - ee) / (magic * sqrtMagic)) * math.Pi)
	dLng = dLng * 180.0 / (a / sqrtMagic * math.Cos(radLat) * math.Pi)
	return dLng, dLat
}

// WGS84ToGCJ02 WGS84 → GCJ02
func WGS84ToGCJ02(lng, lat float64) (float64, float64) {
	if outOfChina(lng, lat) {
		return lng, lat
	}
	dLng, dLat := delta(lng, lat)
	return lng + dLng, lat + dLat
}

// GCJ02ToWGS84 GCJ02 → WGS84。
// 逆变换无解析解，采用定点迭代 2 次（与 gcoord 一致，误差 ~1e-8 度，约 1 毫米）。
func GCJ02ToWGS84(lng, lat float64) (float64, float64) {
	if outOfChina(lng, lat) {
		return lng, lat
	}
	// 先用 delta(gcj) 粗估 WGS84，再用估计点的 delta 修正一轮即可收敛
	wgsLng, wgsLat := lng, lat
	for range 2 {
		dLng, dLat := delta(wgsLng, wgsLat)
		wgsLng = lng - dLng
		wgsLat = lat - dLat
	}
	return wgsLng, wgsLat
}
