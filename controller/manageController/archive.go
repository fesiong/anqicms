package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"time"
)

func ArchiveList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))
	moduleId := uint(ctx.URLParamIntDefault("module_id", 0))
	status := ctx.URLParam("status") // 支持 '':all，draft:0, ok:1, plan:2
	// 回收站
	recycle, _ := ctx.URLParamBool("recycle")
	// 采集的
	collect, _ := ctx.URLParamBool("collect")
	if currentPage < 1 {
		currentPage = 1
	}

	var ops func(tx *gorm.DB) *gorm.DB
	if recycle {
		ops = func(tx *gorm.DB) *gorm.DB {
			return tx.Unscoped().Where("`deleted_at` is not null").Order("id desc")
		}
	} else if collect {
		ops = func(tx *gorm.DB) *gorm.DB {
			return tx.Where("`origin_url` != ''").Order("id desc")
		}
	} else {
		// 必须传递分类
		title := ctx.URLParam("title")
		ops = func(tx *gorm.DB) *gorm.DB {
			if moduleId > 0 {
				tx = tx.Where("`module_id` = ?", moduleId)
			}
			if categoryId > 0 {
				tx = tx.Where("`category_id` = ?", categoryId)
			}
			if status == "draft" {
				tx = tx.Where("`status` = ?", config.ContentStatusDraft)
			} else if status == "ok" {
				tx = tx.Where("`status` = ?", config.ContentStatusOK)
			} else if status == "plan" {
				tx = tx.Where("`status` = ?", config.ContentStatusPlan)
			}
			if title != "" {
				tx = tx.Where("`title` like ?", "%"+title+"%")
			}
			tx = tx.Order("id desc")
			return tx
		}
	}
	archives, total, err := currentSite.GetArchiveList(ops, currentPage, pageSize)

	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	//读取列表的分类
	categories := currentSite.GetCacheCategories()
	// 模型
	modules := currentSite.GetCacheModules()
	for i, v := range archives {
		if v.CategoryId > 0 {
			for _, c := range categories {
				if c.Id == v.CategoryId {
					archives[i].Category = &c
					break
				}
			}
		}
		for _, c := range modules {
			if c.Id == v.ModuleId {
				archives[i].ModuleName = c.Title
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

	archive, err := currentSite.GetArchiveById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 读取data
	archive.ArchiveData, err = currentSite.GetArchiveDataById(archive.Id)
	// 读取 extraDat
	archive.Extra = currentSite.GetArchiveExtra(archive.ModuleId, archive.Id, false)

	tags := currentSite.GetTagsByItemId(archive.Id)
	if len(tags) > 0 {
		var tagNames = make([]string, 0, len(tags))
		for _, v := range tags {
			tagNames = append(tagNames, v.Title)
		}
		archive.Tags = tagNames
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": archive,
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
				"msg":  "相同标题的内容已存在",
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
				"msg":  "已存在相同的固定链接，请更换一个固定链接再提交",
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新文档：%d => %s", archive.Id, archive.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文档已更新",
		"data": archive,
	})
}

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
	archive, err := currentSite.GetUnscopedArchiveById(req.Id)
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("恢复文档：%d => %s", archive.Id, archive.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文章已恢复",
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
	archive, err := currentSite.GetUnscopedArchiveById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 只有待发布的需要发布
	if archive.Status == config.ContentStatusDraft {
		archive.Status = config.ContentStatusOK
		archive.CreatedTime = time.Now().Unix()
		currentSite.DB.Save(archive)
		err = currentSite.SuccessReleaseArchive(archive, true)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		currentSite.AddAdminLog(ctx, fmt.Sprintf("发布文档：%d => %s", archive.Id, archive.Title))
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文章已发布",
	})
}

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
	archive, err := currentSite.GetUnscopedArchiveById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.DeleteArchive(archive)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除文档：%d => %s", archive.Id, archive.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文章已删除",
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
	archive, err := currentSite.GetUnscopedArchiveById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if len(archive.Images) > req.ImageIndex {
		archive.Images = append(archive.Images[:req.ImageIndex], archive.Images[req.ImageIndex+1:]...)
	} else {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "图片不存在",
		})
		return
	}

	currentSite.DB.Save(archive)

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除文档图片：%d => %s", archive.Id, archive.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文章图片已删除",
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("批量更新文档Flag：%v => %s", req.Ids, req.Flag))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文章已更新",
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("批量更新文档状态：%v => %d", req.Ids, req.Status))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文章已更新",
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("批量更新文档时间：%v => %d", req.Ids, req.Time))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文章已更新",
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("批量更新文档分类：%v => %d", req.Ids, req.CategoryId))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文章已更新",
	})
}
