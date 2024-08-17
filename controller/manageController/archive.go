package manageController

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
	"time"
)

func ArchiveList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))
	moduleId := uint(ctx.URLParamIntDefault("module_id", 0))
	status := ctx.URLParamDefault("status", "ok") // 支持 '':all，draft:0, ok:1, plan:2
	sort := ctx.URLParamDefault("sort", "id")
	flag := ctx.URLParam("flag")
	order := strings.ToLower(ctx.URLParamDefault("order", "desc"))
	if order != "asc" {
		order = "desc"
	}
	if sort == "" {
		sort = "id"
	}
	sort = "archives." + sort
	orderBy := sort + " " + order
	// 回收站
	recycle, _ := ctx.URLParamBool("recycle")
	if recycle {
		status = "delete"
	}
	// 采集的
	collect, _ := ctx.URLParamBool("collect")
	if currentPage < 1 {
		currentPage = 1
	}

	var archives []*model.ArchiveDraft

	var ops func(tx *gorm.DB) *gorm.DB

	var dbTable = func(tx *gorm.DB) *gorm.DB {
		if status == "ok" {
			tx = tx.Table("`archives` as archives").Select("*, 1 as status")
		} else {
			tx = tx.Table("`archive_drafts` as archives")
		}
		return tx
	}

	if collect {
		ops = func(tx *gorm.DB) *gorm.DB {
			return tx.Where("`origin_url` != ''").Order(orderBy)
		}
	} else {
		// 必须传递分类
		title := ctx.URLParam("title")
		ops = func(tx *gorm.DB) *gorm.DB {
			if categoryId > 0 {
				subIds := currentSite.GetSubCategoryIds(categoryId, nil)
				subIds = append(subIds, categoryId)
				if currentSite.Content.MultiCategory == 1 {
					tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id and archive_categories.category_id IN (?)", subIds)
				} else {
					if len(subIds) == 1 {
						tx = tx.Where("`category_id` = ?", subIds[0])
					} else {
						tx = tx.Where("`category_id` IN(?)", subIds)
					}
				}
			} else if moduleId > 0 {
				tx = tx.Where("`module_id` = ?", moduleId)
			}
			if status == "delete" {
				tx = tx.Where("`status` = ?", config.ContentStatusDelete)
			} else if status == "draft" {
				tx = tx.Where("`status` = ?", config.ContentStatusDraft)
			} else if status == "ok" {
				// nothing to do
			} else if status == "plan" {
				tx = tx.Where("`status` = ?", config.ContentStatusPlan)
			}
			if flag != "" {
				tx = tx.Joins("INNER JOIN archive_flags ON archives.id = archive_flags.archive_id and archive_flags.flag = ?", flag)
			}
			if title != "" {
				tx = tx.Where("`title` like ?", "%"+title+"%")
			}
			tx = tx.Order(orderBy)
			return tx
		}
	}
	offset := 0
	if currentPage > 0 {
		offset = (currentPage - 1) * pageSize
	}
	builder := dbTable(ops(currentSite.DB))

	builder = dbTable(builder)

	var total int64
	builder.Count(&total)
	// 先查询ID
	var archiveIds []uint
	builder.Limit(pageSize).Offset(offset).Select("archives.id").Pluck("id", &archiveIds)
	if len(archiveIds) > 0 {
		dbTable(currentSite.DB).Where("id IN (?)", archiveIds).Order(orderBy).Scan(&archives)
		for i := range archives {
			archives[i].GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
			archives[i].Link = currentSite.GetUrl("archive", archives[i], 0)
		}
	}

	//读取列表的分类
	categories := currentSite.GetCacheCategories()
	for i := range categories {
		categories[i].Link = currentSite.GetUrl("category", categories[i], 0)
	}
	// 模型
	modules := currentSite.GetCacheModules()
	for i, v := range archives {
		if currentSite.Content.MultiCategory == 1 {
			var catIds []uint
			currentSite.DB.Model(&model.ArchiveCategory{}).Where("`archive_id` = ?", v.Id).Pluck("category_id", &catIds)
			for _, catId := range catIds {
				for _, cat := range categories {
					if cat.Id == catId {
						title := cat.Title
						if cat.Status != config.ContentStatusOK {
							title += ctx.Tr("(Hide)")
						}
						archives[i].CategoryTitles = append(archives[i].CategoryTitles, title)
					}
				}
			}
			archives[i].CategoryIds = catIds
		} else if v.CategoryId > 0 {
			for _, c := range categories {
				if c.Id == v.CategoryId {
					title := c.Title
					if c.Status != config.ContentStatusOK {
						title += ctx.Tr("(Hide)")
					}
					archives[i].CategoryTitles = []string{title}
					break
				}
			}
			archives[i].CategoryIds = []uint{archives[i].CategoryId}
		}
		for _, c := range modules {
			if c.Id == v.ModuleId {
				archives[i].ModuleName = c.Title
			}
		}
	}
	// 读取flags
	if len(archiveIds) > 0 {
		var flags []*model.ArchiveFlags
		currentSite.DB.Model(&model.ArchiveFlag{}).Where("`archive_id` IN (?)", archiveIds).Select("archive_id", "GROUP_CONCAT(`flag`) as flags").Group("archive_id").Scan(&flags)
		for i := range archives {
			for _, f := range flags {
				if f.ArchiveId == archives[i].Id {
					archives[i].Flag = f.Flags
					break
				}
			}
		}
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  archives,
	})
}

func ArchiveDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	archiveDraft, err := currentSite.GetArchiveDraftById(id)
	if err != nil {
		// 从草稿读取
		archive, err := currentSite.GetArchiveById(id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		archiveDraft = &model.ArchiveDraft{
			Archive: *archive,
			Status:  config.ContentStatusOK,
		}
	}
	// 读取分类
	if currentSite.Content.MultiCategory == 1 {
		var catIds []uint
		currentSite.DB.Model(&model.ArchiveCategory{}).Where("`archive_id` = ?", archiveDraft.Id).Pluck("category_id", &catIds)
		archiveDraft.CategoryIds = catIds
	} else {
		archiveDraft.CategoryIds = []uint{archiveDraft.CategoryId}
	}
	// 读取flag
	archiveDraft.Flag = currentSite.GetArchiveFlags(archiveDraft.Id)

	// 读取data
	archiveDraft.ArchiveData, err = currentSite.GetArchiveDataById(archiveDraft.Id)
	// 读取 extraDat
	archiveDraft.Extra = currentSite.GetArchiveExtra(archiveDraft.ModuleId, archiveDraft.Id, false)
	// 读取relation
	archiveDraft.Relations = currentSite.GetArchiveRelations(archiveDraft.Id)

	tags := currentSite.GetTagsByItemId(archiveDraft.Id)
	if len(tags) > 0 {
		var tagNames = make([]string, 0, len(tags))
		for _, v := range tags {
			tagNames = append(tagNames, v.Title)
		}
		archiveDraft.Tags = tagNames
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": archiveDraft,
	})
}

func ArchiveDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.Archive
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 检查是否有重名
	if !req.ForceSave {
		exists, err := currentSite.GetArchiveByTitle(req.Title)
		if err == nil && exists.Id != req.Id {
			// 做提示
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("ContentWithTheSameTitleAlreadyExists"),
				"data": exists,
			})
			return
		}
		// 再检查草稿
		exists2, err := currentSite.GetArchiveDraftByTitle(req.Title)
		if err == nil && exists2.Id != req.Id {
			// 做提示
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("ContentWithTheSameTitleAlreadyExists"),
				"data": exists,
			})
			return
		}
	}
	// 检查 fixed_link
	if req.FixedLink != "" {
		exists, err := currentSite.GetArchiveByFixedLink(req.FixedLink)
		if err == nil && exists.Id != req.Id {
			// 做提示
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("TheSameFixedLinkAlreadyExists"),
			})
			return
		}
		// 再检查草稿
		exists2, err := currentSite.GetArchiveDraftByFixedLink(req.FixedLink)
		if err == nil && exists2.Id != req.Id {
			// 做提示
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("TheSameFixedLinkAlreadyExists"),
			})
			return
		}
	}

	archive, err := currentSite.SaveArchive(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateDocumentLog", archive.Id, archive.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DocumentUpdated"),
		"data": archive,
	})
}

// ArchiveRecover
// 从回收站恢复
func ArchiveRecover(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.Archive
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	archive, err := currentSite.GetArchiveDraftById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.RecoverArchive(archive)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("RestoreDocumentLog", archive.Id, archive.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ArticleRestored"),
	})
}

func ArchiveRelease(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.Archive
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	archive, err := currentSite.GetArchiveDraftById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	archive.CreatedTime = time.Now().Unix()
	currentSite.PublishPlanArchive(archive)

	currentSite.AddAdminLog(ctx, ctx.Tr("PublishDocumentLog", archive.Id, archive.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ArticlePublished"),
	})
}

// ArchiveDelete
// 删除文档，从正式表删除，或从草稿箱删除
func ArchiveDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.Archive
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	archive, err := currentSite.GetArchiveById(req.Id)
	if err == nil {
		// 文档在正式表，把它移到草稿箱
		err = currentSite.DeleteArchive(archive)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		currentSite.AddAdminLog(ctx, ctx.Tr("DeleteDocumentLog", archive.Id, archive.Title))
	} else {
		// 从草稿表删除
		archiveDraft, err := currentSite.GetArchiveDraftById(req.Id)
		if err == nil {
			err = currentSite.DeleteArchiveDraft(archiveDraft)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
			currentSite.AddAdminLog(ctx, ctx.Tr("DeleteDocumentLog", archiveDraft.Id, archiveDraft.Title))
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ArticleDeleted"),
	})
}

func ArchiveDeleteImage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchiveImageDeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	isReleased := false
	archiveDraft, err := currentSite.GetArchiveDraftById(req.Id)
	if err != nil {
		archive, err := currentSite.GetArchiveById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		isReleased = true
		archiveDraft = &model.ArchiveDraft{
			Archive: *archive,
			Status:  config.StatusOK,
		}
	}

	if len(archiveDraft.Images) > req.ImageIndex {
		archiveDraft.Images = append(archiveDraft.Images[:req.ImageIndex], archiveDraft.Images[req.ImageIndex+1:]...)
	} else {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("ImageDoesNotExist"),
		})
		return
	}

	if isReleased {
		currentSite.DB.Save(&archiveDraft.Archive)
	} else {
		currentSite.DB.Save(archiveDraft)
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteDocumentImageLog", archiveDraft.Id, archiveDraft.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ArticleImagesHaveBeenDeleted"),
	})
}

func UpdateArchiveRecommend(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchivesUpdateRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.UpdateArchiveRecommend(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("BatchUpdateDocumentFlagLog", req.Ids, req.Flag))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ArticleUpdated"),
	})
}

func UpdateArchiveStatus(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchivesUpdateRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.UpdateArchiveStatus(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("BatchUpdateDocumentStatusLog", req.Ids, req.Status))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ArticleUpdated"),
	})
}

func UpdateArchiveTime(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchivesUpdateRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.UpdateArchiveTime(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("BatchUpdateDocumentTimeLog", req.Ids, req.Time))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ArticleUpdated"),
	})
}

func UpdateArchiveReleasePlan(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchivesUpdateRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.UpdateArchiveReleasePlan(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("BatchUpdateDocumentScheduledReleaseLog", req.Ids, req.DailyLimit))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ArticleUpdated"),
	})
}

func UpdateArchiveSort(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.Archive
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DB.Model(&model.Archive{}).Where("id = ?", req.Id).UpdateColumn("sort", req.Sort).Error
	err = currentSite.DB.Model(&model.ArchiveDraft{}).Where("id = ?", req.Id).UpdateColumn("sort", req.Sort).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 删除列表缓存
	currentSite.Cache.CleanAll("archive-list")

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateDocumentCustomSortLog", req.Id, req.Sort))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SortingUpdated"),
	})
}

func UpdateArchiveCategory(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchivesUpdateRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.UpdateArchiveCategory(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("BatchUpdateDocumentCategoryLog", req.Ids, req.CategoryIds))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ArticleUpdated"),
	})
}
