package vector

import (
	"fmt"
	"hash/fnv"
	"strconv"
)

// PointDeduper 点要素坐标去重（8 位小数精度，消除跨瓦片重复点）
type PointDeduper struct {
	seen map[string]struct{}
}

func NewPointDeduper() *PointDeduper {
	return &PointDeduper{seen: make(map[string]struct{})}
}

// TryAdd 尝试登记坐标；已存在返回 false
func (d *PointDeduper) TryAdd(lng, lat float64) bool {
	key := strconv.FormatFloat(lng, 'f', 8, 64) + "," + strconv.FormatFloat(lat, 'f', 8, 64)
	if _, ok := d.seen[key]; ok {
		return false
	}
	d.seen[key] = struct{}{}
	return true
}

// GeomDeduper 线/面要素去重：按 style_id+label+sign+坐标序列哈希，
// 消除服务端重复下发的整要素副本（跨瓦片断裂段不去重，它们坐标不同）
type GeomDeduper struct {
	seen map[uint64]struct{}
}

func NewGeomDeduper() *GeomDeduper {
	return &GeomDeduper{seen: make(map[uint64]struct{})}
}

// TryAdd 尝试登记要素；已存在返回 false
func (d *GeomDeduper) TryAdd(styleID int, label, sign any, segs [][]float64) bool {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d|%v|%v|", styleID, label, sign)
	for _, seg := range segs {
		for _, v := range seg {
			h.Write([]byte(strconv.FormatFloat(v, 'f', 8, 64)))
			h.Write([]byte{0})
		}
		h.Write([]byte{255})
	}
	sum := h.Sum64()
	if _, ok := d.seen[sum]; ok {
		return false
	}
	d.seen[sum] = struct{}{}
	return true
}
