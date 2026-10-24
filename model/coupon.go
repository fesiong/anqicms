package model

import (
	"math/rand"
	"time"

	"kandaoni.com/anqicms/config"
)

type Coupon struct {
	Id           int64  `json:"id" gorm:"column:id;type:int(11) not null;primary_key;AUTO_INCREMENT"`
	Money        int64  `json:"money" gorm:"column:money;type:int(11)"`
	CreatedTime  int64  `json:"created_time" gorm:"column:created_time;type:int(11)"`
	UpdatedTime  int64  `json:"updated_time" gorm:"column:updated_time;type:int(11)"`
	StartTime    int64  `json:"start_time" gorm:"column:start_time;type:int(11)"`
	EndTime      int64  `json:"end_time" gorm:"column:end_time;type:int(11)"`
	Title        string `json:"title" gorm:"column:title;type:varchar(100)"`
	Description  string `json:"description" gorm:"column:description;type:text"`
	Expire       int64  `json:"expire" gorm:"column:expire;type:int(11)"` // 有效时间 day
	Status       int    `json:"status" gorm:"column:status;type:tinyint(2)"`
	Type         int    `json:"type" gorm:"column:type;type:tinyint(2)"`
	MaxMoney     int64  `json:"max_money" gorm:"column:max_money;type:int(11)"`
	MoneyType    int    `json:"money_type" gorm:"column:money_type;type:tinyint(4)"`
	NeedMoney    int64  `json:"need_money" gorm:"column:need_money;type:int(11)"`
	NeedIntegral int64  `json:"need_integral" gorm:"column:need_integral;type:int(11)"`
	RangeType    int    `json:"range_type" gorm:"column:range_type;type:tinyint(1)"` // 适用范围类型 0 是全部 1是可用 2是不可用

	Archives []*Archive `json:"archives" gorm:"-"`
}

type CouponArchive struct {
	Id        int64 `json:"id" gorm:"column:id;type:int(20) not null;primary_key;AUTO_INCREMENT"`
	CouponId  int64 `json:"coupon_id" gorm:"column:coupon_id;type:int(11);index"`
	ArchiveId int64 `json:"archive_id" gorm:"column:archive_id;type:int(11);index"`
}

type CouponCode struct {
	Id          int64  `json:"id" gorm:"column:id;type:int(20) not null;primary_key;AUTO_INCREMENT"`
	CouponId    int64  `json:"coupon_id" gorm:"column:coupon_id;type:int(11);index"`
	Code        string `json:"code" gorm:"column:code;type:varchar(255);index"`
	StartTime   int64  `json:"start_time" gorm:"column:start_time;type:int(11);index"`
	EndTime     int64  `json:"end_time" gorm:"column:end_time;type:int(11);index"`
	UserId      int64  `json:"user_id" gorm:"column:user_id;type:int(11);index"`
	GetTime     int64  `json:"get_time" gorm:"column:get_time;type:int(11)"`
	UseTime     int64  `json:"use_time" gorm:"column:use_time;type:int(11)"`
	Status      int    `json:"status" gorm:"column:status;type:tinyint(2)"`
	OrderId     string `json:"order_id" gorm:"column:order_id;type:varchar(32)"`
	Money       int64  `json:"money" gorm:"column:money;type:int(11)"`
	MaxMoney    int64  `json:"max_money" gorm:"column:max_money;type:int(11)"`      // 同 coupon
	MoneyType   int    `json:"money_type" gorm:"column:money_type;type:tinyint(4)"` // 同 coupon
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11)"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:int(11)"`

	UserName    string  `json:"user_name" gorm:"-"`
	Title       string  `json:"title" gorm:"-"`
	Description string  `json:"description" gorm:"-"`
	Coupon      *Coupon `json:"coupon" gorm:"-"`
	Type        int     `json:"type" gorm:"-"` // 优惠券类型 1 = 注册送优惠券,2 = 限量优惠券,3 = 购买成功赠送,4 = 满减促销优惠券,5 = 自由规则,6 = 积分兑换券,7 = 活动奖品(小游戏奖品)
}

func (c *Coupon) GetMoney() int64 {
	money := c.Money
	if c.MoneyType == config.CouponMoneyTypeRandom && c.MaxMoney > c.Money {
		// 随机金额, 去掉分
		rd := rand.New(rand.NewSource(time.Now().UnixNano()))
		money = (rd.Int63n(c.MaxMoney-c.Money) + c.Money) / 10 * 10
	}

	return money
}

// GetMoney 获取优惠券的实际金额
func (c *CouponCode) GetMoney(amount int64) int64 {
	money := c.Money
	if c.Coupon == nil {
		return money
	}
	if c.Coupon.NeedMoney > 0 && amount < c.Coupon.NeedMoney {
		return 0
	}
	if c.MoneyType == config.CouponMoneyTypeDiscount {
		// 折扣金额
		money = amount * c.Money / 100
	}
	if c.Coupon.MaxMoney > 0 && money > c.Coupon.MaxMoney {
		money = c.Coupon.MaxMoney
	}

	return money
}
