package model

// SpiderInclude 网站收录情况
type SpiderInclude struct {
	Id          uint  `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:int(11);index;autoCreateTime"`
	BaiduCount  int   `json:"baidu_count" gorm:"column:baidu_count;type:int(10);default:0"`   //百度收录情况
	SogouCount  int   `json:"sogou_count" gorm:"column:sogou_count;type:int(10);default:0"`   //搜狗收录情况
	SoCount     int   `json:"so_count" gorm:"column:so_count;type:int(10);default:0"`         //360收录情况
	BingCount   int   `json:"bing_count" gorm:"column:bing_count;type:int(10);default:0"`     //必应收录情况
	GoogleCount int   `json:"google_count" gorm:"column:google_count;type:int(10);default:0"` //谷歌收录情况
}
