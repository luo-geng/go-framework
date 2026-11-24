// Package vector 腾讯矢量瓦片（.tmvt）解码、标准 MVT（.mvt）编解码、
// 要素解析管线（POI 输出）与 MapLibre style.json 生成。
//
// 编码管线（逆向自腾讯前端 JS）：
// base64（去 padding、滤非法字符）→ 每2字节一组做3轮自定义异或解扰
// → UTF-8 → 截到最后一个 '}' → JSON；要素坐标为瓦片内差分编码的像素坐标，
// 本包负责还原差分并合并 Multi 分段。
//
// 样式表（styles.json）逆向自腾讯前端，约 4751 条，随包内嵌。
//
// 坐标约定：腾讯数据为 GCJ02；ParseOptions.CRS="wgs84" 时输出转 WGS84。
// 解析标准 .mvt（ParseMVTTile）同样按腾讯数据约定处理（GCJ02）。
package vector
