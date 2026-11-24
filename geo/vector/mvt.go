// 将解码后的腾讯矢量瓦片要素编码为标准 Mapbox Vector Tile（MVT）。
//
// 坐标换算：瓦片内像素(0-256) × 16 → MVT tile 坐标(0-4096)。
// 约定：
//   - layer 按要素大类（group）组织
//   - 面要素环不闭合（腾讯原始数据如此），编码前强制闭合并按 MVT 规范修正绕向（外环顺时针）
//   - 无法区分洞与独立面，所有环视为独立面的外环（MultiPolygon）
package vector

import (
	"fmt"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
)

// pixelToTileUnits 像素 → MVT tile 坐标系数（256 → 4096）
const pixelToTileUnits = 16

// EncodeTile 将单瓦片解码结果编码为 MVT protobuf 字节。
// feats 需已附带分类信息（class/group），见 BuildLayers。
func EncodeTile(layers mvt.Layers) ([]byte, error) {
	data, err := mvt.Marshal(layers)
	if err != nil {
		return nil, fmt.Errorf("MVT 编码失败: %w", err)
	}
	return data, nil
}

// Layerize 按大类分组要素并转换为 MVT layer 结构。
//
// feats 为瓦片解码结果；classify 返回 (stylename, group)。
// 每个要素的属性：name/class/type/style_id。
func Layerize(feats []Feature, zoom int, c *Classifier) mvt.Layers {
	byGroup := map[string]*mvt.Layer{}

	for _, f := range feats {
		gt, stylename, ok := c.Classify(f.StyleID, zoom)
		if !ok {
			stylename = fmt.Sprintf("unknown_%d", f.StyleID)
		}

		group := string(GroupOf(stylename))
		if group == "" {
			// 未归组时按几何类型兜底：点/标注 → poi，其余 → other
			if gt == Point || gt == Label {
				group = string(GroupPOI)
			} else {
				group = string(GroupOther)
			}
		}

		layer, exists := byGroup[group]
		if !exists {
			layer = mvt.NewLayer(group, geojson.NewFeatureCollection())
			layer.Version = 2
			layer.Extent = 4096
			byGroup[group] = layer
		}

		geom, err := toOrbGeometry(gt, f.Segments)
		if err != nil || geom == nil {
			continue // 空几何或类型不支持，跳过
		}

		feat := geojson.NewFeature(geom)
		feat.Properties = geojson.Properties{
			"class":    stylename,
			"type":     string(gt),
			"style_id": f.StyleID,
			"name":     labelString(f.Label),
		}
		layer.Features = append(layer.Features, feat)
	}

	layers := make(mvt.Layers, 0, len(byGroup))
	for _, l := range byGroup {
		layers = append(layers, l)
	}
	return layers
}

// toOrbGeometry 像素坐标段 → orb 几何（tile 坐标系）
func toOrbGeometry(gt GeomType, segs []Segment) (orb.Geometry, error) {
	switch gt {
	case Point, Label:
		if len(segs) == 0 || len(segs[0]) < 2 {
			return nil, nil
		}
		return orb.Point{float64(segs[0][0]) * pixelToTileUnits, float64(segs[0][1]) * pixelToTileUnits}, nil

	case Line, Arrow:
		if len(segs) == 1 {
			return lineString(segs[0]), nil
		}
		mls := make(orb.MultiLineString, len(segs))
		for i, s := range segs {
			mls[i] = lineString(s)
		}
		return mls, nil

	case Region:
		// 每段视为一个独立面的外环
		if len(segs) == 1 {
			return orb.Polygon{orb.Ring(closeRingOriented(lineString(segs[0]), true))}, nil
		}
		mp := make(orb.MultiPolygon, len(segs))
		for i, s := range segs {
			mp[i] = orb.Polygon{orb.Ring(closeRingOriented(lineString(s), true))}
		}
		return mp, nil
	}
	return nil, fmt.Errorf("不支持的几何类型: %s", gt)
}

// lineString 像素段 → tile 坐标 LineString
func lineString(seg Segment) orb.LineString {
	ls := make(orb.LineString, 0, len(seg)/2)
	for i := 0; i+1 < len(seg); i += 2 {
		ls = append(ls, orb.Point{float64(seg[i]) * pixelToTileUnits, float64(seg[i+1]) * pixelToTileUnits})
	}
	return ls
}

// closeRingOriented 闭合环并修正绕向：exterior=true 要求顺时针（屏幕坐标 y 向下，面积>0 即顺时针）
func closeRingOriented(ring orb.LineString, exterior bool) orb.LineString {
	if len(ring) == 0 {
		return ring
	}
	// 闭合：首点 != 末点时追加首点
	if ring[0] != ring[len(ring)-1] {
		ring = append(ring, ring[0])
	}
	if signedArea(ring) > 0 != exterior {
		reverse(ring)
	}
	return ring
}

// signedArea 环形有向面积（tile 坐标，y 向下；正值 = 屏幕顺时针）
func signedArea(ring orb.LineString) float64 {
	var sum float64
	for i := 0; i+1 < len(ring); i++ {
		sum += ring[i][0]*ring[i+1][1] - ring[i+1][0]*ring[i][1]
	}
	return sum / 2
}

// reverse 原地反转环（保持首尾闭合点位置不变）
func reverse(ring orb.LineString) {
	for i, j := 1, len(ring)-2; i < j; i, j = i+1, j-1 {
		ring[i], ring[j] = ring[j], ring[i]
	}
}

// labelString 提取要素名称（非字符串 label 视为无名）
func labelString(label any) string {
	if s, ok := label.(string); ok {
		return s
	}
	return ""
}
