package manageController

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"regexp"
	"strings"
)

func PluginUserFieldsSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"fields": currentSite.GetUserFields(),
		},
	})
}

func PluginUserFieldsSettingForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginUserConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var fields []*config.CustomField
	var existsFields = map[string]struct{}{}
	for _, v := range req.Fields {
		if !v.IsSystem {
			if v.FieldName == "" {
				v.FieldName = strings.ReplaceAll(library.GetPinyin(v.Name, currentSite.Content.UrlTokenType == config.UrlTokenTypeSort), "-", "_")
			}
		}
		// 检查fields
		match, err := regexp.MatchString(`^[a-z][0-9a-z_]+$`, v.FieldName)
		if err != nil || !match {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  v.FieldName + ctx.Tr("IncorrectNaming"),
			})
			return
		}
		v.Required = false
		if _, ok := existsFields[v.FieldName]; !ok {
			existsFields[v.FieldName] = struct{}{}
			fields = append(fields, v)
		}
	}

	currentSite.PluginUser.Fields = fields

	err := currentSite.SaveSettingValue(provider.UserSettingKey, currentSite.PluginUser)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// sync table
	currentSite.MigrateUserTable(fields, true)

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyUserExtraField"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginUserFieldsDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ModuleFieldRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteUserField(req.FieldName)

	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteUserFieldLog", req.Id, req.FieldName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("FieldDeleted"),
	})
}

func PluginUserList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	userId := uint(ctx.URLParamIntDefault("id", 0))
	groupId := uint(ctx.URLParamIntDefault("group_id", 0))
	userName := ctx.URLParam("user_name")
	realName := ctx.URLParam("realName")
	phone := ctx.URLParam("phone")

	ops := func(tx *gorm.DB) *gorm.DB {
		if userId > 0 {
			tx = tx.Where("`id` = ?", userId)
		}
		if groupId > 0 {
			tx = tx.Where("`group_id` = ?", userId)
		}
		if phone != "" {
			tx = tx.Where("`phone` = ?", phone)
		}
		if userName != "" {
			tx = tx.Where("`user_name` like ?", "%"+userName+"%")
		}
		if realName != "" {
			tx = tx.Where("`real_name` like ?", "%"+realName+"%")
		}
		tx = tx.Order("id desc")
		return tx
	}
	users, total := currentSite.GetUserList(ops, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  users,
	})
}

func PluginUserDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	user, err := currentSite.GetUserInfoById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": user,
	})
}

func PluginUserDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveUserInfo(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateUserLog", req.Id, req.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}

func PluginUserDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteUserInfo(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteUserLog", req.Id, req.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

func PluginUserGroupList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	groups := currentSite.GetUserGroups()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": groups,
	})
}

func PluginUserGroupDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	group, err := currentSite.GetUserGroupInfo(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": group,
	})
}

func PluginUserGroupDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserGroupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveUserGroupInfo(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateUserGroupLog", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}

func PluginUserGroupDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserGroupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteUserGroup(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteUserGroupLog", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}
