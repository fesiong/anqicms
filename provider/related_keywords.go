package provider

import (
	"log"
	"slices"
	"sort"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/fulltext"
	"kandaoni.com/anqicms/request"
)

func (w *Website) getRelatedKeywordArchives(req *request.ApiArchiveListRequest, categoryID uint, keywords string) []*model.Archive {
	result := make([]*model.Archive, 0)
	if keywords == "" || req.Limit <= 0 {
		return result
	}
	filters := func(tx *gorm.DB) *gorm.DB {
		if w.Content.MultiCategory == 1 && (categoryID > 0 || len(req.ExcludeCategoryIds) > 0) {
			tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id").Group("archives.id")
		}
		if categoryID > 0 {
			if w.Content.MultiCategory == 1 {
				tx = tx.Where("archive_categories.category_id = ?", categoryID)
			} else {
				tx = tx.Where("`category_id` = ?", categoryID)
			}
		}
		if req.ModuleId > 0 {
			tx = tx.Where("`module_id` = ?", req.ModuleId)
		}
		if len(req.ExcludeCategoryIds) > 0 {
			if w.Content.MultiCategory == 1 {
				tx = tx.Where("archive_categories.category_id NOT IN (?)", req.ExcludeCategoryIds)
			} else {
				tx = tx.Where("`category_id` NOT IN (?)", req.ExcludeCategoryIds)
			}
		}
		return tx.Where("archives.`id` != ?", req.Id)
	}
	cfg := w.PluginFulltext
	useIndex := cfg != nil && cfg.Open && req.ModuleId > 0
	if useIndex {
		useIndex = slices.Contains(cfg.Modules, uint(req.ModuleId))
	}
	if !useIndex {
		pattern := "%" + keywords + "%"
		if w.GetExplainCount("SELECT id FROM archives") > 100000 {
			pattern = keywords + "%"
		}
		archives, _, err := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
			return filters(tx).Where("`keywords` like ?", pattern)
		}, "archives.id DESC", 0, req.Limit, req.Offset)
		if err != nil {
			log.Printf("related keywords query failed: %v", err)
		}
		return archives
	}
	// Apply category/self filters before the requested offset and limit. Fetch
	// more index pages when those filters discard candidates; never use LIKE.
	offset := max(req.Offset, 0)
	batchSize := max(req.Limit, 100)
	for page := 1; ; page++ {
		docs, _, err := w.Search(keywords, uint(req.ModuleId), page, batchSize)
		if err != nil {
			log.Printf("related fulltext search failed (module_id=%d): %v", req.ModuleId, err)
			return []*model.Archive{}
		}
		if len(docs) == 0 {
			break
		}
		ids := make([]int64, 0, len(docs))
		ranks := make(map[int64]int, len(docs))
		for _, doc := range docs {
			if doc.Type == fulltext.ArchiveType && doc.Id != req.Id {
				if _, exists := ranks[doc.Id]; !exists {
					ranks[doc.Id] = len(ids)
					ids = append(ids, doc.Id)
				}
			}
		}
		if len(ids) > 0 {
			archives, _, queryErr := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				return filters(tx).Where("archives.`id` IN (?)", ids)
			}, "archives.id DESC", 0, len(ids))
			if queryErr != nil {
				log.Printf("related fulltext documents query failed: %v", queryErr)
				return []*model.Archive{}
			}
			sort.SliceStable(archives, func(i, j int) bool { return ranks[archives[i].Id] < ranks[archives[j].Id] })
			result = append(result, archives...)
		}
		if len(result) >= offset+req.Limit || len(docs) < batchSize {
			break
		}
	}
	if offset >= len(result) {
		return []*model.Archive{}
	}
	return result[offset:min(offset+req.Limit, len(result))]
}
