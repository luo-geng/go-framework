// 测试向量来自 gcoord（npm）实测输出，用于校验公式移植的正确性。
package gcj02

import (
	"math"
	"testing"
)

// 1e-6 度 ≈ 0.1 米，近似逆变换的固有精度即在此量级
const tolerance = 1e-6

type transformCase struct {
	lng, lat   float64
	wantGCJLng float64
	wantGCJLat float64
}

var cases = []transformCase{
	{116.404, 39.915, 116.41024450, 39.91640428},   // 北京
	{113.63, 34.75, 113.63605129, 34.74882624},     // 郑州
	{121.4737, 31.2304, 121.47822306, 31.22845774}, // 上海
	{104.0665, 30.5723, 104.06900491, 30.56984554}, // 成都
	{87.6168, 43.8256, 87.61964995, 43.82680539},   // 乌鲁木齐（非加密区边界附近仍加密）
}

func TestWGS84ToGCJ02(t *testing.T) {
	for _, c := range cases {
		lng, lat := WGS84ToGCJ02(c.lng, c.lat)
		if math.Abs(lng-c.wantGCJLng) > tolerance || math.Abs(lat-c.wantGCJLat) > tolerance {
			t.Errorf("WGS84ToGCJ02(%v, %v) = (%v, %v)，want (%v, %v)",
				c.lng, c.lat, lng, lat, c.wantGCJLng, c.wantGCJLat)
		}
	}
}

func TestGCJ02ToWGS84_RoundTrip(t *testing.T) {
	for _, c := range cases {
		gcjLng, gcjLat := WGS84ToGCJ02(c.lng, c.lat)
		lng, lat := GCJ02ToWGS84(gcjLng, gcjLat)
		if math.Abs(lng-c.lng) > tolerance || math.Abs(lat-c.lat) > tolerance {
			t.Errorf("往返变换 (%v, %v) → GCJ02 → (%v, %v)，偏差超 %v",
				c.lng, c.lat, lng, lat, tolerance)
		}
	}
}

func TestOutOfChina(t *testing.T) {
	// 境外坐标不做偏移
	for _, c := range [][2]float64{{140.0, 35.0}, {60.0, 40.0}, {100.0, 60.0}} {
		lng, lat := WGS84ToGCJ02(c[0], c[1])
		if lng != c[0] || lat != c[1] {
			t.Errorf("境外坐标 (%v, %v) 不应偏移，得到 (%v, %v)", c[0], c[1], lng, lat)
		}
	}
}
