package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"net"
	"net/url"
	"os"
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

	safeSetting := currentSite.Safe
	if safeSetting.AdminCaptchaOff != 1 {
		// 验证 captcha
		if req.CaptchaId == "" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("VerificationCodeIsIncorrect"),
			})
			return
		}
		if ok := controller.Store.Verify(req.CaptchaId, req.Captcha, true); !ok {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("VerificationCodeIsIncorrect"),
			})
			return
		}
	}

	// 如果连续错了5次，则只能10分钟后再试
	if currentSite.AdminLoginError.Times >= 5 {
		if currentSite.AdminLoginError.LastTime > time.Now().Add(-10*time.Minute).Unix() {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("AdministratorHasBeenTemporarilyLocked"),
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
			"msg":  ctx.Tr("PleaseEnterUsername"),
		})
		return
	}
	//验证密码
	if len(req.Password) < 6 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseEnterAPasswordOf6CharactersOrMore"),
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
			"msg":  ctx.Tr("AdministratorAccountOrPasswordIsIncorrect"),
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
			"msg":  ctx.Tr("AdministratorAccountOrPasswordIsIncorrect"),
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
		"msg":  ctx.Tr("LoginSuccessful"),
		"data": admin,
	})
}

func AdminLogout(ctx iris.Context) {
	// todo
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LoggedOut"),
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
			"msg":  ctx.Tr("UserDoesNotExist"),
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
				"msg":  ctx.Tr("AdministratorDoesNotExist"),
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
				"msg":  ctx.Tr("TheAccountAlreadyExists"),
			})
			return
		}
		admin = &model.Admin{}
	}
	if req.UserName == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("TheAccountCannotBeEmpty"),
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
				"msg":  ctx.Tr("TheCurrentPasswordIsIncorrect"),
			})
			return
		}
		admin.EncryptPassword(req.Password)
	}
	err = currentSite.DB.Save(admin).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("UpdateInfoError"),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateAdministratorLog", admin.Id, admin.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("AdministratorHasBeenUpdated"),
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
			"msg":  ctx.Tr("ThisAdministratorCannotBeDeleted"),
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
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteAdministratorLog", req.Id, req.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
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
			"msg":  ctx.Tr("GroupNameCannotBeEmpty"),
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
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateAdministratorGroupLog", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
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
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteAdministratorGroupLog", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
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

func FindPasswordChooseWay(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.FindPasswordChooseRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 支持2种方式找回，file 文件上传验证, dns 解析验证
	if req.Way != config.PasswordFindWayFile && req.Way != config.PasswordFindWayDNS {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidVerificationMethod"),
		})
		return
	}
	var host = ""
	if req.Way == config.PasswordFindWayDNS {
		parsed, err := url.Parse(currentSite.System.BaseUrl)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("DomainNameResolutionFailed"),
			})
			return
		}

		host = "_anqicms" + "." + parsed.Hostname()
	}

	if currentSite.FindPasswordInfo == nil {
		currentSite.FindPasswordInfo = &response.FindPasswordInfo{
			Token: library.Md5(currentSite.TokenSecret + fmt.Sprintf("%d", time.Now().UnixNano())),
		}
	} else {
		currentSite.FindPasswordInfo.Timer.Stop()
	}
	currentSite.FindPasswordInfo.Host = host
	currentSite.FindPasswordInfo.Way = req.Way
	currentSite.FindPasswordInfo.End = time.Now().Add(59 * time.Minute)
	currentSite.FindPasswordInfo.Timer = time.AfterFunc(1*time.Hour, func() {
		currentSite.FindPasswordInfo = nil
	})

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": currentSite.FindPasswordInfo,
	})
}

func FindPasswordVerify(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if currentSite.FindPasswordInfo == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VerificationHasExpired"),
		})
		return
	}

	if currentSite.FindPasswordInfo.Way == config.PasswordFindWayFile {
		filePath := currentSite.PublicPath + currentSite.FindPasswordInfo.Token + ".txt"
		buf, err := os.ReadFile(filePath)

		if err != nil || strings.TrimSpace(string(buf)) != currentSite.FindPasswordInfo.Token {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("FileDoesNotExistOrTheContentIsIncorrect"),
			})
			return
		}
	} else {
		txt, err := net.LookupTXT(currentSite.FindPasswordInfo.Host)
		if err != nil || len(txt) == 0 || txt[0] != currentSite.FindPasswordInfo.Token {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("DnsResolutionDoesNotExistOrTheContentIsIncorrect"),
			})
			return
		}
	}
	currentSite.FindPasswordInfo.Verified = true

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("VerificationSuccessful"),
		"data": currentSite.FindPasswordInfo,
	})
}

func FindPasswordReset(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if currentSite.FindPasswordInfo == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VerificationHasExpired"),
		})
		return
	}
	if !currentSite.FindPasswordInfo.Verified {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("AuthorizationFailed"),
		})
		return
	}
	var req request.FindPasswordReset
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.UserName == "" || len(req.Password) < 6 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheAdministratorAccountAndPassword"),
		})
		return
	}
	admin, err := currentSite.GetAdminInfoById(1)
	if err != nil {
		admin = &model.Admin{
			Model: model.Model{
				Id: 1,
			},
		}
	}
	admin.UserName = req.UserName
	err = admin.EncryptPassword(req.Password)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PasswordSettingFailed"),
		})
		return
	}
	err = currentSite.DB.Save(admin).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("UpdateInfoError"),
		})
		return
	}
	currentSite.FindPasswordInfo.Timer.Stop()
	currentSite.FindPasswordInfo = nil

	currentSite.AddAdminLog(ctx, ctx.Tr("ResetAdministratorAccountAndPasswordLog", admin.Id, admin.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("AdministratorAccountAndPasswordHaveBeenReset"),
	})
}
