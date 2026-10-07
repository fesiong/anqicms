package manageController

import (
	"regexp"
	"strings"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginUserFieldsSetting 获取当前站点的用户插件设置，包括用户扩展字段配置。
func PluginUserFieldsSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": currentSite.PluginUser,
	})
}

// PluginUserFieldsSettingForm 保存用户插件设置，包括默认用户组、默认状态和用户扩展字段，并同步用户表结构。
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

	currentSite.PluginUser.DefaultGroupId = req.DefaultGroupId
	currentSite.PluginUser.DefaultStatus = req.DefaultStatus
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

// PluginUserFieldsDelete 删除指定的用户扩展字段。
//
// 参数说明：
//   - 请求体 "id": 字段 ID（缺省）。
//   - 请求体 "field_name": 字段名称。
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

// PluginUserList 获取用户列表，支持分页和多条件筛选。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "id": 用户 ID 精确筛选。
//   - 查询参数 "group_id": 用户组 ID 筛选。
//   - 查询参数 "user_name": 用户名模糊搜索。
//   - 查询参数 "real_name": 真实姓名模糊搜索。
//   - 查询参数 "phone": 手机号精确搜索。
//   - 查询参数 "q": 对用户名、真实姓名、手机号的综合模糊搜索。
//   - 查询参数 "status": 用户状态筛选，可选 normal、blocked、pending。
//   - 查询参数 "user_type": 用户类型筛选，可选 all、subscribed、ordered、repurchase。
func PluginUserList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	userId := uint(ctx.URLParamIntDefault("id", 0))
	groupId := uint(ctx.URLParamIntDefault("group_id", 0))
	userName := ctx.URLParam("user_name")
	realName := ctx.URLParam("real_name")
	phone := ctx.URLParam("phone")
	q := ctx.URLParam("q")
	status := ctx.URLParam("status")
	userType := ctx.URLParam("user_type") // all, subscribed, ordered,repurchase

	ops := func(tx *gorm.DB) *gorm.DB {
		if userId > 0 {
			tx = tx.Where("`id` = ?", userId)
		}
		if groupId > 0 {
			tx = tx.Where("`group_id` = ?", userId)
		}
		if q != "" {
			tx = tx.Where("`user_name` like ? or `real_name` like ? or `phone` like ?", "%"+q+"%", "%"+q+"%", "%"+q+"%")
		} else {
			if phone != "" {
				tx = tx.Where("`phone` = ?", phone)
			}
			if userName != "" {
				tx = tx.Where("`user_name` like ?", "%"+userName+"%")
			}
			if realName != "" {
				tx = tx.Where("`real_name` like ?", "%"+realName+"%")
			}
		}
		if status != "" {
			if status == "normal" {
				tx = tx.Where("`status` = ?", 1)
			} else if status == "blocked" {
				tx = tx.Where("`status` = ?", -1)
			} else if status == "pending" {
				tx = tx.Where("`status` = ?", 0)
			}
		}
		// 订阅
		if userType == "subscribed" {
			tx = tx.Where("`subscribed` = ?", true)
		} else if userType == "ordered" {
			tx = tx.Where("order_count > 0")
		} else if userType == "repurchase" {
			tx = tx.Where("order_count > 1")
		}
		tx = tx.Order("users.id desc")
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

// PluginUserDetail 获取指定 ID 的用户详情。
//
// 参数说明：
//   - 查询参数 "id": 用户 ID，必填。
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

// PluginUserDetailForm 保存（新增或更新）用户信息。
//
// 参数说明：
//   - 请求体 "id": 用户 ID，大于 0 表示更新。
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
		// UpdateAll 决定「未传的字段」怎么处理：
		//   true  —— 全量覆盖。未传=零值的字段会被清空。这是**表单提交**的语义：
		//            前端提交整个表单，某个字段没出现在表单里就意味着用户清空了它。
		//   false —— PATCH 语义。只覆盖显式传入的字段，未传的一律保持库里的原值。
		//            这是 **AI/程序化调用**需要的语义：只想改标题，不该动其它字段。
		//
		// 用 Partial（反向开关）而非直接暴露 UpdateAll，是为了区分「调用方没传这个字段」
		// 与「调用方显式传了 false」——Go 的 bool 零值做不到，前端又不传该字段。
		// Partial 优先于调用方传的 update_all。
		if !req.Partial {
			req.UpdateAll = true
		}

	user, err := currentSite.SaveUserInfo(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateUserLog", user.Id, user.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": user,
	})
}

// PluginUserChangeBalance 变更指定用户的余额，金额和备注必填。
//
// 参数说明：
//   - 请求体 "user_id": 用户 ID。
//   - 请求体 "amount": 变更金额，不能为 0，单位：分。
//   - 请求体 "remark": 变更备注。
func PluginUserChangeBalance(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ApiUserBalanceRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Amount == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseEnterTheAmount"),
		})
		return
	}

	if req.Remark == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseEnterRemarks"),
		})
		return
	}

	err := currentSite.UpdateUserBalance(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateUserLog", req.UserId, "balance"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}

// PluginUserDelete 删除指定 ID 的用户。
func PluginUserDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserDeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	user, err := currentSite.GetUserInfoById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	err = currentSite.DeleteUserInfo(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteUserLog", req.Id, user.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

// PluginUserGroupList 获取当前站点的全部用户组列表。
func PluginUserGroupList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	groups := currentSite.GetUserGroups()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": groups,
	})
}

// PluginUserGroupDetail 获取指定 ID 的用户组详情。
//
// 参数说明：
//   - 查询参数 "id": 用户组 ID，必填。
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

// PluginUserGroupDetailForm 保存（新增或更新）用户组信息。
//
// 参数说明：
//   - 请求体 "id": 用户组 ID，大于 0 表示更新。
//   - 请求体 "title": 用户组名称。
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

	group, err := currentSite.SaveUserGroupInfo(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateUserGroupLog", group.Id, group.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": group,
	})
}

// PluginUserGroupDelete 根据指定 ID 删除对应用户组。
func PluginUserGroupDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserGroupDeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	group, err := currentSite.GetUserGroupInfo(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	err = currentSite.DeleteUserGroup(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteUserGroupLog", req.Id, group.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}
