package provider

import (
	"errors"
	"github.com/huichen/wukong/engine"
	"github.com/huichen/wukong/types"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"log"
	"strconv"
)

var (
	searcher       *engine.Engine
	fulltextStatus int // 0 未启用，1初始化中，2 初始化完成
)

const (
	InitSqlLimit = 100
)

type TinyArchive struct {
	Id       uint   `json:"id"`
	ModuleId uint   `json:"module_id"`
	Title    string `json:"title"`
	Keywords string `json:"keywords"`
	Content  string `json:"content"`
}

func GetFullTextStatus() int {
	return fulltextStatus
}

func InitFulltext() {
	if !config.JsonData.PluginFulltext.Open || searcher != nil {
		return
	}
	fulltextStatus = 1
	searcher = new(engine.Engine)
	// 初始化
	searcher.Init(types.EngineInitOptions{SegmenterDictionaries: config.ExecPath + "dictionary.txt"})

	var archiveCount int
	// 导入索引：仅导入标题/关键词和内容
	var lastId uint = 0
	for {
		var archives = make([]*TinyArchive, 0, InitSqlLimit)
		dao.DB.Table("`archives` as a").Joins("left join `archive_data` as d on a.id=d.id").Select("a.id,a.title,a.keywords,a.module_id,d.content").Where("a.`id` > ? and `status` = ?", lastId, config.ContentStatusOK).Order("a.id asc").Limit(InitSqlLimit).Scan(&archives)
		if len(archives) == 0 {
			break
		}
		archiveCount += len(archives)
		lastId = archives[len(archives)-1].Id
		for _, v := range archives {
			AddFulltextIndex(v)
		}
	}
	// 等待索引刷新完毕
	searcher.FlushIndex()
	fulltextStatus = 2

	log.Print("索引总数", archiveCount)

}

func CloseFulltext() {
	fulltextStatus = 0
	if searcher == nil {
		return
	}
	searcher.Close()
	searcher = nil
}

func FlushIndex() {
	if searcher == nil {
		return
	}
	searcher.FlushIndex()
}

func AddFulltextIndex(doc *TinyArchive) {
	if searcher == nil {
		return
	}
	content := doc.Title
	if doc.Keywords != "" {
		content += " " + doc.Keywords
	}
	if doc.Content != "" {
		// 内容搜索的时候，需要去除html标签
		content += " " + library.StripTags(doc.Content)
	}
	searcher.IndexDocument(uint64(doc.Id), types.DocumentIndexData{
		Content: content,
		Labels:  []string{strconv.Itoa(int(doc.ModuleId))},
	}, false)
}

func RemoveFulltextIndex(id uint) {
	if searcher == nil {
		return
	}
	searcher.RemoveDocument(uint64(id), false)
}

func Search(key string, moduleId uint, page, pageSize int) (ids []uint, total int64, err error) {
	if searcher == nil {
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
	output := searcher.Search(types.SearchRequest{
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

func OutputSearchIds(output types.SearchResponse) (ids []uint) {
	for _, doc := range output.Docs {
		ids = append(ids, uint(doc.DocId))
	}
	return
}
