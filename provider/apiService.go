package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/fulltext"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func (w *Website) ApiGetArchive(req *request.ApiArchiveRequest) (*model.Archive, error) {
	var archive *model.Archive
	var err error
	if req.Id > 0 {
		archive = w.GetArchiveByIdFromCache(req.Id)
	}
	if req.UrlToken != "" {
		// 处理特殊的 prev and next
		if req.UrlToken == "prev" || req.UrlToken == "next" {
			if archive == nil {
				return nil, errors.New("no archive found")
			}
			if req.UrlToken == "prev" {
				archive, err = w.GetPreviousArchive(int64(archive.CategoryId), archive.Id)
				if err != nil {
					return nil, err
				}
			} else {
				archive, err = w.GetNextArchive(int64(archive.CategoryId), archive.Id)
				if err != nil {
					return nil, err
				}
			}
		} else {
			archive, err = w.GetArchiveByUrlToken(req.UrlToken)
		}
	}
	if archive == nil {
		return nil, errors.New("no archive found")
	}

	if err != nil && req.Id > 0 {
		if req.UserId > 0 {
			archiveDraft, err2 := w.GetArchiveDraftById(req.Id)
			if err2 == nil {
				if archiveDraft.UserId != req.UserId {
					return nil, errors.New("no archive found")
				} else {
					archive = &archiveDraft.Archive
					err = nil
				}
			}
		}
	}
	if err != nil {
		return nil, err
	}
	// if read level larger than 0, then need to check permission
	if req.UserId > 0 {
		archive = w.CheckArchiveHasOrder(req.UserId, archive, req.UserGroup)
		if archive.Price > 0 {
			discount := w.GetUserDiscount(req.UserId, req.UserInfo)
			if discount > 0 {
				archive.FavorablePrice = archive.Price * discount / 100
			}
		}
	}

	// if read level larger than 0, then need to check permission
	if archive.ReadLevel > 0 && !archive.HasOrdered {
		archive.ArchiveData = &model.ArchiveData{
			Content: w.TplTr("ThisContentRequiresUserLevel%dOrAboveToRead", archive.ReadLevel),
		}
	} else {
		// 读取data
		archive.ArchiveData, _ = w.GetArchiveDataById(archive.Id)
	}
	if req.UserId > 0 {
		exist := w.CheckFavorites(int64(req.UserId), []int64{archive.Id})
		if len(exist) > 0 {
			archive.IsFavorite = true
		}
	}
	// 读取flag
	archive.Flag = w.GetArchiveFlags(archive.Id)
	// 读取分类
	archive.Category = w.GetCategoryFromCache(archive.CategoryId)
	if archive.Category != nil {
		archive.Category.Link = w.GetUrl("category", archive.Category, 0)
	}
	// 读取 extraDate
	archiveParams := w.GetArchiveExtra(archive.ModuleId, archive.Id, true)
	archive.Extra = make(map[string]model.CustomField, len(archiveParams))
	if len(archiveParams) > 0 {
		for i := range archiveParams {
			param := *archiveParams[i]
			if (param.Value == nil || param.Value == "" || param.Value == 0) &&
				param.Type != config.CustomFieldTypeRadio &&
				param.Type != config.CustomFieldTypeCheckbox &&
				param.Type != config.CustomFieldTypeSelect {
				param.Value = param.Default
			}
			if param.FollowLevel && !archive.HasOrdered {
				continue
			}
			if param.Type == config.CustomFieldTypeEditor && req.Render {
				param.Value = library.MarkdownToHTML(fmt.Sprintf("%v", param.Value), w.System.BaseUrl, w.Content.FilterOutlink)
			} else if param.Type == config.CustomFieldTypeArchive {
				// 列表
				arcIds, ok := param.Value.([]int64)
				if !ok && param.Default != "" {
					value, _ := strconv.ParseInt(fmt.Sprint(param.Default), 10, 64)
					if value > 0 {
						arcIds = append(arcIds, value)
					}
				}
				if len(arcIds) > 0 {
					archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
						return tx.Where("archives.`id` IN (?)", arcIds)
					}, "archives.id ASC", 0, len(arcIds))
					param.Value = archives
				} else {
					param.Value = nil
				}
			} else if param.Type == config.CustomFieldTypeCategory {
				value, ok := param.Value.(int64)
				if !ok && param.Default != "" {
					value, _ = strconv.ParseInt(fmt.Sprint(param.Default), 10, 64)
				}
				if value > 0 {
					param.Value = w.GetCategoryFromCache(uint(value))
				} else {
					param.Value = nil
				}
			}
			archive.Extra[i] = param
		}
	}
	tags := w.GetTagsByItemId(archive.Id)
	archive.Tags = tags
	if len(archive.Password) > 0 {
		// password is not visible for user
		password := req.Password
		if password == archive.Password {
			archive.PasswordValid = true
		}
		archive.Password = ""
		archive.HasPassword = true
		// 带密码的文档，如果密码不正确，则不显示内容
		if archive.PasswordValid == false {
			archive.ArchiveData = nil
		}
	}
	if archive.ArchiveData != nil {
		// convert markdown to html
		if req.Render {
			archive.ArchiveData.Content = library.MarkdownToHTML(archive.ArchiveData.Content, w.System.BaseUrl, w.Content.FilterOutlink)
		}
		re, _ := regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
		archive.ArchiveData.Content = re.ReplaceAllStringFunc(archive.ArchiveData.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 2 {
				return s
			}
			if !strings.HasPrefix(match[1], "http") {
				res := w.System.BaseUrl + match[1]
				s = strings.Replace(s, match[1], res, 1)
			}
			return s
		})
		archive.Content = archive.ArchiveData.Content
	}

	return archive, nil
}

func (w *Website) ApiGetArchives(req *request.ApiArchiveListRequest) ([]*model.Archive, int64) {
	var categoryDetail *model.Category
	var module *model.Module
	if len(req.CategoryIds) > 0 {
		categoryDetail = w.GetCategoryFromCache(uint(req.CategoryIds[0]))
		if categoryDetail != nil {
			req.ModuleId = int64(categoryDetail.ModuleId)
		}
	}
	module = w.GetModuleFromCache(uint(req.ModuleId))
	if req.TagId > 0 {
		req.TagIds = append(req.TagIds, req.TagId)
	}
	var tmpResult = make([]*model.Archive, 0, req.Limit)
	var archives []*model.Archive
	var total int64
	if req.Type == "related" {
		//获取id
		var categoryId = uint(0)
		var keywords string
		if len(req.CategoryIds) > 0 {
			categoryId = uint(req.CategoryIds[0])
		}
		if req.Id > 0 {
			archive := w.GetArchiveByIdFromCache(req.Id)
			if archive != nil {
				categoryId = archive.CategoryId
				keywords = strings.Split(strings.ReplaceAll(archive.Keywords, "，", ","), ",")[0]
				category := w.GetCategoryFromCache(categoryId)
				if category != nil {
					req.ModuleId = int64(category.ModuleId)
				}
			}
		}
		// 允许通过keywords调用
		if len(req.Keywords) > 0 {
			keywords = req.Keywords
		}

		if req.Like == "keywords" {
			archives, _, _ = w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				if w.Content.MultiCategory == 1 && (categoryId > 0 || len(req.ExcludeCategoryIds) > 0) {
					tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id")
				}
				if categoryId > 0 {
					if w.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id = ?", categoryId)
					} else {
						tx = tx.Where("`category_id` = ?", categoryId)
					}
				} else if req.ModuleId > 0 {
					tx = tx.Where("`module_id` = ?", req.ModuleId)
				}
				if len(req.ExcludeCategoryIds) > 0 {
					if w.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id NOT IN (?)", req.ExcludeCategoryIds)
					} else {
						tx = tx.Where("`category_id` NOT IN (?)", req.ExcludeCategoryIds)
					}
				}
				tx = tx.Where("`keywords` like ? AND archives.`id` != ?", "%"+keywords+"%", req.Id)
				return tx
			}, "archives.id DESC", 0, req.Limit, req.Offset)
		} else if req.Like == "relation" {
			if categoryId > 0 || req.ModuleId > 0 || len(req.ExcludeCategoryIds) > 0 {
				archives, total, _ = w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
					tx = tx.Table("`archives` as archives").Group("archives.id").
						Joins("INNER JOIN `archive_relations` as t ON archives.id = t.relation_id AND t.archive_id = ? AND archives.`id` != ?", req.Id, req.Id)
					if w.Content.MultiCategory == 1 && (categoryId > 0 || len(req.ExcludeCategoryIds) > 0) {
						tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id")
					}
					if categoryId > 0 {
						if w.Content.MultiCategory == 1 {
							tx = tx.Where("archive_categories.category_id = ?", categoryId)
						} else {
							tx = tx.Where("archives.`category_id` = ?", categoryId)
						}
					} else if req.ModuleId > 0 {
						tx = tx.Where("archives.`module_id` = ?", req.ModuleId)
					}
					if len(req.ExcludeCategoryIds) > 0 {
						if w.Content.MultiCategory == 1 {
							tx = tx.Where("archive_categories.category_id NOT IN (?)", req.ExcludeCategoryIds)
						} else {
							tx = tx.Where("archives.`category_id` NOT IN (?)", req.ExcludeCategoryIds)
						}
					}

					return tx
				}, "archives.id DESC", 0, req.Limit, req.Offset)
			} else {
				archives = w.GetArchiveRelations(req.Id)
			}
		} else if req.Like == "tag" {
			// 根据tag来调用相关
			var tmpTagIds []uint
			w.DB.WithContext(w.Ctx()).Model(&model.TagData{}).Where("`item_id` = ?", req.Id).Pluck("tag_id", &tmpTagIds)
			if len(tmpTagIds) > 0 {
				archives, total, _ = w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
					tx = tx.Table("`archives` as archives").Group("archives.id").
						Joins("INNER JOIN `tag_data` as t ON archives.id = t.item_id AND t.`tag_id` IN (?) AND archives.`id` != ?", tmpTagIds, req.Id)
					if w.Content.MultiCategory == 1 && (categoryId > 0 || len(req.ExcludeCategoryIds) > 0) {
						tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id")
					}
					if categoryId > 0 {
						if w.Content.MultiCategory == 1 {
							tx = tx.Where("archive_categories.category_id = ?", categoryId)
						} else {
							tx = tx.Where("archives.`category_id` = ?", categoryId)
						}
					} else if req.ModuleId > 0 {
						tx = tx.Where("archives.`module_id` = ?", req.ModuleId)
					}
					if len(req.ExcludeCategoryIds) > 0 {
						if w.Content.MultiCategory == 1 {
							tx = tx.Where("archive_categories.category_id NOT IN (?)", req.ExcludeCategoryIds)
						} else {
							tx = tx.Where("archives.`category_id` NOT IN (?)", req.ExcludeCategoryIds)
						}
					}

					return tx
				}, "archives.id DESC", 0, req.Limit, req.Offset)
			}
		} else if req.Like == "id" {
			halfLimit := int(math.Ceil(float64(req.Limit) / 2))
			archives1, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				if w.Content.MultiCategory == 1 {
					// 多分类支持
					tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id and archive_categories.category_id = ?", categoryId)
				} else {
					tx = tx.Where("`category_id` = ?", categoryId)
				}
				if len(req.ExcludeCategoryIds) > 0 {
					if w.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id NOT IN (?)", req.ExcludeCategoryIds)
					} else {
						tx = tx.Where("`category_id` NOT IN (?)", req.ExcludeCategoryIds)
					}
				}
				tx = tx.Where("archives.`id` > ?", req.Id)
				return tx
			}, "archives.id ASC", 0, req.Limit, req.Offset)
			archives2, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				if w.Content.MultiCategory == 1 {
					// 多分类支持
					tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id and archive_categories.category_id = ?", categoryId)
				} else {
					tx = tx.Where("`category_id` = ?", categoryId)
				}
				if len(req.ExcludeCategoryIds) > 0 {
					if w.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id NOT IN (?)", req.ExcludeCategoryIds)
					} else {
						tx = tx.Where("`category_id` NOT IN (?)", req.ExcludeCategoryIds)
					}
				}
				tx = tx.Where("archives.`id` < ?", req.Id)
				return tx
			}, "archives.id DESC", 0, req.Limit, req.Offset)
			if len(archives1)+len(archives2) > req.Limit {
				if len(archives1) > halfLimit && len(archives2) > halfLimit {
					archives1 = archives1[:halfLimit]
					archives2 = archives2[:halfLimit]
				} else if len(archives1) > len(archives2) {
					archives1 = archives1[:req.Limit-len(archives2)]
				} else if len(archives2) > len(archives1) {
					archives2 = archives2[:req.Limit-len(archives1)]
				}
			}
			archives = append(archives2, archives1...)
			// 如果数量超过，则截取
			if len(archives) > req.Limit {
				archives = archives[:req.Limit]
			}
		} else {
			// 检查是否有相关文档
			archives = w.GetArchiveRelations(req.Id)
			if len(archives) == 0 {
				halfLimit := int(math.Ceil(float64(req.Limit) / 2))
				archives1, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
					if w.Content.MultiCategory == 1 {
						// 多分类支持
						tx = tx.Group("archives.id").Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id and archive_categories.category_id = ?", categoryId)
					} else {
						tx = tx.Where("`category_id` = ?", categoryId)
					}
					if len(req.ExcludeCategoryIds) > 0 {
						if w.Content.MultiCategory == 1 {
							tx = tx.Group("archives.id").Where("archive_categories.category_id NOT IN (?)", req.ExcludeCategoryIds)
						} else {
							tx = tx.Where("`category_id` NOT IN (?)", req.ExcludeCategoryIds)
						}
					}
					tx = tx.Where("archives.`id` > ?", req.Id)
					return tx
				}, "archives.id ASC", 0, req.Limit, req.Offset)
				archives2, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
					if w.Content.MultiCategory == 1 {
						// 多分类支持
						tx = tx.Group("archives.id").Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id and archive_categories.category_id = ?", categoryId)
					} else {
						tx = tx.Where("`category_id` = ?", categoryId)
					}
					if len(req.ExcludeCategoryIds) > 0 {
						if w.Content.MultiCategory == 1 {
							tx = tx.Group("archives.id").Where("archive_categories.category_id NOT IN (?)", req.ExcludeCategoryIds)
						} else {
							tx = tx.Where("`category_id` NOT IN (?)", req.ExcludeCategoryIds)
						}
					}
					tx = tx.Where("archives.`id` < ?", req.Id)
					return tx
				}, "archives.id DESC", 0, req.Limit, req.Offset)
				if len(archives1)+len(archives2) > req.Limit {
					if len(archives1) > halfLimit && len(archives2) > halfLimit {
						archives1 = archives1[:halfLimit]
						archives2 = archives2[:halfLimit]
					} else if len(archives1) > len(archives2) {
						archives1 = archives1[:req.Limit-len(archives2)]
					} else if len(archives2) > len(archives1) {
						archives2 = archives2[:req.Limit-len(archives1)]
					}
				}
				archives = append(archives2, archives1...)
				// 如果数量超过，则截取
				if len(archives) > req.Limit {
					archives = archives[:req.Limit]
				}
			}
		}
	} else {
		var fulltextSearch bool
		var fulltextTotal int64
		var err2 error
		var ids []int64
		var searchCatIds []uint
		var searchTagIds []uint
		if len(req.Q) > 0 {
			var tmpDocs []fulltext.TinyArchive
			tmpDocs, fulltextTotal, err2 = w.Search(req.Q, uint(req.ModuleId), req.Page, req.Limit)
			if err2 == nil {
				fulltextSearch = true
				for _, doc := range tmpDocs {
					if doc.Type == fulltext.ArchiveType {
						ids = append(ids, doc.Id)
					} else if doc.Type == fulltext.CategoryType {
						searchCatIds = append(searchCatIds, uint(doc.Id))
					} else if doc.Type == fulltext.TagType {
						searchTagIds = append(searchTagIds, uint(doc.Id))
					} else {
						// 其他值
					}
				}
				if len(tmpDocs) == 0 || len(ids) == 0 {
					ids = append(ids, 0)
				}
				req.Offset = 0
			}
		}
		if len(searchCatIds) > 0 {
			cats := w.GetCacheCategoriesByIds(searchCatIds)
			// 将cats 按 searchCatIds 顺序排列
			idToIndex := make(map[uint]int)
			// 建立ID到索引的映射关系
			for i, id := range searchCatIds {
				idToIndex[id] = i
			}

			// 按照映射的索引进行排序
			sort.Slice(cats, func(i, j int) bool {
				indexI, existsI := idToIndex[cats[i].Id]
				indexJ, existsJ := idToIndex[cats[j].Id]

				// 如果两个ID都在指定列表中，则按指定顺序排序
				if existsI && existsJ {
					return indexI < indexJ
				}
				// 如果只有i在列表中，则i排在前面
				if existsI && !existsJ {
					return true
				}
				// 如果只有j在列表中，则j排在前面
				if !existsI && existsJ {
					return false
				}
				// 如果都不在列表中，则保持原有顺序
				return i < j
			})
			for _, cat := range cats {
				cat.Link = w.GetUrl("category", cat, 0)
				tmpResult = append(tmpResult, &model.Archive{
					Type:        "category",
					Id:          int64(cat.Id),
					CreatedTime: cat.CreatedTime,
					UpdatedTime: cat.UpdatedTime,
					Title:       cat.Title,
					SeoTitle:    cat.SeoTitle,
					UrlToken:    cat.UrlToken,
					Keywords:    cat.Keywords,
					Description: cat.Description,
					ModuleId:    cat.ModuleId,
					CategoryId:  cat.ParentId,
					Images:      cat.Images,
					Logo:        cat.Logo,
					Link:        cat.Link,
					Thumb:       cat.Thumb,
					Sort:        cat.Sort,
				})
			}
		}
		if len(searchTagIds) > 0 {
			tags := w.GetTagsByIds(searchTagIds)
			// 将tags 按 searchTagIds 顺序排列
			idToIndex := make(map[uint]int)
			// 建立ID到索引的映射关系
			for i, id := range searchTagIds {
				idToIndex[id] = i
			}

			// 按照映射的索引进行排序
			sort.Slice(tags, func(i, j int) bool {
				indexI, existsI := idToIndex[tags[i].Id]
				indexJ, existsJ := idToIndex[tags[j].Id]

				// 如果两个ID都在指定列表中，则按指定顺序排序
				if existsI && existsJ {
					return indexI < indexJ
				}
				// 如果只有i在列表中，则i排在前面
				if existsI && !existsJ {
					return true
				}
				// 如果只有j在列表中，则j排在前面
				if !existsI && existsJ {
					return false
				}
				// 如果都不在列表中，则保持原有顺序
				return i < j
			})
			for _, tag := range tags {
				tag.Link = w.GetUrl("tag", tag, 0)
				tag.GetThumb(w.PluginStorage.StorageUrl, w.GetDefaultThumb(int(tag.Id)))
				tmpResult = append(tmpResult, &model.Archive{
					Type:        "tag",
					Id:          int64(tag.Id),
					CreatedTime: tag.CreatedTime,
					UpdatedTime: tag.UpdatedTime,
					Title:       tag.Title,
					SeoTitle:    tag.SeoTitle,
					UrlToken:    tag.UrlToken,
					Keywords:    tag.Keywords,
					Description: tag.Description,
					Link:        tag.Link,
					Logo:        tag.Logo,
					Thumb:       tag.Thumb,
				})
			}
		}
		ops := func(tx *gorm.DB) *gorm.DB {
			if req.AuthorId > 0 {
				tx = tx.Where("user_id = ?", req.AuthorId)
			}
			if req.ParentId > 0 {
				tx = tx.Where("parent_id = ?", req.ParentId)
			}
			if req.Flag != "" {
				tx = tx.Joins("INNER JOIN archive_flags ON archives.id = archive_flags.archive_id and archive_flags.flag = ?", req.Flag)
			} else if len(req.ExcludeFlags) > 0 {
				tx = tx.Joins("LEFT JOIN archive_flags ON archives.id = archive_flags.archive_id and archive_flags.flag IN (?)", req.ExcludeFlags).Where("archive_flags.archive_id IS NULL")
			}
			needDistinct := false
			if len(req.ExtraFields) > 0 {
				needDistinct = true
				// 先查询module 的字段
				module = w.GetModuleFromCache(uint(req.ModuleId))
				if module != nil && len(module.Fields) > 0 {
					var fields [][2]interface{}
					for _, v := range module.Fields {
						// 如果有筛选条件，从这里开始筛选
						if param, ok := req.ExtraFields[v.FieldName]; ok && param != "" {
							paramValues := strings.Split(fmt.Sprint(param), ",")
							var validValues []string
							for _, val := range paramValues {
								val = strings.TrimSpace(val)
								if val != "" {
									validValues = append(validValues, val)
								}
							}
							if len(validValues) > 1 {
								fields = append(fields, [2]interface{}{"`" + module.TableName + "`.`" + v.FieldName + "` IN(?)", validValues})
							} else if len(validValues) == 1 {
								fields = append(fields, [2]interface{}{"`" + module.TableName + "`.`" + v.FieldName + "` = ?", validValues[0]})
							}
						}
					}
					if len(fields) > 0 {
						tx = tx.InnerJoins(fmt.Sprintf("INNER JOIN `%s` on `%s`.id = `archives`.id", module.TableName, module.TableName))
						for _, field := range fields {
							tx = tx.Where(field[0], field[1])
						}
					}
				}
				// 其它字段，价格字段也在这里，skuOptions字段也在这类
				if tmpPrice, ok := req.ExtraFields["price"]; ok {
					price := fmt.Sprint(tmpPrice)
					price = strings.ReplaceAll(price, "~", "-")
					price = strings.ReplaceAll(price, ",", "-")
					priceItems := strings.Split(price, "-")
					minPrice, _ := strconv.Atoi(priceItems[0])
					maxPrice := 0
					if len(priceItems) > 1 {
						maxPrice, _ = strconv.Atoi(priceItems[1])
					}
					if maxPrice >= minPrice {
						tx = tx.Where("archives.price >= ? AND archives.price <= ?", minPrice, maxPrice)
					} else {
						tx = tx.Where("archives.price >= ?", minPrice)
					}
				}
			}
			if w.Content.MultiCategory == 1 || needDistinct || req.Flag != "" || len(req.ExcludeFlags) > 0 || len(req.TagIds) > 0 {
				tx = tx.Group("archives.id")
			}
			if w.Content.MultiCategory == 1 && (len(req.CategoryIds) > 0 || len(req.ExcludeCategoryIds) > 0) {
				tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id")
			}
			if len(req.CategoryIds) > 0 {
				if req.Child {
					var subIds []uint
					for _, v := range req.CategoryIds {
						tmpIds := w.GetSubCategoryIds(uint(v), nil)
						subIds = append(subIds, tmpIds...)
						subIds = append(subIds, uint(v))
					}
					if w.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id IN (?)", subIds)
					} else {
						if len(subIds) == 1 {
							tx = tx.Where("`category_id` = ?", subIds[0])
						} else {
							tx = tx.Where("`category_id` IN(?)", subIds)
						}
					}
				} else {
					if w.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id IN (?)", req.CategoryIds)
					} else {
						tx = tx.Where("`category_id` IN(?)", req.CategoryIds)
					}
				}
			} else if req.ModuleId > 0 {
				tx = tx.Where("`module_id` = ?", req.ModuleId)
			}
			if len(req.ExcludeCategoryIds) > 0 {
				if w.Content.MultiCategory == 1 {
					tx = tx.Where("archive_categories.category_id NOT IN (?)", req.ExcludeCategoryIds)
				} else {
					tx = tx.Where("`category_id` NOT IN (?)", req.ExcludeCategoryIds)
				}
			}
			if len(req.TagIds) > 0 {
				tx = tx.Joins("INNER JOIN `tag_data` as t ON archives.id = t.item_id AND t.`tag_id` IN (?)", req.TagIds)
			}
			if len(req.Ids) > 0 {
				tx = tx.Where("archives.`id` IN(?)", req.Ids)
			} else if len(ids) > 0 {
				tx = tx.Where("archives.`id` IN(?)", ids)
			} else if req.Q != "" {
				// 如果文章数量达到10万，则只能匹配开头，否则就模糊搜索
				var allArchives int64
				allArchives = w.GetExplainCount("SELECT id FROM archives")
				if allArchives > 100000 {
					tx = tx.Where("`title` like ?", req.Q+"%")
				} else {
					tx = tx.Where("`title` like ?", "%"+req.Q+"%")
				}
			}
			return tx
		}
		if req.Type != "page" {
			// 如果不是分页，则不查询count
			req.Page = 0
		}
		tmpPage := req.Page
		if fulltextSearch {
			tmpPage = 1
		}
		if req.Order != "" {
			req.Order = ParseOrderBy(req.Order, "archives")
		} else {
			// 默认排序规则
			if w.Content.UseSort == 1 {
				req.Order = "archives.`sort` desc, archives.`created_time` desc"
			} else {
				req.Order = "archives.`created_time` desc"
			}
		}
		draftInt := 0
		if req.Draft {
			draftInt = 1
		}
		archives, total, _ = w.GetArchiveList(ops, req.Order, tmpPage, req.Limit, req.Offset, draftInt)
		if fulltextSearch {
			total = fulltextTotal
		}
		// 如果存在 argIds 或 ids，则按他们的顺序排序
		if len(req.Ids) > 0 || len(ids) > 0 {
			// 创建ID到位置索引的映射
			idToIndex := make(map[int64]int)
			var sortIds []int64

			if len(req.Ids) > 0 {
				sortIds = req.Ids
			} else {
				sortIds = ids
			}
			// 建立ID到索引的映射关系
			for i, id := range sortIds {
				idToIndex[id] = i
			}

			// 按照映射的索引进行排序
			sort.Slice(archives, func(i, j int) bool {
				indexI, existsI := idToIndex[archives[i].Id]
				indexJ, existsJ := idToIndex[archives[j].Id]

				// 如果两个ID都在指定列表中，则按指定顺序排序
				if existsI && existsJ {
					return indexI < indexJ
				}
				// 如果只有i在列表中，则i排在前面
				if existsI && !existsJ {
					return true
				}
				// 如果只有j在列表中，则j排在前面
				if !existsI && existsJ {
					return false
				}
				// 如果都不在列表中，则保持原有顺序
				return i < j
			})
		}
	}
	var combineArchive *model.Archive
	if req.CombineId > 0 {
		combineArchive, _ = w.GetArchiveById(req.CombineId)
	}
	var archiveIds = make([]int64, 0, len(archives))
	for i := range archives {
		archiveIds = append(archiveIds, archives[i].Id)
		if len(archives[i].Password) > 0 {
			archives[i].Password = ""
			archives[i].HasPassword = true
		}
		if combineArchive != nil {
			if req.CombineMode == "from" {
				archives[i].Link = w.GetUrl("archive", combineArchive, 0, archives[i])
			} else {
				archives[i].Link = w.GetUrl("archive", archives[i], 0, combineArchive)
			}
		}
	}

	// 读取flags,content,extra
	if len(archiveIds) > 0 {
		if req.ShowFlag {
			var flags []*model.ArchiveFlags
			w.DB.WithContext(w.Ctx()).Model(&model.ArchiveFlag{}).Where("`archive_id` IN (?)", archiveIds).Select("archive_id", "GROUP_CONCAT(`flag`) as flags").Group("archive_id").Scan(&flags)
			for i := range archives {
				for _, f := range flags {
					if f.ArchiveId == archives[i].Id {
						archives[i].Flag = f.Flags
						break
					}
				}
			}
		}
		if req.ShowCategory {
			var categoryIds []uint
			for _, archive := range archives {
				categoryIds = append(categoryIds, archive.CategoryId)
			}
			categories := w.GetCacheCategoriesByIds(categoryIds)
			for _, archive := range archives {
				for _, category := range categories {
					if archive.CategoryId == category.Id {
						archive.Category = category
						break
					}
				}
			}
		}
		if req.ShowTag {
			tags := w.GetTagsByItemIds(archiveIds)
			for _, archive := range archives {
				archive.Tags = tags[archive.Id]
			}
		}
		if req.ShowContent {
			var archiveData []model.ArchiveData
			w.DB.WithContext(w.Ctx()).Where("`id` IN (?)", archiveIds).Find(&archiveData)
			for i := range archives {
				for _, d := range archiveData {
					if d.Id == archives[i].Id {
						if req.Render {
							d.Content = library.MarkdownToHTML(d.Content, w.System.BaseUrl, w.Content.FilterOutlink)
						}
						archives[i].Content = d.Content
						break
					}
				}
			}
		}
		if req.ShowExtra && module != nil && len(module.Fields) > 0 {
			for j := range archives {
				archiveParams := w.GetArchiveExtra(archives[j].ModuleId, archives[j].Id, true)
				if len(archiveParams) > 0 {
					var extras = make(map[string]model.CustomField, len(archiveParams))
					for i := range archiveParams {
						param := *archiveParams[i]
						if (param.Value == nil || param.Value == "" || param.Value == 0) &&
							param.Type != config.CustomFieldTypeRadio &&
							param.Type != config.CustomFieldTypeCheckbox &&
							param.Type != config.CustomFieldTypeSelect {
							param.Value = param.Default
						}
						if param.FollowLevel && !archives[j].HasOrdered {
							continue
						}
						if param.Type == config.CustomFieldTypeEditor && req.Render {
							param.Value = library.MarkdownToHTML(fmt.Sprintf("%v", param.Value), w.System.BaseUrl, w.Content.FilterOutlink)
						} else if param.Type == config.CustomFieldTypeArchive {
							// 列表
							arcIds, ok := param.Value.([]int64)
							if !ok && param.Default != "" {
								value, _ := strconv.ParseInt(fmt.Sprint(param.Default), 10, 64)
								if value > 0 {
									arcIds = append(arcIds, value)
								}
							}
							if len(arcIds) > 0 {
								arcs, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
									return tx.Where("archives.`id` IN (?)", arcIds)
								}, "archives.id ASC", 0, len(arcIds))
								param.Value = arcs
							} else {
								param.Value = nil
							}
						} else if param.Type == config.CustomFieldTypeCategory {
							value, ok := param.Value.(int64)
							if !ok && param.Default != "" {
								value, _ = strconv.ParseInt(fmt.Sprint(param.Default), 10, 64)
							}
							if value > 0 {
								param.Value = w.GetCategoryFromCache(uint(value))
							} else {
								param.Value = nil
							}
						}
						extras[i] = param
					}
					archives[j].Extra = extras
				}
			}
		}
		if req.UserId > 0 {
			// 读取 favorite
			var archiveFavorites []*model.ArchiveFavorite
			w.DB.Model(&model.ArchiveFavorite{}).Where("archive_id IN(?) and user_id = ?", archiveIds, req.UserId).Find(&archiveFavorites)
			for j, archive := range archives {
				for _, favorite := range archiveFavorites {
					if archive.Id == favorite.ArchiveId {
						archives[j].IsFavorite = true
						break
					}
				}
			}
		}
	}

	tmpResult = append(archives, tmpResult...)

	return tmpResult, total
}

func (w *Website) ApiGetFilters(req *request.ApiFilterRequest) ([]response.FilterGroup, error) {
	module := w.GetModuleFromCache(uint(req.ModuleId))
	if module == nil {
		return nil, errors.New("module not found")
	}

	if req.AllText == "" {
		req.AllText = w.TplTr("All")
	}
	if req.AllText == "false" || req.ShowAll == false {
		req.AllText = ""
	}

	// 只有有多项选择的才能进行筛选，如 单选，多选，下拉，并且不是跟随阅读等级
	var newParams = make(url.Values)
	if len(req.UrlParams) > 0 {
		for k, v := range req.UrlParams {
			if k == "page" {
				continue
			}
			newParams.Set(k, v)
		}
	}
	newQuery := newParams.Encode()
	urlMatch := ""
	var matchData interface{}
	if req.CategoryId > 0 {
		category := w.GetCategoryFromCache(uint(req.CategoryId))
		if category != nil {
			matchData = category
			urlMatch = "category"
		}
	} else {
		matchData = module
		urlMatch = "archiveIndex"
	}

	urlPatten := w.GetUrl(urlMatch, matchData, 1)
	if strings.Contains(urlPatten, "?") {
		urlPatten += "&"
	} else {
		urlPatten += "?"
	}

	// 只有有多项选择的才能进行筛选，如 单选，多选，下拉
	var filterFields []config.CustomField
	var filterGroups []response.FilterGroup

	if len(module.Fields) > 0 {
		for _, v := range module.Fields {
			if v.IsFilter {
				filterFields = append(filterFields, v)
			}
		}

		// 所有参数的url都附着到query中
		for _, v := range filterFields {
			values := v.SplitContent()
			if len(values) == 0 {
				continue
			}
			var tmpUrlParam = map[string]bool{}
			if req.UrlParams != nil && req.UrlParams[v.FieldName] != "" {
				tmpData := strings.Split(req.UrlParams[v.FieldName], ",")
				for _, v := range tmpData {
					tmpUrlParam[v] = true
				}
			}

			var filterItems []response.FilterItem
			if req.AllText != "" {
				tmpParams, _ := url.ParseQuery(newQuery)
				tmpParams.Set(v.FieldName, "")
				isCurrent := false
				if len(tmpUrlParam) == 0 {
					isCurrent = true
				}
				// 需要插入 全部 标签
				filterItems = append(filterItems, response.FilterItem{
					Label:     req.AllText,
					Value:     "",
					Link:      urlPatten + tmpParams.Encode(),
					IsCurrent: isCurrent,
				})
			}
			for _, val := range values {
				tmpParams, _ := url.ParseQuery(newQuery)
				tmpParams.Set(v.FieldName, val)
				isCurrent := false
				if tmpUrlParam[val] {
					isCurrent = true
				}
				filterItems = append(filterItems, response.FilterItem{
					Label:     val,
					Value:     val,
					Link:      urlPatten + tmpParams.Encode(),
					IsCurrent: isCurrent,
				})
			}
			filterGroups = append(filterGroups, response.FilterGroup{
				Name:      v.Name,
				FieldName: v.FieldName,
				Items:     filterItems,
			})
		}
	}
	if req.ShowPrice {
		// maxPrice
		var maxPrice int64
		w.DB.Model(model.Archive{}).Select("max(price)").Scan(&maxPrice)
		tmpParams, _ := url.ParseQuery(newQuery)
		tmpParams.Set("price", req.UrlParams["price"])
		// 把价格范围分成5份
		filterGroups = append(filterGroups, response.FilterGroup{
			Name:      "Price",
			FieldName: "price",
			Range: &response.FilterRange{
				Max:   int64(math.Ceil(float64(maxPrice) / 100)),
				Min:   0,
				Value: req.UrlParams["price"],
				Link:  urlPatten + tmpParams.Encode(),
			},
		})
	}
	if req.ShowCategory {
		categories := w.GetCategoriesFromCache(uint(req.ModuleId), uint(req.ParentId), config.CategoryTypeArchive, false)
		var categoryItems []response.FilterItem
		for _, v := range categories {
			v.Link = w.GetUrl("category", v, 0)
			categoryItems = append(categoryItems, response.FilterItem{
				Label:     v.Title,
				Value:     fmt.Sprintf("%d", v.Id),
				Link:      v.Link,
				IsCurrent: v.Id == uint(req.CategoryId),
			})
		}
		filterGroups = append(filterGroups, response.FilterGroup{
			Name:      "Category",
			FieldName: "category",
			Items:     categoryItems,
		})
	}

	return filterGroups, nil
}

func (w *Website) ApiGetArchiveParams(req *request.ApiArchiveRequest) ([]model.CustomField, error) {
	var archive *model.Archive
	var err error
	if req.Id > 0 {
		archive = w.GetArchiveByIdFromCache(req.Id)
	}
	if req.UrlToken != "" {
		// 处理特殊的 prev and next
		if req.UrlToken == "prev" || req.UrlToken == "next" {
			if archive == nil {
				return nil, errors.New("no archive found")
			}
			if req.UrlToken == "prev" {
				archive, err = w.GetPreviousArchive(int64(archive.CategoryId), archive.Id)
				if err != nil {
					return nil, err
				}
			} else {
				archive, err = w.GetNextArchive(int64(archive.CategoryId), archive.Id)
				if err != nil {
					return nil, err
				}
			}
		} else {
			archive, err = w.GetArchiveByUrlToken(req.UrlToken)
		}
	}
	if archive == nil {
		return nil, errors.New("no archive found")
	}

	archiveParams := w.GetArchiveExtra(archive.ModuleId, archive.Id, true)
	// if read level larger than 0, then need to check permission
	if req.UserId > 0 {
		archive = w.CheckArchiveHasOrder(req.UserId, archive, req.UserGroup)
		if archive.Price > 0 {
			discount := w.GetUserDiscount(req.UserId, req.UserInfo)
			if discount > 0 {
				archive.FavorablePrice = archive.Price * discount / 100
			}
		}
	}

	var extras = make(map[string]model.CustomField, len(archiveParams))
	if len(archiveParams) > 0 {
		for i := range archiveParams {
			param := *archiveParams[i]
			if (param.Value == nil || param.Value == "" || param.Value == 0) &&
				param.Type != config.CustomFieldTypeRadio &&
				param.Type != config.CustomFieldTypeCheckbox &&
				param.Type != config.CustomFieldTypeSelect {
				param.Value = param.Default
			}
			if param.FollowLevel && !archive.HasOrdered {
				continue
			}
			if param.Type == config.CustomFieldTypeEditor && req.Render {
				param.Value = library.MarkdownToHTML(fmt.Sprintf("%v", param.Value), w.System.BaseUrl, w.Content.FilterOutlink)
			} else if param.Type == config.CustomFieldTypeArchive {
				// 列表
				arcIds, ok := param.Value.([]int64)
				if !ok && param.Default != "" {
					value, _ := strconv.ParseInt(fmt.Sprint(param.Default), 10, 64)
					if value > 0 {
						arcIds = append(arcIds, value)
					}
				}
				if len(arcIds) > 0 {
					archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
						return tx.Where("archives.`id` IN (?)", arcIds)
					}, "archives.id ASC", 0, len(arcIds))
					param.Value = archives
				} else {
					param.Value = nil
				}
			} else if param.Type == config.CustomFieldTypeCategory {
				value, ok := param.Value.(int64)
				if !ok && param.Default != "" {
					value, _ = strconv.ParseInt(fmt.Sprint(param.Default), 10, 64)
				}
				if value > 0 {
					param.Value = w.GetCategoryFromCache(uint(value))
				} else {
					param.Value = nil
				}
			}
			extras[i] = param
		}
	}

	var extraFields []model.CustomField
	module := w.GetModuleFromCache(archive.ModuleId)
	if module != nil && len(module.Fields) > 0 {
		for _, v := range module.Fields {
			extraFields = append(extraFields, extras[v.FieldName])
		}
	}

	return extraFields, nil
}

func (w *Website) ApiGetCategory(req *request.ApiCategoryRequest) (*model.Category, error) {
	var category *model.Category

	if req.Id > 0 {
		category = w.GetCategoryFromCache(uint(req.Id))
	} else if req.UrlToken != "" {
		// 处理特殊的 prev and next
		category = w.GetCategoryFromCacheByToken(req.UrlToken)
	}
	if category == nil {
		return nil, errors.New("no category found")
	}

	category.Thumb = category.GetThumb(w.PluginStorage.StorageUrl, w.GetDefaultThumb(int(category.Id)))
	// convert markdown to html
	if req.Render {
		category.Content = library.MarkdownToHTML(category.Content, w.System.BaseUrl, w.Content.FilterOutlink)
	}
	category.Content = w.ReplaceContentUrl(category.Content, true)
	// extra replace
	if category.Extra != nil {
		module := w.GetModuleFromCache(category.ModuleId)
		if module != nil && len(module.CategoryFields) > 0 {
			categoryExtra := map[string]interface{}{}
			for _, field := range module.CategoryFields {
				categoryExtra[field.FieldName] = category.Extra[field.FieldName]
				if (categoryExtra[field.FieldName] == nil || categoryExtra[field.FieldName] == "" || categoryExtra[field.FieldName] == 0) &&
					field.Type != config.CustomFieldTypeRadio &&
					field.Type != config.CustomFieldTypeCheckbox &&
					field.Type != config.CustomFieldTypeSelect {
					// default
					categoryExtra[field.FieldName] = field.Content
				}
				if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
					categoryExtra[field.FieldName] != nil {
					value, ok2 := categoryExtra[field.FieldName].(string)
					if ok2 {
						if field.Type == config.CustomFieldTypeEditor && req.Render {
							value = library.MarkdownToHTML(value, w.System.BaseUrl, w.Content.FilterOutlink)
						}
						categoryExtra[field.FieldName] = w.ReplaceContentUrl(value, true)
					}
				} else if field.Type == config.CustomFieldTypeImages && categoryExtra[field.FieldName] != nil {
					if val, ok := categoryExtra[field.FieldName].([]interface{}); ok {
						for j, v2 := range val {
							v2s, _ := v2.(string)
							val[j] = w.ReplaceContentUrl(v2s, true)
						}
						categoryExtra[field.FieldName] = val
					}
				} else if field.Type == config.CustomFieldTypeTexts && categoryExtra[field.FieldName] != nil {
					var texts []model.CustomFieldTexts
					_ = json.Unmarshal([]byte(fmt.Sprint(categoryExtra[field.FieldName])), &texts)
					categoryExtra[field.FieldName] = texts
				} else if field.Type == config.CustomFieldTypeTimeline && categoryExtra[field.FieldName] != nil {
					var val model.TimelineField
					_ = json.Unmarshal([]byte(fmt.Sprint(categoryExtra[field.FieldName])), &val)
					categoryExtra[field.FieldName] = val
				} else if field.Type == config.CustomFieldTypeArchive && categoryExtra[field.FieldName] != nil {
					// 列表
					var arcIds []int64
					buf, _ := json.Marshal(categoryExtra[field.FieldName])
					_ = json.Unmarshal(buf, &arcIds)
					if len(arcIds) == 0 && field.Content != "" {
						value, _ := strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
						if value > 0 {
							arcIds = append(arcIds, value)
						}
					}
					if len(arcIds) > 0 {
						archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
							return tx.Where("archives.`id` IN (?)", arcIds)
						}, "archives.id ASC", 0, len(arcIds))
						categoryExtra[field.FieldName] = archives
					} else {
						categoryExtra[field.FieldName] = nil
					}
				} else if field.Type == config.CustomFieldTypeCategory {
					value, err := strconv.ParseInt(fmt.Sprint(categoryExtra[field.FieldName]), 10, 64)
					if err != nil && field.Content != "" {
						value, _ = strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
					}
					if value > 0 {
						categoryExtra[field.FieldName] = w.GetCategoryFromCache(uint(value))
					} else {
						categoryExtra[field.FieldName] = nil
					}
				}
			}
			category.Extra = categoryExtra
		}
	}

	return category, nil
}

func (w *Website) ApiGetCategories(req *request.ApiCategoryListRequest) ([]*model.Category, int64) {

	categoryList := w.GetCategoriesFromCache(uint(req.ModuleId), uint(req.ParentId), config.CategoryTypeArchive, req.All)
	var total int64 = int64(len(categoryList))
	var resultList []*model.Category
	for i := 0; i < len(categoryList); i++ {
		if req.Offset > i {
			continue
		}
		if req.Limit > 0 && i >= (req.Limit+req.Offset) {
			break
		}
		categoryList[i].GetThumb(w.PluginStorage.StorageUrl, w.GetDefaultThumb(int(categoryList[i].Id)))
		categoryList[i].Link = w.GetUrl("category", categoryList[i], 0)
		categoryList[i].IsCurrent = false
		resultList = append(resultList, categoryList[i])
	}

	return resultList, total
}

func (w *Website) ApiGetTag(req *request.ApiTagRequest) (*model.Tag, error) {
	var tagDetail *model.Tag
	var err error
	if req.Id > 0 {
		tagDetail, err = w.GetTagById(uint(req.Id))
	} else if req.UrlToken != "" {
		// 处理特殊的 prev and next
		tagDetail, err = w.GetTagByUrlToken(req.UrlToken)
	}
	if err != nil {
		return nil, errors.New("no tag found")
	}

	tagDetail.Link = w.GetUrl("tag", tagDetail, 0)
	tagDetail.GetThumb(w.PluginStorage.StorageUrl, w.GetDefaultThumb(int(tagDetail.Id)))
	tagContent, err := w.GetTagContentById(tagDetail.Id)
	if err == nil {
		tagDetail.Content = tagContent.Content
		// convert markdown to html
		if req.Render {
			tagDetail.Content = library.MarkdownToHTML(tagDetail.Content, w.System.BaseUrl, w.Content.FilterOutlink)
		}
		tagDetail.Extra = tagContent.Extra
		if tagDetail.Extra != nil {
			fields := w.GetTagFields()
			if len(fields) > 0 {
				for _, field := range fields {
					if (tagDetail.Extra[field.FieldName] == nil || tagDetail.Extra[field.FieldName] == "" || tagDetail.Extra[field.FieldName] == 0) &&
						field.Type != config.CustomFieldTypeRadio &&
						field.Type != config.CustomFieldTypeCheckbox &&
						field.Type != config.CustomFieldTypeSelect {
						// default
						tagDetail.Extra[field.FieldName] = field.Content
					}
					if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
						tagDetail.Extra[field.FieldName] != nil {
						value, ok2 := tagDetail.Extra[field.FieldName].(string)
						if ok2 {
							if field.Type == config.CustomFieldTypeEditor && req.Render {
								value = library.MarkdownToHTML(value, w.System.BaseUrl, w.Content.FilterOutlink)
							}
							tagDetail.Extra[field.FieldName] = w.ReplaceContentUrl(value, true)
						}
					}
					if field.Type == config.CustomFieldTypeImages && tagDetail.Extra[field.FieldName] != nil {
						if val, ok := tagDetail.Extra[field.FieldName].([]interface{}); ok {
							for j, v2 := range val {
								v2s, _ := v2.(string)
								val[j] = w.ReplaceContentUrl(v2s, true)
							}
							tagDetail.Extra[field.FieldName] = val
						}
					} else if field.Type == config.CustomFieldTypeTexts && tagDetail.Extra[field.FieldName] != nil {
						var texts []model.CustomFieldTexts
						_ = json.Unmarshal([]byte(fmt.Sprint(tagDetail.Extra[field.FieldName])), &texts)
						tagDetail.Extra[field.FieldName] = texts
					} else if field.Type == config.CustomFieldTypeTimeline && tagDetail.Extra[field.FieldName] != nil {
						var val model.TimelineField
						_ = json.Unmarshal([]byte(fmt.Sprint(tagDetail.Extra[field.FieldName])), &val)
						tagDetail.Extra[field.FieldName] = val
					} else if field.Type == config.CustomFieldTypeArchive && tagDetail.Extra[field.FieldName] != nil {
						// 列表
						var arcIds []int64
						buf, _ := json.Marshal(tagDetail.Extra[field.FieldName])
						_ = json.Unmarshal(buf, &arcIds)
						if len(arcIds) == 0 && field.Content != "" {
							value, _ := strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
							if value > 0 {
								arcIds = append(arcIds, value)
							}
						}
						if len(arcIds) > 0 {
							archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
								return tx.Where("archives.`id` IN (?)", arcIds)
							}, "archives.id ASC", 0, len(arcIds))
							tagDetail.Extra[field.FieldName] = archives
						} else {
							tagDetail.Extra[field.FieldName] = nil
						}
					} else if field.Type == config.CustomFieldTypeCategory {
						value, err := strconv.ParseInt(fmt.Sprint(tagDetail.Extra[field.FieldName]), 10, 64)
						if err != nil && field.Content != "" {
							value, _ = strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
						}
						if value > 0 {
							tagDetail.Extra[field.FieldName] = w.GetCategoryFromCache(uint(value))
						} else {
							tagDetail.Extra[field.FieldName] = nil
						}
					}
				}
			}
		}
	}

	return tagDetail, nil
}

func (w *Website) ApiGetTags(req *request.ApiTagListRequest) ([]*model.Tag, int64) {
	if req.Type == "page" {
		if req.Page > 1 {
			req.Offset = (req.Page - 1) * req.Limit
		}
	}
	var categoryIds []uint
	for _, v := range req.CategoryIds {
		categoryIds = append(categoryIds, uint(v))
	}

	tagList, total, _ := w.GetTagList(req.ItemId, "", categoryIds, req.Letter, req.Page, req.Limit, req.Offset, req.Order)
	for i := range tagList {
		tagList[i].Link = w.GetUrl("tag", tagList[i], 0)
		tagList[i].GetThumb(w.PluginStorage.StorageUrl, w.GetDefaultThumb(int(tagList[i].Id)))
	}

	return tagList, total
}

var (
	fieldNameRegex  = regexp.MustCompile("^`?[a-zA-Z0-9_]+`?$")
	tableFieldRegex = regexp.MustCompile("^`?[a-zA-Z0-9_]+`?\\.`?[a-zA-Z0-9_]+`?$")
	sqlFuncRegex    = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*\(.*\)$`)
)

func ParseOrderBy(order string, prefix string) string {
	if order == "" {
		return ""
	}
	if prefix != "" && strings.HasSuffix(prefix, ".") {
		prefix = prefix[:len(prefix)-1]
	}

	orders := strings.Split(order, ",")
	validOrders := make([]string, 0, len(orders))

	for _, o := range orders {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}

		// 分割字段和排序方向
		fields := strings.Fields(o)
		if len(fields) == 0 {
			continue
		}

		fieldName := fields[0]

		processedField, isValid := processFieldName(fieldName, prefix)
		if !isValid {
			continue // 跳过无效字段
		}
		orderStr := processedField

		// 检查排序方向
		if len(fields) > 1 {
			direction := strings.ToUpper(fields[1])
			if direction == "ASC" || direction == "DESC" {
				orderStr += " " + fields[1]
			}
			// 如果有额外参数，忽略
		}

		validOrders = append(validOrders, orderStr)
	}

	if len(validOrders) == 0 {
		return ""
	}

	return strings.Join(validOrders, ", ")
}

// processFieldName 处理字段名，应用表名前缀并验证安全性
func processFieldName(fieldName string, prefix string) (string, bool) {
	// 特殊处理 RAND() 函数（大小写不敏感）
	if strings.EqualFold(fieldName, "rand()") || strings.EqualFold(fieldName, "rand") {
		return "RAND()", true
	}
	// 检查是否已经是完整的表名.字段名格式
	if tableFieldRegex.MatchString(fieldName) {
		// 验证表名和字段名都安全
		parts := strings.Split(fieldName, ".")
		if len(parts) == 2 {
			if fieldNameRegex.MatchString(parts[0]) && fieldNameRegex.MatchString(parts[1]) {
				return fieldName, true
			}
		}
		return "", false
	}

	// 检查是否为 SQL 函数调用
	if sqlFuncRegex.MatchString(fieldName) {
		// 验证函数调用安全性
		if isValidSQLFunction(fieldName) {
			return fieldName, true
		}
		return "", false
	}

	// 检查是否为普通字段名
	if fieldNameRegex.MatchString(fieldName) {
		// 如果提供了前缀，则添加前缀
		if prefix != "" {
			return prefix + "." + fieldName, true
		}
		return fieldName, true
	}

	return "", false
}

// isValidSQLFunction 验证 SQL 函数调用的安全性
func isValidSQLFunction(funcCall string) bool {
	// 提取函数名和参数
	openParen := strings.Index(funcCall, "(")
	if openParen == -1 {
		return false
	}

	funcName := funcCall[:openParen]
	params := funcCall[openParen+1 : len(funcCall)-1] // 去掉末尾的 ")"

	// 验证函数名
	if !fieldNameRegex.MatchString(funcName) {
		return false
	}

	// 常见的允许的 SQL 函数白名单
	allowedFunctions := map[string]bool{
		"rand": true, "random": true, "length": true, "char_length": true,
		"upper": true, "lower": true, "substr": true, "substring": true,
		"concat": true, "coalesce": true, "nullif": true, "max": true, "min": true, "sum": true,
	}

	funcNameLower := strings.ToLower(funcName)
	if _, ok := allowedFunctions[funcNameLower]; !ok {
		return false
	}

	// 验证参数（可以包含逗号、空格和字段名）
	if params == "" {
		return true // 允许无参数函数
	}

	// 分割多个参数
	paramList := strings.Split(params, ",")
	for _, param := range paramList {
		param = strings.TrimSpace(param)

		// 允许数字字面量
		if isNumeric(param) {
			continue
		}

		// 允许字符串字面量（单引号包围）
		if len(param) >= 2 && param[0] == '\'' && param[len(param)-1] == '\'' {
			// 简单的字符串字面量验证
			continue
		}

		// 允许字段名
		if !fieldNameRegex.MatchString(param) {
			return false
		}
	}

	return true
}

// isNumeric 检查字符串是否为数字
func isNumeric(s string) bool {
	if s == "" {
		return false
	}

	// 检查是否全是数字
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func (w *Website) ApiGetSystemSetting() *config.SystemConfig {
	setting := *w.System
	if !strings.HasPrefix(setting.SiteLogo, "http") {
		setting.SiteLogo = w.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(w.System.SiteLogo, "/")
	}
	return &setting
}

func (w *Website) ApiGetContactSetting() *config.ContactConfig {
	setting := *w.Contact
	if !strings.HasPrefix(setting.Qrcode, "http") {
		setting.Qrcode = w.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(setting.Qrcode, "/")
	}

	return &setting
}

func (w *Website) ApiGetIndexSetting() *config.IndexConfig {
	setting := *w.Index

	return &setting
}

func (w *Website) ApiGetDiyFields(render bool) []config.ExtraField {
	fields := w.GetDiyFieldSetting()
	var newFields = make([]config.ExtraField, 0, len(fields))
	for _, field := range fields {
		if (field.Value == nil || field.Value == "" || field.Value == 0) &&
			field.Type != config.CustomFieldTypeRadio &&
			field.Type != config.CustomFieldTypeCheckbox &&
			field.Type != config.CustomFieldTypeSelect {
			// default
			field.Value = field.Content
		}
		if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
			field.Value != nil {
			value, ok2 := field.Value.(string)
			if ok2 {
				if field.Type == config.CustomFieldTypeEditor && render {
					value = library.MarkdownToHTML(value, w.System.BaseUrl, w.Content.FilterOutlink)
				}
				field.Value = w.ReplaceContentUrl(value, true)
			}
		} else if field.Type == config.CustomFieldTypeImages && field.Value != nil {
			if val, ok := field.Value.([]interface{}); ok {
				for j, v2 := range val {
					v2s, _ := v2.(string)
					val[j] = w.ReplaceContentUrl(v2s, true)
				}
				field.Value = val
			}
		} else if field.Type == config.CustomFieldTypeTexts && field.Value != nil {
			var texts []model.CustomFieldTexts
			_ = json.Unmarshal([]byte(fmt.Sprint(field.Value)), &texts)
			field.Value = texts
		} else if field.Type == config.CustomFieldTypeTimeline && field.Value != nil {
			var val model.TimelineField
			_ = json.Unmarshal([]byte(fmt.Sprint(field.Value)), &val)
			field.Value = val
		} else if field.Type == config.CustomFieldTypeArchive && field.Value != nil {
			// 列表
			var arcIds []int64
			buf, _ := json.Marshal(field.Value)
			_ = json.Unmarshal(buf, &arcIds)
			if len(arcIds) == 0 && field.Content != "" {
				value, _ := strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
				if value > 0 {
					arcIds = append(arcIds, value)
				}
			}
			if len(arcIds) > 0 {
				archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
					return tx.Where("archives.`id` IN (?)", arcIds)
				}, "archives.id ASC", 0, len(arcIds))
				field.Value = archives
			} else {
				field.Value = nil
			}
		} else if field.Type == config.CustomFieldTypeCategory {
			value, err := strconv.ParseInt(fmt.Sprint(field.Value), 10, 64)
			if err != nil && field.Content != "" {
				value, _ = strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
			}
			if value > 0 {
				field.Value = w.GetCategoryFromCache(uint(value))
			} else {
				field.Value = nil
			}
		}

		newFields = append(newFields, field)
	}

	return fields
}

func (w *Website) ApiGetGuestbookFields() []*config.CustomField {
	fields := w.GetGuestbookFields()
	for i := range fields {
		//分割items
		fields[i].SplitContent()
	}

	return fields
}

func (w *Website) ApiGetBanners(bannerType string) ([]*config.BannerItem, error) {
	var bannerList = make([]*config.BannerItem, 0, 10)
	for _, tmpList := range w.Banner.Banners {
		if tmpList.Type == bannerType {
			for _, banner := range tmpList.List {
				if !strings.HasPrefix(banner.Logo, "http") && !strings.HasPrefix(banner.Logo, "//") {
					banner.Logo = w.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(banner.Logo, "/")
				}
				bannerList = append(bannerList, &banner)
			}
		}
	}

	return bannerList, nil
}

func (w *Website) ApiGetLanguages() ([]config.MultiLangSite, error) {
	// 获取当前的链接
	mainId := w.ParentId
	if mainId == 0 {
		mainId = w.Id
	}

	mainSite := GetWebsite(mainId)
	if mainSite.MultiLanguage.Open == false {
		return nil, nil
	}

	languages := w.GetMultiLangSites(mainId, false)

	return languages, nil
}

func (w *Website) GetMetadata(params map[string]string) *response.WebInfo {
	var err error
	var currentPage = 1
	if params["page"] != "" {
		currentPage, err = strconv.Atoi(params["page"])
		if err != nil {
			currentPage = 1
		}
	}
	webInfo := &response.WebInfo{
		StatusCode: 200,
		Params:     params,
	}

	switch params["match"] {
	case "notfound":
		// 走到 not Found
		webInfo.StatusCode = 404
		webInfo.Title = "404 Not Found"
		break
	case PatternArchive:
		id, _ := strconv.ParseInt(params["id"], 10, 64)
		urlToken := params["filename"]
		var archive *model.Archive
		var err error
		if urlToken != "" {
			//优先使用urlToken
			archive, err = w.GetArchiveByUrlToken(urlToken)
		} else {
			archive, err = w.GetArchiveById(id)
		}
		if err != nil {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}
		archive.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
		webInfo.Title = archive.Title
		if archive.SeoTitle != "" {
			webInfo.Title = archive.SeoTitle
		}
		webInfo.Keywords = archive.Keywords
		webInfo.Description = archive.Description
		webInfo.NavBar = int64(archive.CategoryId)
		webInfo.PageId = archive.Id
		webInfo.ModuleId = int64(archive.ModuleId)
		webInfo.Image = archive.Logo
		//设置页面名称，方便tags识别
		webInfo.PageName = "archiveDetail"
		webInfo.CanonicalUrl = archive.CanonicalUrl
		if webInfo.CanonicalUrl == "" {
			webInfo.CanonicalUrl = w.GetUrl("archive", archive, 0)
		}
		break
	case PatternArchiveIndex:
		urlToken := params["module"]
		module := w.GetModuleFromCacheByToken(urlToken)
		if module == nil {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}
		webInfo.Title = module.Title
		webInfo.Keywords = module.Keywords
		webInfo.Description = module.Description

		//设置页面名称，方便tags识别
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "archiveIndex"
		webInfo.NavBar = int64(module.Id)
		webInfo.PageId = int64(module.Id)
		webInfo.ModuleId = int64(module.Id)
		webInfo.CanonicalUrl = w.GetUrl("archiveIndex", module, 0)
		break
	case PatternCategory:
		categoryId, _ := strconv.ParseInt(params["id"], 10, 64)
		catId, _ := strconv.ParseInt(params["catid"], 10, 64)
		if catId > 0 {
			categoryId = catId
		}
		var category *model.Category
		urlToken := params["filename"]
		multiCatNames := params["multicatname"]
		if multiCatNames != "" {
			chunkCatNames := strings.Split(multiCatNames, "/")
			urlToken = chunkCatNames[len(chunkCatNames)-1]
			isErr := false
			for _, catName := range chunkCatNames {
				tmpCat := w.GetCategoryFromCacheByToken(catName, category)
				if tmpCat == nil || (category != nil && tmpCat.ParentId != category.Id) {
					isErr = true
					break
				}
				category = tmpCat
			}
			if isErr {
				webInfo.StatusCode = 404
				webInfo.Title = "404 Not Found"
				break
			}
		} else {
			if urlToken != "" {
				//优先使用urlToken
				category = w.GetCategoryFromCacheByToken(urlToken)
			} else {
				category = w.GetCategoryFromCache(uint(categoryId))
			}
		}
		if category == nil || category.Status != config.ContentStatusOK {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}
		category.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
		webInfo.Title = category.Title
		if category.SeoTitle != "" {
			webInfo.Title = category.SeoTitle
		}
		webInfo.CurrentPage = currentPage
		webInfo.Keywords = category.Keywords
		webInfo.Description = category.Description
		webInfo.NavBar = int64(category.Id)
		webInfo.PageId = int64(category.Id)
		webInfo.ModuleId = int64(category.ModuleId)
		webInfo.PageName = "archiveList"
		webInfo.CanonicalUrl = w.GetUrl("category", category, currentPage)
		break
	case PatternPage:
		categoryId, _ := strconv.ParseInt(params["id"], 10, 64)
		catId, _ := strconv.ParseInt(params["catid"], 10, 64)
		if catId > 0 {
			categoryId = catId
		}
		urlToken := params["filename"]
		var category *model.Category
		if urlToken != "" {
			//优先使用urlToken
			category = w.GetCategoryFromCacheByToken(urlToken)
		} else {
			category = w.GetCategoryFromCache(uint(categoryId))
		}
		if category == nil || category.Status != config.ContentStatusOK {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}

		//修正，如果这里读到的的category，则跳到category中
		if category.Type != config.CategoryTypePage {
			webInfo.StatusCode = 301
			webInfo.Title = "301 Redirect"
			webInfo.CanonicalUrl = w.GetUrl("category", category, 0)
			break
		}
		category.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
		webInfo.Title = category.Title
		if category.SeoTitle != "" {
			webInfo.Title = category.SeoTitle
		}
		webInfo.Keywords = category.Keywords
		webInfo.Description = category.Description
		webInfo.NavBar = int64(category.Id)
		webInfo.PageId = int64(category.Id)
		webInfo.PageName = "pageDetail"
		webInfo.CanonicalUrl = w.GetUrl("page", category, 0)
		break
	case PatternSearch:
		q := strings.TrimSpace(params["q"])
		moduleToken := params["module"]
		var module *model.Module
		if len(moduleToken) > 0 {
			module = w.GetModuleFromCacheByToken(moduleToken)
		}

		webInfo.Title = w.TplTr("Search%s", "")
		if module != nil {
			webInfo.Title = module.Title + webInfo.Title
			webInfo.ModuleId = int64(module.Id)
		}
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "search"
		webInfo.CanonicalUrl = w.GetUrl(fmt.Sprintf("/search?q=%s(&page={page})", url.QueryEscape(q)), nil, currentPage)
		break
	case PatternTagIndex:
		webInfo.Title = w.TplTr("TagList")
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "tagIndex"
		webInfo.CanonicalUrl = w.GetUrl("tagIndex", nil, currentPage)
		break
	case PatternTag:
		tagId, _ := strconv.ParseInt(params["id"], 10, 64)
		urlToken := params["filename"]
		var tag *model.Tag
		var err error
		if urlToken != "" {
			//优先使用urlToken
			tag, err = w.GetTagByUrlToken(urlToken)
		} else {
			tag, err = w.GetTagById(uint(tagId))
		}
		if err != nil {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}
		tag.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
		webInfo.Title = tag.Title
		if tag.SeoTitle != "" {
			webInfo.Title = tag.SeoTitle
		}
		webInfo.CurrentPage = currentPage
		webInfo.Keywords = tag.Keywords
		webInfo.Description = tag.Description
		webInfo.NavBar = int64(tag.Id)
		webInfo.PageId = int64(tag.Id)
		webInfo.PageName = "tag"
		webInfo.CanonicalUrl = w.GetUrl("tag", tag, currentPage)
		break
	case "index":
		webTitle := w.Index.SeoTitle
		webInfo.Title = webTitle
		webInfo.Keywords = w.Index.SeoKeywords
		webInfo.Description = w.Index.SeoDescription
		webInfo.Image = w.System.SiteLogo
		//设置页面名称，方便tags识别
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "index"
		webInfo.CanonicalUrl = w.GetUrl("", nil, 0)
		break
	case PatternPeople:
		id, _ := strconv.ParseInt(params["id"], 10, 64)
		urlToken := params["filename"]
		var user *model.User
		var err error
		if urlToken != "" {
			//优先使用urlToken
			user, err = w.GetUserInfoByUrlToken(urlToken)
		} else {
			user, err = w.GetUserInfoById(uint(id))
		}
		if err != nil {
			webInfo.StatusCode = 404
			webInfo.Title = "404 Not Found"
			break
		}

		webInfo.Title = user.UserName
		webInfo.NavBar = int64(user.Id)
		webInfo.PageId = int64(user.Id)
		webInfo.PageName = "userDetail"
		webInfo.CanonicalUrl = w.GetUrl(PatternPeople, user, 0)
		break
	}

	return webInfo
}
