package provider

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/fulltext"
	"log"
)

const (
	InitSqlLimit = 100
)

type FulltextStatus struct {
	Status  int    `json:"status"` // 0 未启用，1初始化中，2 初始化完成，-1 错误
	Total   int64  `json:"total"`
	Current int64  `json:"current"`
	Msg     string `json:"msg"`
}

func (w *Website) GetFullTextStatus() *FulltextStatus {
	return w.fulltextStatus
}

func (w *Website) InitFulltext(focus bool) {
	if w.PluginFulltext == nil || !w.PluginFulltext.Open || len(w.PluginFulltext.Modules) == 0 || w.searcher != nil {
		return
	}
	w.fulltextStatus = &FulltextStatus{
		Status: 1,
		Msg:    "Initializing",
	}
	var err error

	log.Println("fulltext init")
	// 使用数据库名称作为indexName
	indexName := w.Mysql.Database
	if w.PluginFulltext.Engine == "zincsearch" {
		// zinc
		w.searcher, err = fulltext.NewZincSearchService(w.PluginFulltext, indexName)
	} else if w.PluginFulltext.Engine == "meilisearch" {
		// meili
		w.searcher, err = fulltext.NewMeiliSearchService(w.PluginFulltext, indexName)
	} else if w.PluginFulltext.Engine == "elasticsearch" {
		w.searcher, err = fulltext.NewElasticSearchService(w.PluginFulltext, indexName)
	} else {
		// 默认使用 wukong
		w.searcher, err = fulltext.NewWukongService(w.PluginFulltext, indexName)
		w.PluginFulltext.Initialed = false
	}
	//由于指针的原因，因此需要将它赋值给原始对象
	w2 := websites.MustGet(w.Id)
	w2.PluginFulltext = w.PluginFulltext
	w2.searcher = w.searcher
	log.Println("fulltext init", err)
	if err != nil {
		w.fulltextStatus.Status = -1
		w.fulltextStatus.Msg = "error:" + err.Error()
		log.Print("init fulltext error", err)
		return
	}
	if focus || w.PluginFulltext.Initialed == false {
		w.fulltextStatus.Total = w.GetExplainCount(w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
			return tx.Model(&model.Archive{}).Where("`module_id` IN(?)", w.PluginFulltext.Modules).Find(&[]*model.Archive{})
		}))
		w.fulltextStatus.Current = 0
		var archiveCount int64
		// 导入索引：仅导入标题/关键词/描述和内容
		var lastId int64 = 0
		for {
			var archives = make([]fulltext.TinyArchive, 0, InitSqlLimit)
			if w.PluginFulltext.UseContent {
				w.DB.Table("`archives` as archives").Joins("left join `archive_data` as d on archives.id=d.id").Select("archives.id,archives.title,archives.keywords,archives.description,archives.module_id,d.content,'archive' as `type`").Where("archives.`id` > ? and archives.`module_id` IN(?)", lastId, w.PluginFulltext.Modules).Order("archives.id asc").Limit(InitSqlLimit).Scan(&archives)
			} else {
				w.DB.Table("`archives` as archives").Select("archives.id,archives.title,archives.keywords,archives.description,archives.module_id,'archive' as `type`").Where("archives.`id` > ? and archives.`module_id` IN(?)", lastId, w.PluginFulltext.Modules).Order("archives.id asc").Limit(InitSqlLimit).Scan(&archives)
			}
			if len(archives) == 0 {
				break
			}
			archiveCount += int64(len(archives))
			lastId = archives[len(archives)-1].Id
			w.fulltextStatus.Msg = fmt.Sprintf("Archive id: %d", archives[0].Id)
			w.BulkFulltextIndex(archives)
			w.fulltextStatus.Current = archiveCount
		}
		// 导入分类
		if w.PluginFulltext.UseCategory {
			w.fulltextStatus.Total += w.GetExplainCount(w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.Category{}).Where("`module_id` IN(?)", w.PluginFulltext.Modules).Find(&[]*model.Category{})
			}))
			lastId = 0
			for {
				var categories = make([]fulltext.TinyArchive, 0, InitSqlLimit)
				if w.PluginFulltext.UseContent {
					w.DB.Model(&model.Category{}).Select("id,title,keywords,description,module_id,content,'category' as `type`").Where("`id` > ? and `module_id` IN(?)", lastId, w.PluginFulltext.Modules).Order("id asc").Limit(InitSqlLimit).Scan(&categories)
				} else {
					w.DB.Model(&model.Category{}).Select("id,title,keywords,description,module_id,'category' as `type`").Where("`id` > ? and `module_id` IN(?)", lastId, w.PluginFulltext.Modules).Order("id asc").Limit(InitSqlLimit).Scan(&categories)
				}
				if len(categories) == 0 {
					break
				}
				archiveCount += int64(len(categories))
				lastId = categories[len(categories)-1].Id
				w.fulltextStatus.Msg = fmt.Sprintf("Category id: %d", categories[0].Id)
				w.BulkFulltextIndex(categories)
				w.fulltextStatus.Current = archiveCount
			}
		}
		// 导入标签
		if w.PluginFulltext.UseTag {
			w.fulltextStatus.Total += w.GetExplainCount(w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.Tag{}).Find(&[]*model.Tag{})
			}))
			lastId = 0
			for {
				var tags = make([]fulltext.TinyArchive, 0, InitSqlLimit)
				w.DB.Model(&model.Tag{}).Select("id,title,keywords,description,'tag' as `type`").Where("`id` > ?", lastId).Order("id asc").Limit(InitSqlLimit).Scan(&tags)
				if len(tags) == 0 {
					break
				}
				archiveCount += int64(len(tags))
				lastId = tags[len(tags)-1].Id
				w.fulltextStatus.Msg = fmt.Sprintf("Tag id: %d", tags[0].Id)
				w.BulkFulltextIndex(tags)
				w.fulltextStatus.Current = archiveCount
			}
		}
		// 等待索引刷新完毕
		w.searcher.Flush()
		w.fulltextStatus.Status = 2
		w.fulltextStatus.Msg = "Initialized"

		w.PluginFulltext.Initialed = true
		_ = w.SaveSettingValue(FulltextSettingKey, w.PluginFulltext)

		log.Print("索引总数", archiveCount)
	} else {
		w.fulltextStatus.Status = 2
	}
}

func (w *Website) CloseFulltext() {
	// 防止出错
	w2 := GetWebsite(w.Id)
	w2.fulltextStatus = &FulltextStatus{}
	if w2.searcher == nil {
		return
	}
	w2.searcher.Close()
	w2.searcher = nil
}

func (w *Website) FlushIndex() {
	if w.searcher != nil {
		w.searcher.Flush()
	}
}

func (w *Website) AddFulltextIndex(doc fulltext.TinyArchive) {
	if w.searcher == nil {
		return
	}
	if w.PluginFulltext.UseContent && doc.Content != "" {
		// 内容搜索的时候，需要去除html标签
		doc.Content = library.StripTags(doc.Content)
	}
	_ = w.searcher.Create(doc)
}

func (w *Website) UpdateFulltextIndex(doc fulltext.TinyArchive) {
	if w.searcher == nil {
		return
	}
	if w.PluginFulltext.UseContent && doc.Content != "" {
		// 内容搜索的时候，需要去除html标签
		doc.Content = library.StripTags(doc.Content)
	}
	_ = w.searcher.Update(doc)
}

func (w *Website) BulkFulltextIndex(docs []fulltext.TinyArchive) {
	if w.searcher == nil {
		return
	}
	for i, doc := range docs {
		docs[i].Content = library.StripTags(doc.Content)
	}
	_ = w.searcher.Bulk(docs)
}

func (w *Website) RemoveFulltextIndex(doc fulltext.TinyArchive) {
	if w.searcher == nil {
		return
	}
	_ = w.searcher.Delete(doc)
}

func (w *Website) Search(key string, moduleId uint, page, pageSize int) (docs []fulltext.TinyArchive, total int64, err error) {
	if w.searcher == nil {
		err = errors.New(w.Tr("Uninitialized"))
		return
	}

	docs, total, err = w.searcher.Search(key, moduleId, page, pageSize)
	if err != nil {
		return
	}

	return
}
