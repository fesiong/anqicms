package model

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/config"
)

type OrderReview struct {
	Id           uint           `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime  int64          `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime  int64          `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	ArchiveId    int64          `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0;index:idx_archive_id"`
	ArchiveSkuId int64          `json:"archive_sku_id" gorm:"column:archive_sku_id;type:bigint(20) not null;default:0;index"`
	UserId       uint           `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index:idx_user_id"`
	OrderId      string         `json:"order_id" gorm:"column:order_id;type:varchar(36) not null;index"`
	Ip           string         `json:"ip" gorm:"column:ip;type:varchar(32) not null;default:''"`
	VoteCount    int            `json:"vote_count" gorm:"column:vote_count;type:int(10) not null;default:0;index:idx_vote_count"`
	Score        int            `json:"score" gorm:"column:score;type:int(10) not null;default:0"`
	Content      string         `json:"content" gorm:"column:content;type:longtext default null"`
	ParentId     uint           `json:"parent_id" gorm:"column:parent_id;type:int(10) unsigned not null;default:0;index:idx_parent_id"`
	Images       pq.StringArray `json:"images" gorm:"column:images;type:text default null"`
	Status       uint           `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"` // 状态，0 未处理，1 正常，-1 删除
	UserName     string         `json:"user_name" gorm:"column:user_name;type:varchar(64) not null;default:''"`
	Email        string         `json:"email" gorm:"column:email;type:varchar(100) not null;default:'';index:idx_email"`

	AvatarURL    string `json:"avatar_url" gorm:"-"`
	ArchiveTitle string `json:"archive_title" gorm:"-"`
	SkuTitle     string `json:"sku_title" gorm:"-"`
	Active       bool   `json:"active" gorm:"-"`
}

type OrderReviewPraise struct {
	Id          int64 `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	ArchiveId   int64 `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0;index"`
	ReviewId    int64 `json:"review_id" gorm:"column:review_id;type:bigint(20) not null;default:0;index:idx_review_id;index:idx_user_review_id,priority:2"`
	UserId      uint  `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index:idx_user_review_id,priority:1"` // 只有登陆用户可以点赞
	Rate        int   `json:"rate" gorm:"column:rate;type:tinyint(1) not null;default:1"`                                                 // 1=赞，-1=踩，目前只支持 赞
}

func (o *OrderReview) AfterCreate(tx *gorm.DB) (err error) {
	// 更新数量	// 更新评分
	var sumScore float32
	var total int64
	tx.Model(&OrderReview{}).Where("`archive_id` = ?", o.ArchiveId).Count(&total)
	tx.Model(&OrderReview{}).Where("`archive_id` = ?", o.ArchiveId).Select("COALESCE(SUM(score), 0) as sum_score").Scan(&sumScore)

	var averageScore float32
	if total > 0 {
		averageScore = sumScore / float32(total)
	}

	// review_count / average_score 已迁到 archive_products；UpdateColumns 会跳过
	// hook（gorm 硬写 SkipHooks），所以这里必须显式写侧表。
	// 用 upsert 而不是 Update：只填了价格、没配任何商品属性的商品在侧表里没有行，
	// 直接 Update 会命中 0 行，评分永远不显示。
	tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "archive_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"review_count":  total,
			"average_score": averageScore,
		}),
	}).Create(&ArchiveProduct{
		ArchiveId:    o.ArchiveId,
		ReviewCount:  total,
		AverageScore: averageScore,
		ProductType:  config.ProductTypeNormal,
	})
	// // 更新点赞数量
	// var praiseTotal int64
	// tx.Model(&OrderReviewPraise{}).Where("`review_id` = ?", o.Id).Count(&praiseTotal)
	// tx.Model(&OrderReview{}).Where("`id` = ?", o.Id).UpdateColumn("vote_count", praiseTotal)
	return
}

func (cp *OrderReviewPraise) AfterCreate(tx *gorm.DB) (err error) {
	// 更新数量
	var total int64
	tx.Model(&OrderReviewPraise{}).Where("`review_id` = ?", cp.ReviewId).Count(&total)
	tx.Model(&OrderReview{}).Where("`id` = ?", cp.ReviewId).UpdateColumn("vote_count", total)

	return
}
