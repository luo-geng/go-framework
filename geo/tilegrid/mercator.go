package tilegrid

import (
	"sync"

	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/maptile/tilecover"
)

// mercatorTiles EPSG:3857 (Web Mercator)：orb/tilecover 覆盖计算，支持任意几何类型
func mercatorTiles(fc *geojson.FeatureCollection, minZ, maxZ int) (map[Tile]struct{}, error) {
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
				s, err := tilecover.Geometry(feat.Geometry, maptile.Zoom(zoom))
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				mu.Lock()
				for t, covered := range s {
					if covered {
						set[Tile{Z: int(t.Z), X: int(t.X), Y: int(t.Y)}] = struct{}{}
					}
				}
				mu.Unlock()
			}(f, z)
		}
	}
	wg.Wait()
	return set, firstErr
}
