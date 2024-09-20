package provider

import (
	"errors"
	"github.com/huichen/wukong/engine"
	"github.com/huichen/wukong/types"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"log"
	"strconv"
)

const (
	InitSqlLimit    = 100
	CategoryDivider = 1000000000
	TagDivider      = 2000000000
	TagDividerEnd   = 3000000000
)

type TinyArchive struct {
	Id          uint64 `json:"id"` // 小于1000000000=文档ID，1000000000开头是分类ID，2000000000开头标签ID
	ModuleId    uint   `json:"module_id"`
	Title       string `json:"title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

func (w *Website) GetFullTextStatus() int {
	return w.fulltextStatus
}

func (w *Website) InitFulltext() {
	if !w.PluginFulltext.Open || len(w.PluginFulltext.Modules) == 0 || w.searcher != nil {
		return
	}
	w.fulltextStatus = 1
	w.searcher = new(engine.Engine)
	// 初始化
	w.searcher.Init(types.EngineInitOptions{SegmenterDictionaries: config.ExecPath + "dictionary.txt"})

	var archiveCount int
	// 导入索引：仅导入标题/关键词/描述和内容
	var lastId uint64 = 0
	for {
		var archives = make([]*TinyArchive, 0, InitSqlLimit)
		if w.PluginFulltext.UseContent {
			w.DB.Table("`archives` as archives").Joins("left join `archive_data` as d on archives.id=d.id").Select("archives.id,archives.title,archives.keywords,archives.description,archives.module_id,d.content").Where("archives.`id` > ? and archives.`module_id` IN(?)", lastId, w.PluginFulltext.Modules).Order("archives.id asc").Limit(InitSqlLimit).Scan(&archives)
		} else {
			w.DB.Table("`archives` as archives").Select("archives.id,archives.title,archives.keywords,archives.description,archives.module_id").Where("archives.`id` > ? and archives.`module_id` IN(?)", lastId, w.PluginFulltext.Modules).Order("archives.id asc").Limit(InitSqlLimit).Scan(&archives)
		}
		if len(archives) == 0 {
			break
		}
		archiveCount += len(archives)
		lastId = archives[len(archives)-1].Id
		for _, v := range archives {
			w.AddFulltextIndex(v)
		}
	}
	// 导入分类
	if w.PluginFulltext.UseCategory {
		var categories = make([]*TinyArchive, 0, InitSqlLimit)
		if w.PluginFulltext.UseContent {
			w.DB.Model(&model.Category{}).Select("id,title,keywords,description,module_id,content").Order("id asc").Scan(&categories)
		} else {
			w.DB.Model(&model.Category{}).Select("id,title,keywords,description,module_id").Order("id asc").Scan(&categories)
		}
		archiveCount += len(categories)
		for _, v := range categories {
			// 分类ID需 1000000000 开头
			v.Id = CategoryDivider + v.Id
			w.AddFulltextIndex(v)
		}
	}
	// 导入标签
	if w.PluginFulltext.UseTag {
		var tags = make([]*TinyArchive, 0, InitSqlLimit)
		w.DB.Model(&model.Tag{}).Select("id,title,keywords,description").Order("id asc").Scan(&tags)
		archiveCount += len(tags)
		for _, v := range tags {
			// 标签ID需 2000000000 开头
			v.Id = TagDivider + v.Id
			w.AddFulltextIndex(v)
		}

	}
	// 等待索引刷新完毕
	w.searcher.FlushIndex()
	w.fulltextStatus = 2

	log.Print("索引总数", archiveCount)

}

func (w *Website) CloseFulltext() {
	w.fulltextStatus = 0
	if w.searcher == nil {
		return
	}
	w.searcher.Close()
	w.searcher = nil
}

func (w *Website) FlushIndex() {
	if w.searcher == nil {
		return
	}
	w.searcher.FlushIndex()
}

func (w *Website) AddFulltextIndex(doc *TinyArchive) {
	if w.searcher == nil {
		return
	}
	content := doc.Title
	if doc.Keywords != "" {
		content += " " + doc.Keywords
	}
	if doc.Description != "" {
		// 内容搜索的时候，需要去除html标签
		content += " " + library.StripTags(doc.Description)
	}
	if w.PluginFulltext.UseContent && doc.Content != "" {
		// 内容搜索的时候，需要去除html标签
		content += " " + library.StripTags(doc.Content)
	}
	w.searcher.IndexDocument(doc.Id, types.DocumentIndexData{
		Content: content,
		Labels:  []string{strconv.Itoa(int(doc.ModuleId))},
	}, false)
}

func (w *Website) RemoveFulltextIndex(id uint64) {
	if w.searcher == nil {
		return
	}
	w.searcher.RemoveDocument(id, false)
}

func (w *Website) Search(key string, moduleId uint, page, pageSize int) (ids []uint64, total int64, err error) {
	if w.searcher == nil {
		err = errors.New(w.Tr("Uninitialized"))
		return
	}
	if page < 1 {
		page = 1
	}

	var labels []string
	if moduleId > 0 {
		labels = append(labels, strconv.Itoa(int(moduleId)))
	}
	output := w.searcher.Search(types.SearchRequest{
		Text:   key,
		Labels: labels,
		RankOptions: &types.RankOptions{
			OutputOffset: pageSize * (page - 1),
			MaxOutputs:   pageSize,
		}})
	for _, doc := range output.Docs {
		ids = append(ids, doc.DocId)
	}
	total = int64(output.NumDocs)

	return
}

func (w *Website) OutputSearchIds(output types.SearchResponse) (ids []uint64) {
	for _, doc := range output.Docs {
		ids = append(ids, doc.DocId)
	}
	return
}
