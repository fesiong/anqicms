package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
	"time"
)

func AdminLogin(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AdminInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 验证 captcha
	if req.CaptchaId == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "验证码不正确",
		})
		return
	}
	if ok := controller.Store.Verify(req.CaptchaId, req.Captcha, true); !ok {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "验证码不正确",
		})
		return
	}

	// 如果连续错了5次，则只能10分钟后再试
	if currentSite.AdminLoginError.Times >= 5 {
		if currentSite.AdminLoginError.LastTime > time.Now().Add(-10*time.Minute).Unix() {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "管理员已被临时锁定，请稍后重试",
			})
			return
		} else {
			currentSite.AdminLoginError.Times = 0
		}
	}

	req.UserName = strings.TrimSpace(req.UserName)
	req.Password = strings.TrimSpace(req.Password)

	if req.UserName == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请输入用户名",
		})
		return
	}
	//验证密码
	if len(req.Password) < 6 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请输入6位及以上长度的密码",
		})
		return
	}

	admin, err := currentSite.GetAdminByUserName(req.UserName)
	if err != nil {
		currentSite.AdminLoginError.Times++
		currentSite.AdminLoginError.LastTime = time.Now().Unix()

		// 记录日志
		adminLog := model.AdminLoginLog{
			AdminId:  0,
			Ip:       ctx.RemoteAddr(),
			Status:   0,
			UserName: req.UserName,
			Password: req.Password,
		}
		currentSite.DB.Create(&adminLog)

		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "管理员账号或密码错误",
		})
		return
	}

	if !admin.CheckPassword(req.Password) {
		currentSite.AdminLoginError.Times++
		currentSite.AdminLoginError.LastTime = time.Now().Unix()

		// 记录日志
		adminLog := model.AdminLoginLog{
			AdminId:  admin.Id,
			Ip:       ctx.RemoteAddr(),
			Status:   0,
			UserName: req.UserName,
			Password: req.Password,
		}
		currentSite.DB.Create(&adminLog)

		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "管理员账号或密码错误",
		})
		return
	}

	// 重置管理员登录失败次数
	currentSite.AdminLoginError.Times = 0
	admin.Token = currentSite.GetAdminAuthToken(admin.Id, req.Remember)

	// 记录日志
	adminLog := model.AdminLoginLog{
		AdminId:  admin.Id,
		Ip:       ctx.RemoteAddr(),
		Status:   1,
		UserName: req.UserName,
		Password: "",
	}
	currentSite.DB.Create(&adminLog)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "登录成功",
		"data": admin,
	})
}

func AdminLogout(ctx iris.Context) {
	// todo

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已退出登录",
	})
}

func AdminList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	searchId := uint(ctx.URLParamIntDefault("id", 0))
	groupId := uint(ctx.URLParamIntDefault("group_id", 0))
	userName := ctx.URLParam("user_name")

	ops := func(tx *gorm.DB) *gorm.DB {
		if searchId > 0 {
			tx = tx.Where("`id` = ?", searchId)
		}
		if groupId > 0 {
			tx = tx.Where("`group_id` = ?", groupId)
		}
		if userName != "" {
			tx = tx.Where("`user_name` like ?", "%"+userName+"%")
		}
		tx = tx.Order("id desc")
		return tx
	}
	users, total := currentSite.GetAdminList(ops, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  users,
	})
}

func AdminDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	adminId := ctx.Values().GetUintDefault("adminId", 0)
	queryId := uint(ctx.URLParamIntDefault("id", 0))
	if queryId == 0 {
		queryId = adminId
	}

	admin, err := currentSite.GetAdminInfoById(queryId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "用户不存在",
		})
		return
	}
	admin.SiteId = currentSite.Id

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": admin,
	})
}

func AdminDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AdminInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	adminId := ctx.Values().GetUintDefault("adminId", 0)
	var admin *model.Admin
	var err error

	if req.Id > 0 {
		admin, err = currentSite.GetAdminInfoById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "管理员不存在",
			})
			return
		}
		if admin.Id == adminId {
			req.Status = 1
		}
	} else {
		admin, err = currentSite.GetAdminByUserName(req.UserName)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "该账号已存在",
			})
			return
		}
		admin = &model.Admin{}
	}
	if req.UserName == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "账号不能为空",
		})
		return
	}

	admin.GroupId = req.GroupId
	admin.Status = req.Status
	admin.UserName = req.UserName
	if req.Password != "" {
		if req.OldPassword != "" && !admin.CheckPassword(req.OldPassword) {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "当前密码不正确",
			})
			return
		}
		admin.EncryptPassword(req.Password)
	}
	err = currentSite.DB.Save(admin).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "更新信息出错",
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新管理员信息：%d => %s", admin.Id, admin.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "管理员信息已更新",
	})
}

func AdminDetailDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AdminInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	adminId := ctx.Values().GetUintDefault("adminId", 0)
	// 不能删除自己，不能删除id = 1 的管理员
	if adminId == 1 || req.Id == adminId {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "该管理员不可删除",
		})
		return
	}

	err := currentSite.DeleteAdminInfo(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除管理员：%d => %s", req.Id, req.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func GetAdminLoginLog(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize

	var logs []model.AdminLoginLog
	var total int64
	currentSite.DB.Model(&model.AdminLoginLog{}).Count(&total).Limit(pageSize).Offset(offset).Order("id desc").Find(&logs)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  logs,
	})
}

func GetAdminLog(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize

	var logs []model.AdminLog
	var total int64
	currentSite.DB.Model(&model.AdminLog{}).Count(&total).Limit(pageSize).Offset(offset).Order("id desc").Find(&logs)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  logs,
	})
}

func AdminGroupList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	groups := currentSite.GetAdminGroups()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": groups,
	})
}

func AdminGroupDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	group, err := currentSite.GetAdminGroupInfo(id)
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

func AdminGroupDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.GroupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Title == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "分组名称不能为空",
		})
		return
	}

	err := currentSite.SaveAdminGroupInfo(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新管理员组信息：%d => %s", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
	})
}

func AdminGroupDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.GroupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteAdminGroup(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除管理员组：%d => %s", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

// AdminMenus 后台操作按钮
func AdminMenus(ctx iris.Context) {

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": config.DefaultMenuGroups,
	})
}
