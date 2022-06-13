package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"strings"
	"time"
)

var adminLoginError = response.LoginError{}

func AdminLogin(ctx iris.Context) {
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
	if adminLoginError.Times >= 5 {
		if adminLoginError.LastTime > time.Now().Add(-10*time.Minute).Unix() {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "管理员已被临时锁定，请稍后重试",
			})
			return
		} else {
			adminLoginError.Times = 0
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

	admin, err := provider.GetAdminByUserName(req.UserName)
	if err != nil {
		adminLoginError.Times++
		adminLoginError.LastTime = time.Now().Unix()

		// 记录日志
		adminLog := model.AdminLoginLog{
			AdminId:  0,
			Ip:       ctx.RemoteAddr(),
			Status:   0,
			UserName: req.UserName,
			Password: req.Password,
		}
		dao.DB.Create(&adminLog)

		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "管理员账号或密码错误",
		})
		return
	}

	if !admin.CheckPassword(req.Password) {
		adminLoginError.Times++
		adminLoginError.LastTime = time.Now().Unix()

		// 记录日志
		adminLog := model.AdminLoginLog{
			AdminId:  admin.Id,
			Ip:       ctx.RemoteAddr(),
			Status:   0,
			UserName: req.UserName,
			Password: req.Password,
		}
		dao.DB.Create(&adminLog)

		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "管理员账号或密码错误",
		})
		return
	}

	// 重置管理员登录失败次数
	adminLoginError.Times = 0
	admin.Token = provider.GetAdminAuthToken(admin.Id, ctx.RemoteAddr(), req.Remember)

	// 记录日志
	adminLog := model.AdminLoginLog{
		AdminId:  admin.Id,
		Ip:       ctx.RemoteAddr(),
		Status:   1,
		UserName: req.UserName,
		Password: "",
	}
	dao.DB.Create(&adminLog)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "登录成功",
		"data": admin,
	})
}

func UserLogout(ctx iris.Context) {
	// todo

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已退出登录",
	})
}

func UserDetail(ctx iris.Context) {
	adminId := uint(ctx.Values().GetIntDefault("adminId", 0))

	admin, err := provider.GetAdminById(adminId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "用户不存在",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": admin,
	})
}

func UserDetailForm(ctx iris.Context) {
	var req request.ChangeAdmin
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	adminId := uint(ctx.Values().GetIntDefault("adminId", 0))

	admin, err := provider.GetAdminById(adminId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "用户不存在",
		})
		return
	}

	admin.UserName = req.UserName
	if req.Password != "" {
		if !admin.CheckPassword(req.OldPassword) {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "当前密码不正确",
			})
			return
		}
		admin.EncryptPassword(req.Password)
	}

	err = admin.Save(dao.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "更新信息出错",
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新管理员信息：%d => %s", admin.Id, admin.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "管理员信息已更新",
	})
}

func GetAdminLoginLog(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize

	var logs []model.AdminLoginLog
	var total int64
	dao.DB.Model(&model.AdminLoginLog{}).Count(&total).Limit(pageSize).Offset(offset).Order("id desc").Find(&logs)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  logs,
	})
}

func GetAdminLog(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize

	var logs []model.AdminLog
	var total int64
	dao.DB.Model(&model.AdminLog{}).Count(&total).Limit(pageSize).Offset(offset).Order("id desc").Find(&logs)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  logs,
	})
}
