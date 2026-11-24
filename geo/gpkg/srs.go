package gpkg

import "fmt"

// KnownSRS 预置常用空间参考（对照 go-spatial/geom 的 KnownSRS）。
// 用户可通过 EnsureSRS 一键写入，无需手写 WKT。
var KnownSRS = map[int32]SpatialRefSys{
	4326: {
		SRSID:      4326,
		AuthName:   "EPSG",
		AuthSRID:   4326,
		Definition: `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]]`,
		SRSText:    `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]]`,
	},
	3857: {
		SRSID:      3857,
		AuthName:   "EPSG",
		AuthSRID:   3857,
		Definition: `PROJCS["WGS 84 / Pseudo-Mercator",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Mercator_1SP"],PARAMETER["central_meridian",0],PARAMETER["scale_factor",1],PARAMETER["false_easting",0],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["X",EAST],AXIS["Y",NORTH],AUTHORITY["EPSG","3857"]]`,
		SRSText:    `PROJCS["WGS 84 / Pseudo-Mercator",GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]],PROJECTION["Mercator_1SP"],PARAMETER["central_meridian",0],PARAMETER["scale_factor",1],PARAMETER["false_easting",0],PARAMETER["false_northing",0],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["X",EAST],AXIS["Y",NORTH],AUTHORITY["EPSG","3857"]]`,
	},
	4490: {
		SRSID:      4490,
		AuthName:   "EPSG",
		AuthSRID:   4490,
		Definition: `GEOGCS["China Geodetic Coordinate System 2000",DATUM["China_2000",SPHEROID["CGCS2000",6378137,298.257222101,AUTHORITY["EPSG","1024"]],AUTHORITY["EPSG","1043"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4490"]]`,
		SRSText:    `GEOGCS["China Geodetic Coordinate System 2000",DATUM["China_2000",SPHEROID["CGCS2000",6378137,298.257222101,AUTHORITY["EPSG","1024"]],AUTHORITY["EPSG","1043"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4490"]]`,
	},
	// 4979: WGS 84 三维地理坐标（经纬度 + 椭球高）
	4979: {
		SRSID:      4979,
		AuthName:   "EPSG",
		AuthSRID:   4979,
		Definition: `GEOGCS["WGS 84",DATUM["World Geodetic System 1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AXIS["geodetic latitude (Lat)",NORTH],AXIS["geodetic longitude (Lon)",EAST],AXIS["ellipsoidal height (h)",UP],AUTHORITY["EPSG","4979"]]`,
		SRSText:    `GEOGCS["WGS 84",DATUM["World Geodetic System 1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AXIS["geodetic latitude (Lat)",NORTH],AXIS["geodetic longitude (Lon)",EAST],AXIS["ellipsoidal height (h)",UP],AUTHORITY["EPSG","4979"]]`,
	},
	// 5737: 1985 国家高程基准（黄海高程，垂直坐标系）
	5737: {
		SRSID:      5737,
		AuthName:   "EPSG",
		AuthSRID:   5737,
		Definition: `VERT_CS["Yellow Sea 1985 height",VERT_DATUM["Yellow Sea 1985",2005,AUTHORITY["EPSG","5137"]],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Gravity-related height",UP],AUTHORITY["EPSG","5737"]]`,
		SRSText:    `VERT_CS["Yellow Sea 1985 height",VERT_DATUM["Yellow Sea 1985",2005,AUTHORITY["EPSG","5137"]],UNIT["metre",1,AUTHORITY["EPSG","9001"]],AXIS["Gravity-related height",UP],AUTHORITY["EPSG","5737"]]`,
	},
}

// EnsureSRS 将预置的空间参考写入 gpkg_spatial_ref_sys，幂等。
// 未知 SRS 返回错误；已存在时保持原记录不变。
func (w *Writer) EnsureSRS(ids ...int32) error {
	for _, id := range ids {
		srs, ok := KnownSRS[id]
		if !ok {
			return fmt.Errorf("未知预置空间参考: %d", id)
		}
		if err := w.AddSRS(srs); err != nil {
			return err
		}
	}
	return nil
}
