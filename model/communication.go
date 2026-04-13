package model

import (
	"path/filepath"
	"strings"

	"github.com/lib/pq"
	"kandaoni.com/anqicms/config"
)

type CommunicationOrder struct {
	Id             int64          `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	OrderId        string         `json:"order_id" gorm:"column:order_id;type:varchar(36) not null;index"` // 对应订单ID
	CreatedTime    int64          `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime    int64          `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	ArchiveId      int64          `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0;index"`
	UserId         int64          `json:"user_id" gorm:"column:user_id;type:bigint(20) not null;default:0;index"`
	Phone          string         `json:"phone" gorm:"column:phone;type:varchar(100) not null;default:''"` // 电话
	Images         pq.StringArray `json:"images" gorm:"column:images;type:text default null"`
	SketchImages   pq.StringArray `json:"sketch_images" gorm:"column:sketch_images;type:text default null"`
	FinishedImages pq.StringArray `json:"finished_images" gorm:"column:finished_images;type:text default null"`
	Title          string         `json:"title" gorm:"column:title;type:varchar(255) not null;default:''"`           // 标题，对应 budget
	Content        string         `json:"content" gorm:"column:content;type:text default null"`                      // 用户需求描述
	EventDate      string         `json:"event_date" gorm:"column:event_date;type:varchar(255) not null;default:''"` // 对应活动时间
	Status         int8           `json:"status" gorm:"column:status;type:tinyint(4) not null;default:0"`            // 状态，进度，0=待处理， 1=沟通中， 2=已报价待客户确认， 3=接受报价， -1=取消， 4=拒绝报价， 4=完成
	AdminId        int64          `json:"admin_id" gorm:"column:admin_id;type:bigint(20) not null;default:0"`        // 处理人id
	QuotePrice     int64          `json:"quote_price" gorm:"column:quote_price;type:bigint(20) not null;default:0"`
	QuoteContent   string         `json:"quote_content" gorm:"column:quote_content;type:text default null"`           // 报价详情
	OrderExtra     ExtraData      `json:"order_extra,omitempty" gorm:"column:order_extra;type:longtext default null"` // 订单自定义字段

	OrderFields []*config.CustomField   `json:"order_fields" gorm:"-"`
	User        *User                   `json:"user" gorm:"-"`
	Messages    []*CommunicationMessage `json:"messages,omitempty" gorm:"-"`
	QuoteExpire int64                   `json:"quote_expire" gorm:"-"`

	Logo  string `json:"logo" gorm:"-"`
	Thumb string `json:"thumb" gorm:"-"`
}

func (a *CommunicationOrder) GetThumb(storageUrl, defaultThumb string) string {
	//取第一张
	if len(a.Images) > 0 {
		for i := range a.Images {
			if !strings.HasPrefix(a.Images[i], "http") && !strings.HasPrefix(a.Images[i], "//") {
				a.Images[i] = storageUrl + "/" + strings.TrimPrefix(a.Images[i], "/")
			}
		}
		a.Logo = a.Images[0]
		if strings.HasPrefix(a.Logo, storageUrl) && !strings.HasSuffix(a.Logo, ".svg") {
			paths, fileName := filepath.Split(a.Logo)
			a.Thumb = paths + "thumb_" + fileName
		} else {
			a.Thumb = a.Logo
		}
	} else if defaultThumb != "" {
		a.Logo = defaultThumb
		if !strings.HasPrefix(a.Logo, "http") && !strings.HasPrefix(a.Logo, "//") {
			a.Logo = storageUrl + "/" + strings.TrimPrefix(a.Logo, "/")
		}
		a.Thumb = a.Logo
	}

	return a.Thumb
}

type CommunicationMessage struct {
	Id          int64  `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	CommOrderId int64  `json:"comm_order_id" gorm:"column:comm_order_id;type:bigint(20) not null"` // 沟通订单ID
	SenderId    int64  `json:"sender_id" gorm:"column:sender_id;type:bigint(20) not null"`         // 发送者ID, 0表示系统消息
	SenderType  string `json:"sender_type" gorm:"column:sender_type;type:varchar(20) not null"`    // user or admin or system
	SenderName  string `json:"sender_name" gorm:"column:sender_name;type:varchar(100) not null"`
	MessageType string `json:"message_type" gorm:"column:message_type;type:varchar(20) not null"` // text | image | quote | video | file | audio | location | contact | sticker | unknown
	Content     string `json:"content" gorm:"column:content;type:text"`
}
