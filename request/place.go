package request

import (
	"github.com/lib/pq"
	"kandaoni.com/anqicms/model"
)

type PlaceRequest struct {
	Id          uint            `json:"id"`              // 城市 ID
	Title       string          `json:"title"`           // 城市名称
	SeoTitle    string          `json:"seo_title"`       // SEO 标题（默认城市名称）
	Keywords    string          `json:"keywords"`        // 关键词
	UrlToken    string          `json:"url_token"`       // URL 别名
	Description string          `json:"description"`     // 描述
	Content     string          `json:"content"`         // 内容
	ParentId    uint            `json:"parent_id"`       // 父级 ID
	Sort        uint            `json:"sort"`            // 排序，数值越小越靠前
	Template    string          `json:"template"`        // 自定义模板
	IsInherit   uint            `json:"is_inherit"`      // 下级是否继承模板
	Images      pq.StringArray  `json:"images"`          // 城市图片列表
	Logo        string          `json:"logo"`            // 图标
	Extra       model.ExtraData `json:"extra,omitempty"` // 城市自定义字段
	Latitude    float64         `json:"latitude"`        // 纬度
	Longitude   float64         `json:"longitude"`       // 经度
	Timezone    string          `json:"timezone"`        // 时区
	Status      uint            `json:"status"`          // 启用状态：0=停用，1=启用
}

type PlaceDeleteRequest struct {
	Id uint `json:"id"` // 城市 ID
}
