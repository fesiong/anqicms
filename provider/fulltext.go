package provider

import (
	"errors"
	"github.com/huichen/wukong/engine"
	"github.com/huichen/wukong/types"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
	"strconv"
)

const (
	InitSqlLimit = 100
)

type TinyArchive struct {
	Id          uint   `json:"id"`
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
	var lastId uint = 0
	for {
		var archives = make([]*TinyArchive, 0, InitSqlLimit)
		if w.PluginFulltext.UseContent {
			w.DB.Table("`archives` as a").Joins("left join `archive_data` as d on a.id=d.id").Select("a.id,a.title,a.keywords,a.description,a.module_id,d.content").Where("a.`id` > ? and a.`module_id` IN(?) and a.`status` = ?", lastId, w.PluginFulltext.Modules, config.ContentStatusOK).Order("a.id asc").Limit(InitSqlLimit).Scan(&archives)
		} else {
			w.DB.Table("`archives` as a").Select("a.id,a.title,a.keywords,a.description,a.module_id").Where("a.`id` > ? and a.`module_id` IN(?) and a.`status` = ?", lastId, w.PluginFulltext.Modules, config.ContentStatusOK).Order("a.id asc").Limit(InitSqlLimit).Scan(&archives)
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
	w.searcher.IndexDocument(uint64(doc.Id), types.DocumentIndexData{
		Content: content,
		Labels:  []string{strconv.Itoa(int(doc.ModuleId))},
	}, false)
}

func (w *Website) RemoveFulltextIndex(id uint) {
	if w.searcher == nil {
		return
	}
	w.searcher.RemoveDocument(uint64(id), false)
}

func (w *Website) Search(key string, moduleId uint, page, pageSize int) (ids []uint, total int64, err error) {
	if w.searcher == nil {
		err = errors.New("未初始化")
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
		ids = append(ids, uint(doc.DocId))
	}
	total = int64(output.NumDocs)

	return
}

func (w *Website) OutputSearchIds(output types.SearchResponse) (ids []uint) {
	for _, doc := range output.Docs {
		ids = append(ids, uint(doc.DocId))
	}
	return
}
