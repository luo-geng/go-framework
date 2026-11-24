// 串联 ROI 计算、瓦片解析、过滤去重与输出。
package vector

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"encoding/json/v2"

	"github.com/luo-geng/go-framework/gis/gcj02"
	"github.com/luo-geng/go-framework/gis/tilemath"
)

// Point 经纬度点 [lng, lat]
type LonLat = [2]float64

// Ring 闭合坐标环
type Ring []LonLat

// ROI 感兴趣区域：WGS84 输入转 GCJ02 后的多边形集合 + 总包围盒
type ROI struct {
	Polygons []Ring     // 各多边形外环（ROI 过滤只用外环），GCJ02
	BBox     [4]float64 // 总包围盒 [minLng, minLat, maxLng, maxLat]，GCJ02
}

// LoadROI 从 GeoJSON 文件（面/MultiPolygon 要素集合）加载 ROI。
// 输入坐标约定 WGS84（EPSG:4326），内部统一转 GCJ02 以匹配腾讯瓦片坐标系。
func LoadROI(geojsonPath string) (*ROI, error) {
	raw, err := os.ReadFile(geojsonPath)
	if err != nil {
		return nil, err
	}

	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			Geometry struct {
				Type        string `json:"type"`
				Coordinates any    `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, fmt.Errorf("GeoJSON 解析失败: %w", err)
	}

	roi := &ROI{BBox: [4]float64{180, 90, -180, -90}}
	for _, f := range fc.Features {
		rings, err := extractRings(f.Geometry.Type, f.Geometry.Coordinates)
		if err != nil {
			return nil, err
		}
		for _, ring := range rings {
			gcjRing := transformRing(ring)
			roi.Polygons = append(roi.Polygons, gcjRing)
			roi.BBox = mergeBBox(roi.BBox, ringBBox(gcjRing))
		}
	}
	if len(roi.Polygons) == 0 {
		return nil, fmt.Errorf("GeoJSON 中未找到面要素")
	}
	return roi, nil
}

// ParseBBox 解析 "minLng,minLat,maxLng,maxLat"（WGS84）为 ROI
func ParseBBox(s string) (*ROI, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("bbox 格式应为 minLng,minLat,maxLng,maxLat，得到: %q", s)
	}
	vals := make([]float64, 4)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("bbox 第 %d 个分量解析失败: %w", i+1, err)
		}
		vals[i] = v
	}

	// 以 GCJ02 bbox 四角构造矩形多边形
	minLng, minLat := gcj02.WGS84ToGCJ02(vals[0], vals[1])
	maxLng, maxLat := gcj02.WGS84ToGCJ02(vals[2], vals[3])
	rect := Ring{{minLng, minLat}, {maxLng, minLat}, {maxLng, maxLat}, {minLng, maxLat}, {minLng, minLat}}
	return &ROI{
		Polygons: []Ring{rect},
		BBox:     [4]float64{minLng, minLat, maxLng, maxLat},
	}, nil
}

// Tiles 返回覆盖 ROI 的全部 TMS 瓦片
func (r *ROI) Tiles(z int) []tilemath.Tile {
	return tilemath.TilesFromBBox(r.BBox, z)
}

// ContainsPoint 判断 GCJ02 点是否在 ROI 内（射线法，仅外环）
func (r *ROI) ContainsPoint(lng, lat float64) bool {
	for _, ring := range r.Polygons {
		if pointInRing(lng, lat, ring) {
			return true
		}
	}
	return false
}

// IntersectsBBox 判断 GCJ02 包围盒是否与 ROI 总包围盒相交（线/面要素粗过滤）
func (r *ROI) IntersectsBBox(bbox [4]float64) bool {
	return bbox[0] <= r.BBox[2] && bbox[2] >= r.BBox[0] &&
		bbox[1] <= r.BBox[3] && bbox[3] >= r.BBox[1]
}

// extractRings 从 GeoJSON geometry 提取外环（WGS84）
func extractRings(geomType string, coords any) ([]Ring, error) {
	switch geomType {
	case "Polygon":
		ring, err := toRing(coords)
		if err != nil {
			return nil, err
		}
		return []Ring{ring}, nil
	case "MultiPolygon":
		arr, ok := coords.([]any)
		if !ok {
			return nil, fmt.Errorf("MultiPolygon coordinates 非数组")
		}
		var rings []Ring
		for _, c := range arr {
			ring, err := toRing(c)
			if err != nil {
				return nil, err
			}
			// 仅取每个多边形的外环（第 0 环）
			rings = append(rings, ring)
		}
		return rings, nil
	}
	return nil, fmt.Errorf("不支持的几何类型: %s（仅支持 Polygon/MultiPolygon）", geomType)
}

// toRing 解析 GeoJSON Polygon coordinates 的外环 → Ring
func toRing(coords any) (Ring, error) {
	arr, ok := coords.([]any)
	if !ok || len(arr) == 0 {
		return nil, fmt.Errorf("Polygon coordinates 非数组或为空")
	}
	return toPoints(arr[0])
}

// toPoints 解析坐标点数组 [[lng,lat],...] → Ring
func toPoints(raw any) (Ring, error) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("坐标环非数组")
	}
	ring := make(Ring, 0, len(arr))
	for _, pt := range arr {
		pair, ok := pt.([]any)
		if !ok || len(pair) < 2 {
			return nil, fmt.Errorf("坐标点异常: %v", pt)
		}
		lng, ok1 := pair[0].(float64)
		lat, ok2 := pair[1].(float64)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("坐标点非数字: %v", pt)
		}
		ring = append(ring, LonLat{lng, lat})
	}
	return ring, nil
}

// transformRing WGS84 环 → GCJ02 环
func transformRing(ring Ring) Ring {
	out := make(Ring, len(ring))
	for i, pt := range ring {
		lng, lat := gcj02.WGS84ToGCJ02(pt[0], pt[1])
		out[i] = LonLat{lng, lat}
	}
	return out
}

// ringBBox 环包围盒
func ringBBox(ring Ring) [4]float64 {
	bbox := [4]float64{180, 90, -180, -90}
	for _, pt := range ring {
		bbox[0] = min(bbox[0], pt[0])
		bbox[1] = min(bbox[1], pt[1])
		bbox[2] = max(bbox[2], pt[0])
		bbox[3] = max(bbox[3], pt[1])
	}
	return bbox
}

// mergeBBox 扩大包围盒以包含另一个
func mergeBBox(a, b [4]float64) [4]float64 {
	return [4]float64{min(a[0], b[0]), min(a[1], b[1]), max(a[2], b[2]), max(a[3], b[3])}
}

// pointInRing 射线法判断点是否在环内（不含边界）
func pointInRing(lng, lat float64, ring Ring) bool {
	inside := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		yi, yj := ring[i][1], ring[j][1]
		xi, xj := ring[i][0], ring[j][0]
		if (yi > lat) != (yj > lat) && lng < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}
