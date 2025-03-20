package model

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Archive struct {
	//默认字段
	Id           int64          `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	ParentId     int64          `json:"parent_id" gorm:"column:parent_id;type:bigint(20) not null;default:0;index"`
	CreatedTime  int64          `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time;index:idx_category_created_time,priority:2;index:idx_module_created_time,priority:2"`
	UpdatedTime  int64          `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	Title        string         `json:"title" gorm:"column:title;type:varchar(190) not null;default:'';index"`
	SeoTitle     string         `json:"seo_title" gorm:"column:seo_title;type:varchar(250) not null;default:''"`
	UrlToken     string         `json:"url_token" gorm:"column:url_token;type:varchar(190) not null;default:'';index"`
	Keywords     string         `json:"keywords" gorm:"column:keywords;type:varchar(250) not null;default:''"`
	Description  string         `json:"description" gorm:"column:description;type:varchar(1000) not null;default:''"`
	ModuleId     uint           `json:"module_id" gorm:"column:module_id;type:int(10) unsigned not null;default:1;index:idx_module_created_time,priority:1"`
	CategoryId   uint           `json:"category_id" gorm:"column:category_id;type:int(10) unsigned not null;default:0;index:idx_category_created_time,priority:1"`
	Views        uint           `json:"views" gorm:"column:views;type:int(10) unsigned not null;default:0;index:idx_views"`
	CommentCount uint           `json:"comment_count" gorm:"column:comment_count;type:int(10) unsigned not null;default:0"`
	Images       pq.StringArray `json:"images" gorm:"column:images;type:text default null"`
	Template     string         `json:"template" gorm:"column:template;type:varchar(250) not null;default:''"`
	CanonicalUrl string         `json:"canonical_url" gorm:"column:canonical_url;type:varchar(250) not null;default:''"` // 规范链接
	FixedLink    string         `json:"fixed_link" gorm:"column:fixed_link;type:varchar(190) default null;index"`        // 固化的链接
	UserId       uint           `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Price        int64          `json:"price" gorm:"column:price;type:bigint(20) not null;default:0"`
	Stock        int64          `json:"stock" gorm:"column:stock;type:bigint(20) not null;default:9999999"`
	ReadLevel    int            `json:"read_level" gorm:"column:read_level;type:int(10) not null;default:0"`             // 阅读关联 group level
	Password     string         `json:"password" gorm:"column:password;type:varchar(32) not null;default:''"`            // 明文密码，需要使用密码查看文档的时候填写
	Sort         uint           `json:"sort" gorm:"column:sort;type:int(10) unsigned not null;default:0;index:idx_sort"` // 数值越大，越靠前
	//采集专用
	HasPseudo   int    `json:"has_pseudo" gorm:"column:has_pseudo;type:tinyint(1) not null;default:0"`
	KeywordId   uint   `json:"keyword_id" gorm:"column:keyword_id;type:bigint(20) not null;default:0"`
	OriginUrl   string `json:"origin_url" gorm:"column:origin_url;type:varchar(190) not null;default:'';index:idx_origin_url"`
	OriginTitle string `json:"origin_title" gorm:"column:origin_title;type:varchar(190) not null;default:'';index:idx_origin_title"`
	OriginId    int    `json:"origin_id" gorm:"column:origin_id;type:tinyint(1) unsigned not null;default:0"` // 来源 0， 1 采集，2 AI生成
	// 其他内容
	Category       *Category               `json:"category" gorm:"-"`
	ModuleName     string                  `json:"module_name" gorm:"-"`
	ArchiveData    *ArchiveData            `json:"data" gorm:"-"`
	Logo           string                  `json:"logo" gorm:"-"`
	Thumb          string                  `json:"thumb" gorm:"-"`
	Extra          map[string]*CustomField `json:"extra" gorm:"-"`
	Link           string                  `json:"link" gorm:"-"`
	Tags           []string                `json:"tags,omitempty" gorm:"-"`
	HasOrdered     bool                    `json:"has_ordered" gorm:"-"` // 是否订购了
	FavorablePrice int64                   `json:"favorable_price" gorm:"-"`
	HasPassword    bool                    `json:"has_password" gorm:"-"`             // 需要密码的时候，这个字段为true
	PasswordValid  bool                    `json:"password_valid,omitempty" gorm:"-"` // 验证密码正确
	CategoryTitles []string                `json:"category_titles" gorm:"-"`
	CategoryIds    []uint                  `json:"category_ids" gorm:"-"`
	Flag           string                  `json:"flag" gorm:"-"` // 同 flags，只是这是用,分割的
	Type           string                  `json:"type" gorm:"-"` // 类型，default 是archive，其他值：category，tag
	Relations      []*Archive              `json:"relations" gorm:"-"`
}

type ArchiveData struct {
	Id      int64  `json:"id" gorm:"column:id;type:bigint(20) unsigned not null AUTO_INCREMENT;primaryKey"`
	Content string `json:"content" gorm:"column:content;type:longtext default null"`
}

// ArchiveDraft
// 表结构和 archives一致，但是存放的是草稿，已删除的文章，待发布的文章，用于发布时，将数据复制到 archives 表
// 所有文章的ID都从ArchiveDraft中生成
type ArchiveDraft struct {
	Archive
	Status uint `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
}

func (a *ArchiveDraft) BeforeCreate(tx *gorm.DB) (err error) {
	if a.Id == 0 {
		a.Id = GetNextArchiveId(tx, false)
	}
	return
}

type NextArchiveId struct {
	Id        int64 `json:"-"`
	QueryTime int64 `json:"-"`
}

var nextArchiveIdStore = sync.Mutex{}

// GetNextArchiveId
// ArchiveId 同时检查 archives表和archive_drafts表
// 每次获取，自动加 1
func GetNextArchiveId(tx *gorm.DB, forceUpdate bool) int64 {
	plugin := tx.Config.Plugins["nextArchiveId"]
	pluginNext, _ := plugin.(*NextArchiveIdPlugin)
	nextArchiveIdStore.Lock()
	defer nextArchiveIdStore.Unlock()
	// 仅缓存60秒
	nextArchiveId, nextTime := pluginNext.GetId()
	if nextTime+60 > time.Now().Unix() {
		nextArchiveId += 1
		if forceUpdate {
			nextTime = time.Now().Unix()
		}
		_ = pluginNext.SetId(nextArchiveId, nextTime)

		return nextArchiveId
	}
	// 从数据库读取
	var lastId int64
	tx.Model(Archive{}).Order("id desc").Limit(1).Pluck("id", &lastId)
	var lastIdTmp int64
	tx.Model(ArchiveDraft{}).Order("id desc").Limit(1).Pluck("id", &lastIdTmp)
	if lastId < lastIdTmp {
		lastId = lastIdTmp
	}
	// 下一个ID
	nextArchiveId = lastId + 1
	_ = pluginNext.SetId(nextArchiveId, time.Now().Unix()+60)

	return nextArchiveId
}

func (a *Archive) AddViews(db *gorm.DB) error {
	db.Model(&Archive{}).Where("`id` = ?", a.Id).UpdateColumn("views", gorm.Expr("`views` + 1"))
	return nil
}

func (a *Archive) GetThumb(storageUrl, defaultThumb string) string {
	//取第一张
	if len(a.Images) > 0 {
		for i := range a.Images {
			if !strings.HasPrefix(a.Images[i], "http") && !strings.HasPrefix(a.Images[i], "//") {
				a.Images[i] = storageUrl + "/" + strings.TrimPrefix(a.Images[i], "/")
			}
		}
		a.Logo = a.Images[0]
		if strings.HasPrefix(a.Logo, storageUrl) {
			paths, fileName := filepath.Split(a.Logo)
			a.Thumb = paths + "thumb_" + fileName
		} else {
			a.Thumb = a.Logo
		}
		if strings.HasSuffix(a.Logo, ".svg") {
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

type NextArchiveIdPlugin struct {
	Id        int64 `json:"-"`
	QueryTime int64 `json:"-"`
}

func (n *NextArchiveIdPlugin) Name() string {
	return "nextArchiveId"
}

func (n *NextArchiveIdPlugin) Initialize(*gorm.DB) error {
	return nil
}

func (n *NextArchiveIdPlugin) SetId(id int64, nextTime int64) (err error) {
	n.Id = id
	n.QueryTime = nextTime

	return nil
}

func (n *NextArchiveIdPlugin) GetId() (int64, int64) {
	return n.Id, n.QueryTime
}
