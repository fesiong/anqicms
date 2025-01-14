package model

import (
	"fmt"
	"gorm.io/gorm"
	"time"
)

type Order struct {
	Model
	OrderId           string `json:"order_id" gorm:"column:order_id;type:varchar(36) not null;unique"`
	PaymentId         string `json:"payment_id" gorm:"column:payment_id;type:varchar(36) not null;index"`
	UserId            uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	AddressId         uint   `json:"address_id" gorm:"column:address_id;type:int(10) unsigned not null;default:0"`
	Remark            string `json:"remark" gorm:"column:remark;type:varchar(250) not null;default:''"`
	Type              string `json:"type" gorm:"column:type;type:varchar(20) not null;default:'goods'"` // 订单类型，支持 goods|vip,默认goods
	Status            int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
	RefundStatus      int    `json:"refund_status" gorm:"column:refund_status;type:tinyint(1) not null;default:0"`
	OriginAmount      int64  `json:"origin_amount" gorm:"column:origin_amount;type:bigint(20) not null;default:0;comment:订单商品总价"`
	Amount            int64  `json:"amount" gorm:"column:amount;type:bigint(20) not null;default:0;comment:订单计算运费、优惠后的总价"`
	PaidTime          int64  `json:"paid_time" gorm:"column:paid_time;type:int(10) not null;default:0;comment:支付时间"`
	EndTime           int64  `json:"end_time" gorm:"column:end_time;type:int(10) not null;default:0;comment:在确认支付之前，为订单关闭时间，在支付完成后，为自动确认收货时间"`
	DeliverTime       int64  `json:"deliver_time" gorm:"column:deliver_time;type:int(10) not null;default:0;comment:发货时间"`
	FinishedTime      int64  `json:"finished_time" gorm:"column:finished_time;type:int(10) not null;default:0;comment:订单完成时间"`
	DiscountAmount    int64  `json:"discount_amount" gorm:"column:discount_amount;type:bigint(20) not null;default:0;comment:优惠金额"` // 可能一个订单支持多个优惠
	CouponCodeId      string `json:"-" gorm:"column:coupon_code_id;type:varchar(36) not null;default:''"`
	SellerId          uint   `json:"seller_id" gorm:"column:seller_id;type:int(10) unsigned not null;default:0;index"` // 卖家
	SellerAmount      int64  `json:"seller_amount" gorm:"column:seller_amount;type:bigint(20) not null;default:0;comment:卖家可得金额"`
	ShareUserId       uint   `json:"share_user_id" gorm:"column:share_user_id;type:int(10) unsigned not null;default:0;index"`                // 分享者
	ShareParentUserId uint   `json:"share_parent_user_id" gorm:"column:share_parent_user_id;type:int(10) unsigned not null;default:0;index"`  // 分享者的上级
	ShareAmount       int64  `json:"share_amount" gorm:"column:share_amount;type:bigint(20) not null;default:0;comment:分销可得金额"`               // 分销可得金额
	ShareParentAmount int64  `json:"share_parent_amount" gorm:"column:share_parent_amount;type:bigint(20) not null;default:0;comment:奖励金，上级"` // 分销可得金额
	ExpressCompany    string `json:"express_company" gorm:"column:express_company;type:varchar(100) not null;default:''"`                     // 快递公司
	TrackingNumber    string `json:"tracking_number" gorm:"column:tracking_number;type:varchar(100) not null;default:''"`                     // 快递单号

	OrderAddress *OrderAddress  `json:"order_address,omitempty" gorm:"-"`
	User         *User          `json:"user" gorm:"-"`
	ShareUser    *User          `json:"share_user,omitempty" gorm:"-"`
	ParentUser   *User          `json:"parent_user,omitempty" gorm:"-"`
	IsUpdated    int            `json:"is_updated" gorm:"-"`
	Details      []*OrderDetail `json:"details" gorm:"-"`
}

func (o *Order) AfterCreate(tx *gorm.DB) (err error) {
	if o.OrderId == "" {
		tmp := fmt.Sprintf("%04d", o.Id)
		o.OrderId = time.Now().Format("20060102150405") + tmp[len(tmp)-4:]
		err = tx.Model(o).UpdateColumn("order_id", o.OrderId).Error
		if err != nil {
			return err
		}
	}
	return
}

type OrderDetail struct {
	Model
	OrderId      string     `json:"order_id" gorm:"column:order_id;type:varchar(36) not null;default:'';index"`
	UserId       uint       `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	GoodsId      int64      `json:"goods_id" gorm:"column:goods_id;type:bigint(20) not null;index"`
	GoodsItemId  int64      `json:"goods_item_id" gorm:"column:goods_item_id;type:bigint(20) not null;default:0;index"`
	Price        int64      `json:"price" gorm:"column:price;type:bigint(20) not null;default:0"`
	OriginPrice  int64      `json:"origin_price" gorm:"column:origin_price;type:bigint(20) not null;default:0"`
	Amount       int64      `json:"amount" gorm:"column:amount;type:bigint(20) not null;default:0"` // 实际支付的金额，用于退款的时候进行退款操作
	OriginAmount int64      `json:"origin_amount" gorm:"column:origin_amount;type:bigint(20) not null;default:0"`
	Quantity     int        `json:"quantity" gorm:"column:quantity;type:int(10) not null;default:0"`
	Status       int        `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
	Goods        *Archive   `json:"goods" gorm:"-"`
	Group        *UserGroup `json:"group" gorm:"-"`
}

type OrderAddress struct {
	Model
	UserId      uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Name        string `json:"name" gorm:"column:name;type:varchar(64) not null;default:''"`
	Phone       string `json:"phone" gorm:"column:phone;type:varchar(20) not null;default:'';index"`
	Province    string `json:"province" gorm:"column:province;type:varchar(100) not null;default:''"`
	City        string `json:"city" gorm:"column:city;type:varchar(100) not null;default:''"`
	Country     string `json:"country" gorm:"column:country;type:varchar(100) not null;default:''"`
	AddressInfo string `json:"address_info" gorm:"column:address_info;type:varchar(255) not null;default:''"`
	Postcode    string `json:"postcode" gorm:"column:postcode;type:varchar(36) not null;default:''"`
	Status      int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
}

// OrderRefund 退款记录
type OrderRefund struct {
	Model
	RefundId   string `json:"refund_id" gorm:"column:refund_id;type:varchar(36) not null;unique"`
	OrderId    string `json:"order_id" gorm:"column:order_id;type:varchar(36) not null"`
	DetailId   uint   `json:"detail_id" gorm:"column:detail_id;type:int(10) not null;default:0"`
	UserId     uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Amount     int64  `json:"amount" gorm:"column:amount;type:bigint(20) not null;default:0;index;comment:'退款金额'"`
	RefundTime int64  `json:"refund_time" gorm:"column:refund_time;type:int(10) not null;default:0;comment:支付时间"` //该子订单支付时间
	Status     int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
	ErrorTimes int    `json:"error_times" gorm:"column:error_times;type:int(10) not null;default:0"` // 执行错误次数
	LastTime   int64  `json:"last_time" gorm:"column:last_time;type:int(10) not null;default:0"`     // 上次执行时间
	Remark     string `json:"remark" gorm:"column:remark;type:varchar(255) default null"`            //备注
}

func (o *OrderRefund) AfterCreate(tx *gorm.DB) (err error) {
	if o.RefundId == "" {
		tmp := fmt.Sprintf("%04d", o.Id)
		o.RefundId = time.Now().Format("20060102150405") + tmp[len(tmp)-4:]
		err = tx.Model(o).UpdateColumn("refund_id", o.RefundId).Error
		if err != nil {
			return err
		}
	}
	return
}

type Payment struct {
	Model
	PaymentId string `json:"payment_id" gorm:"column:payment_id;type:varchar(36) not null;unique"`
	TerraceId string `json:"terrace_id" gorm:"column:terrace_id;type:varchar(64) not null;index"` //交易id，服务商返回
	UserId    uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	OrderId   string `json:"order_id" gorm:"column:order_id;type:varchar(36) not null;index"` //订单id
	Amount    int64  `json:"amount" gorm:"column:amount;type:bigint(20) not null;default:0;comment:'支付总价'"`
	Status    int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`                 //支付状态
	PayWay    string `json:"pay_way" gorm:"column:pay_way;type:varchar(32) not null;default:'';index"`       // 支付方式
	PaidTime  int64  `json:"paid_time" gorm:"column:paid_time;type:int(10) not null;default:0;comment:支付时间"` //该订单支付时间
	Remark    string `json:"remark" gorm:"column:remark;type:varchar(255) default null"`                     //备注
	BuyerId   string `json:"buyer_id" gorm:"column:buyer_id;type:varchar(64) not null;default:''"`           //用户标识
	BuyerInfo string `json:"buyer_info" gorm:"column:buyer_info;type:varchar(255) not null;default:''"`      //买家信息，PayPal有返回
}

func (p *Payment) AfterCreate(tx *gorm.DB) (err error) {
	if p.PaymentId == "" {
		tmp := fmt.Sprintf("%04d", p.Id)
		p.PaymentId = time.Now().Format("20060102150405") + tmp[len(tmp)-4:]
		err = tx.Model(p).UpdateColumn("payment_id", p.PaymentId).Error
		if err != nil {
			return err
		}
	}
	return
}
