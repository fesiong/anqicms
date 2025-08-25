package response

import "kandaoni.com/anqicms/model"

type ModuleCount struct {
	Id    uint   `json:"id"`
	Name  string `json:"name"`
	Total int64  `json:"total"`
}
type ArchiveCount struct {
	Total     int64 `json:"total"`
	LastWeek  int64 `json:"last_week"`
	UnRelease int64 `json:"un_release"`
	Draft     int64 `json:"draft"`
	Today     int64 `json:"today"`
}
type SplitCount struct {
	Total int64 `json:"total"`
	Today int64 `json:"today"`
}
type Statistics struct {
	CacheTime       int64
	ShowGuide       bool                `json:"show_guide"`
	ModuleCounts    []ModuleCount       `json:"archive_counts"`
	ArchiveCount    ArchiveCount        `json:"archive_count"`
	CategoryCount   int64               `json:"category_count"`
	LinkCount       int64               `json:"link_count"`
	GuestbookCount  int64               `json:"guestbook_count"`
	TrafficCount    SplitCount          `json:"traffic_count"`
	SpiderCount     SplitCount          `json:"spider_count"`
	IncludeCount    model.SpiderInclude `json:"include_count"`
	TemplateCount   int64               `json:"template_count"`
	PageCount       int64               `json:"page_count"`
	AttachmentCount int64               `json:"attachment_count"`
	Exact           bool                `json:"exact"` // 是否取的是真实值
}
