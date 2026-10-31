package request

import (
	"github.com/lib/pq"
	"kandaoni.com/anqicms/model"
)

// PlaceRequest 城市站保存请求。指针字段语义见 request/archive.go 顶部注释：
// nil = 没提这个字段（保持原值），非 nil = 明确写入（含 0 / 空串）。
//
// 城市站是「改一个字段清空一整条记录」最严重的端点：Title/UrlToken 之外
// 全是逐字段无条件赋值，而 UrlToken 在未传时还会用空 Title 重新生成拼音，
// 于是「把城市停用」会顺带把它的 URL 改掉，旧链接直接 404。
//
// Extra 保持值类型（map），nil 天然表示「没提」——不需要指针。
type PlaceRequest struct {
	Id          uint            `json:"id"`              // 城市 ID
	Title       *string         `json:"title"`           // 城市名称
	SeoTitle    *string         `json:"seo_title"`       // SEO 标题（默认城市名称）
	Keywords    *string         `json:"keywords"`        // 关键词
	UrlToken    *string         `json:"url_token"`       // URL 别名；不传=保持原值，传空串=按标题重新生成
	Description *string         `json:"description"`     // 描述
	Content     *string         `json:"content"`         // 内容
	ParentId    *uint           `json:"parent_id"`       // 父级 ID
	Sort        *uint           `json:"sort"`            // 排序，数值越小越靠前
	Template    *string         `json:"template"`        // 自定义模板
	IsInherit   *uint           `json:"is_inherit"`      // 下级是否继承模板
	Images      *pq.StringArray `json:"images"`          // 城市图片列表
	Logo        *string         `json:"logo"`            // 图标
	Extra       model.ExtraData `json:"extra,omitempty"` // 城市自定义字段
	Latitude    *float64        `json:"latitude"`        // 纬度
	Longitude   *float64        `json:"longitude"`       // 经度
	Timezone    *string         `json:"timezone"`        // 时区
	Status      *uint           `json:"status"`          // 启用状态：0=停用，1=启用

	UpdateAll bool `json:"update_all" ast:"-"`
	// Partial 走 PATCH 语义：只覆盖显式传入的字段，未传的一律保持原值。
	// 与 UpdateAll 互为反向开关，且优先——控制器用 req.UpdateAll = !req.Partial 归一。
	Partial bool `json:"partial" ast:"-"`
}

type PlaceDeleteRequest struct {
	Id uint `json:"id"` // 城市 ID
}
