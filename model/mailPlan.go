package model

import "github.com/lib/pq"

type MailPlan struct {
	Id           int64          `json:"id" gorm:"column:id;type:bigint(20) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime  int64          `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime  int64          `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	Identifier   string         `json:"identifier" gorm:"column:identifier;type:varchar(100);index:idx_identifier;comment:邮件计划标识;index"` // 用来取消计划
	Recipients   pq.StringArray `json:"recipients" gorm:"column:recipients;type:text;comment:收件人列表"`
	Subject      string         `json:"subject" gorm:"column:subject;type:varchar(255) not null;default:'';comment:邮件主题"`
	Content      string         `json:"content" gorm:"column:content;type:text;comment:邮件内容"`
	AttachPath   string         `json:"attach_path" gorm:"column:attach_path;type:varchar(255);comment:附件路径"`
	SendTime     int64          `json:"send_time" gorm:"column:send_time;type:int(11);not null;default:0;comment:计划发送时间戳;index:idx_status_send_time,priority:2"`
	ExecutedTime int64          `json:"executed_time" gorm:"column:executed_time;type:int(11);default:0;comment:实际执行时间"`
	Status       int            `json:"status" gorm:"column:status;type:tinyint(4);not null;default:0;comment:状态0待发送1发送中2已发送-1已取消3发送错误;index:idx_status_send_time,priority:1"`
	SendResult   string         `json:"send_result" gorm:"column:send_result;type:varchar(1000);comment:发送结果统计"`
}
