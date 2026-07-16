package model

import (
	"database/sql/driver"
	"encoding/json"
)

// StatisticLog * 每天浏览量
type StatisticLog struct {
	Id          uint        `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64       `json:"created_time" gorm:"column:created_time;type:bigint(20);unique"`
	SpiderCount SpiderCount `json:"spider_count" gorm:"column:spider_count;type:varchar(250) default null"`
	VisitCount  VisitCount  `json:"visit_count" gorm:"column:visit_count;type:varchar(250) default null"`
	ShopCount   ShopCount   `json:"shop_count" gorm:"column:shop_count;type:varchar(1000) default null"`
}

type SearchLog struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	Keyword     string `json:"keyword" gorm:"column:keyword;type:varchar(100) not null;default:''"`
	UserId      int64  `json:"user_id" gorm:"column:user_id;type:int(11)"`
	Ip          string `json:"ip" gorm:"column:ip;type:varchar(32) not null;default:''"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
}

type VisitCount struct {
	IPCount int `json:"ip_count"`
	PVCount int `json:"pv_count"`
}

type ShopCount struct {
	CouponGet   int `json:"coupon_get"`   // 优惠券获取数量
	CouponUse   int `json:"coupon_use"`   // 优惠券使用数量
	OrderCreate int `json:"order_create"` // 下单数量
	FirstOrder  int `json:"first_order"`  // 新客
	Repurchase  int `json:"repurchase"`   // 复购
	PayCreate   int `json:"pay_create"`   // 支付创建数量
	UserCreate  int `json:"user_create"`  // 新用户数量
	UserLogin   int `json:"user_login"`   // 用户登录数量
	ArticleView int `json:"article_view"` // 文章浏览量
	ProductView int `json:"product_view"` // 产品浏览量
	Cart        int `json:"cart"`         // 加购数量
	Checkout    int `json:"checkout"`     // 进入确认地址页面
	Confirm     int `json:"confirm"`      // 进入支付页面
	Payment     int `json:"payment"`      // 付款数量
}

func (v ShopCount) Value() (driver.Value, error) {
	return json.Marshal(v)
}

func (v *ShopCount) Scan(data interface{}) error {
	_ = json.Unmarshal(data.([]byte), &v)
	return nil
}

func (v VisitCount) Value() (driver.Value, error) {
	return json.Marshal(v)
}

func (v *VisitCount) Scan(data interface{}) error {
	_ = json.Unmarshal(data.([]byte), &v)
	return nil
}

type SpiderCount map[string]int

func (s SpiderCount) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *SpiderCount) Scan(data interface{}) error {
	_ = json.Unmarshal(data.([]byte), &s)
	return nil
}
