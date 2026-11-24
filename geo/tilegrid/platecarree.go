package tilegrid

import (
	"fmt"
	"math"
	"slices"
	"sync"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
)

// plateCarreeTiles 经纬度矩阵（4326 或 4490）扫描线填充。
// cgcs2000=true 时使用 4490 矩阵（宽度 2^z、高度 2^(z-1)），否则 4326 方阵（2^z × 2^z）。
// 仅支持 Polygon/MultiPolygon。
func plateCarreeTiles(fc *geojson.FeatureCollection, minZ, maxZ int, cgcs2000 bool) (map[Tile]struct{}, error) {
	set := make(map[Tile]struct{})
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	sem := make(chan struct{}, 4)
	for _, f := range fc.Features {
		for z := minZ; z <= maxZ; z++ {
			sem <- struct{}{}
			wg.Add(1)
			go func(feat *geojson.Feature, zoom int) {
				defer wg.Done()
				defer func() { <-sem }()
				geomSet, err := scanGeometry(feat.Geometry, zoom, cgcs2000)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				mu.Lock()
				for t := range geomSet {
					set[Tile{Z: int(t.Z), X: int(t.X), Y: int(t.Y)}] = struct{}{}
				}
				mu.Unlock()
			}(f, z)
		}
	}
	wg.Wait()
	return set, firstErr
}

// scanGeometry 将几何体转换为经纬度矩阵瓦片集合（扫描线 + 多点采样奇偶填充）
func scanGeometry(g orb.Geometry, z int, cgcs2000 bool) (maptile.Set, error) {
	if g == nil {
		return nil, nil
	}
	set := make(maptile.Set)
	switch geom := g.(type) {
	case orb.Polygon:
		scanPolygon(geom, z, cgcs2000, set)
	case orb.MultiPolygon:
		for _, p := range geom {
			scanPolygon(p, z, cgcs2000, set)
		}
	default:
		return nil, fmt.Errorf("tilegrid: 不支持的几何类型 %T（经纬度矩阵仅支持 Polygon/MultiPolygon）", g)
	}
	return set, nil
}

// scanPolygon 扫描线算法：按行(Y)遍历，多点采样（顶部/中部/底部）+ 奇偶规则填充。
// 多点采样避免低级别大瓦片的中心点落在多边形外导致漏瓦片。
// 复杂度 O(E + N)，E 为边数、N 为瓦片数。
func scanPolygon(polygon orb.Polygon, z int, cgcs2000 bool, set maptile.Set) {
	bound := polygon.Bound()
	minLon, minLat := bound.Min.Lon(), bound.Min.Lat()
	maxLon, maxLat := bound.Max.Lon(), bound.Max.Lat()

	minT := lonLatToTileXY(minLon, maxLat, z, cgcs2000)
	maxT := lonLatToTileXY(maxLon, minLat, z, cgcs2000)
	minY, maxY := minT.Y, maxT.Y
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	xMin, xMax := minT.X, maxT.X
	if xMin > xMax {
		xMin, xMax = xMax, xMin
	}

	// 经度方向瓦片宽 360°/width；纬度方向行高 180°/height（4490 与 4326 高度不同，不可混用）
	lonSize := 360.0 / float64(matrixWidth(z, cgcs2000))
	latSize := 180.0 / float64(matrixHeight(z, cgcs2000))

	for y := minY; y <= maxY; y++ {
		scanLats := []float64{
			90.0 - float64(y)*latSize,       // 瓦片顶部
			90.0 - (float64(y)+0.5)*latSize, // 瓦片中心
			90.0 - (float64(y)+1.0)*latSize, // 瓦片底部
		}
		for _, scanLat := range scanLats {
			var xs []float64
			for _, ring := range polygon {
				collectIntersections(ring, scanLat, &xs)
			}
			if len(xs) < 2 {
				continue
			}
			slices.Sort(xs)

			// 奇偶规则：奇数交点到偶数交点之间在多边形内
			for i := 0; i+1 < len(xs); i += 2 {
				xStart := uint32(math.Floor((xs[i] + 180.0) / lonSize))
				xEnd := uint32(math.Floor((xs[i+1] + 180.0) / lonSize))
				if xStart < xMin {
					xStart = xMin
				}
				if xEnd > xMax {
					xEnd = xMax
				}
				if xStart >= matrixWidth(z, cgcs2000) {
					continue
				}
				for x := xStart; x <= xEnd; x++ {
					set[maptile.New(x, y, maptile.Zoom(z))] = true
				}
			}
		}
	}
}

// collectIntersections 收集环与水平扫描线的交点经度（射线法边相交，线性插值）
func collectIntersections(ring orb.Ring, scanLat float64, xs *[]float64) {
	if len(ring) < 4 {
		return
	}
	for i := 0; i+1 < len(ring); i++ {
		p1, p2 := ring[i], ring[i+1]
		y1, y2 := p1.Lat(), p2.Lat()
		// 扫描线与边不相交（顶点处取半开区间，避免重复计数）
		if (y1 > scanLat) == (y2 > scanLat) {
			continue
		}
		if y2 == y1 {
			continue // 水平边除零保护
		}
		*xs = append(*xs, p1.Lon()+(p2.Lon()-p1.Lon())*(scanLat-y1)/(y2-y1))
	}
}

// matrixHeight 矩阵高度：4490 为 2^(z-1)（z=0 时为 1），4326 为 2^z
func matrixHeight(z int, cgcs2000 bool) uint32 {
	if !cgcs2000 {
		return uint32(1) << z
	}
	if z <= 0 {
		return 1
	}
	return uint32(1) << (z - 1)
}

// matrixWidth 矩阵宽度恒为 2^z
func matrixWidth(z int, _ bool) uint32 {
	return uint32(1) << z
}

// lonLatToTileXY 经纬度 → 瓦片坐标；Y 轴从北极向下
func lonLatToTileXY(lon, lat float64, z int, cgcs2000 bool) maptile.Tile {
	width := matrixWidth(z, cgcs2000)
	height := matrixHeight(z, cgcs2000)

	// 经度环绕；lon=180 保持原值，由边界保护归入最后一列
	if lon < -180.0 {
		lon += 360.0
	} else if lon > 180.0 {
		lon -= 360.0
	}
	x := uint32(math.Floor((lon + 180.0) / (360.0 / float64(width))))
	if x >= width {
		x = width - 1
	}

	if lat > 90.0 {
		lat = 90.0
	} else if lat < -90.0 {
		lat = -90.0
	}
	y := uint32(math.Floor((90.0 - lat) / (180.0 / float64(height))))
	if y >= height {
		y = height - 1
	}
	return maptile.New(x, y, maptile.Zoom(z))
}
