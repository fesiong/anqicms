package manageController

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

// AdminLogin 管理员登录，验证账号密码（支持验证码和签名登录），成功后返回带 token 的管理员信息。
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
	req.UserName = strings.TrimSpace(req.UserName)
	req.Password = strings.TrimSpace(req.Password)

	// 站点切换登录：校验服务端签发的一次性票据，通过后直接建立会话。
	if req.Sign != "" && req.Nonce != "" {
		if req.SiteId > 0 {
			ctx.Values().Set("siteId", req.SiteId)
			currentSite = provider.CurrentSite(ctx)
		}

		// 与账号密码登录共用同一套 IP 锁定计数
		keyPrefix := "forbidden-admin-"
		storeKey := keyPrefix + ctx.RemoteAddr()
		var loginError response.LoginError
		if err := currentSite.Cache.Get(storeKey, &loginError); err == nil && loginError.Times >= 5 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("AdministratorHasBeenTemporarilyLocked"),
			})
			return
		}

		// 先验票、再查账号，失败提示统一，不区分「账号不存在」和「票据不对」
		ticketErr := currentSite.VerifyAdminSSO(req.UserName, req.Nonce, req.Sign)
		if ticketErr != nil {
			var ipLoginError response.LoginError
			if err := currentSite.Cache.Get(storeKey, &ipLoginError); err == nil {
				ipLoginError.Times++
			} else {
				ipLoginError.Times = 1
			}
			ipLoginError.LastTime = time.Now().Unix()
			_ = currentSite.Cache.Set(storeKey, ipLoginError, 600)

			currentSite.DB.Create(&model.AdminLoginLog{
				Ip:       ctx.RemoteAddr(),
				Status:   0,
				UserName: req.UserName,
			})

			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("LoginFailed"),
			})
			return
		}

		admin, err := currentSite.GetAdminByUserName(req.UserName)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("LoginFailed"),
			})
			return
		}
		if admin.Status != 1 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("AdministratorHasBeenTemporarilyLocked"),
			})
			return
		}

		// 验证通过，直接完成登录
		currentSite.Cache.Delete(storeKey)
		admin.Token = currentSite.GetAdminAuthToken(admin.Id, req.Remember)
		admin.IsSuper = currentSite.Id == 1 && admin.GroupId == 1
		currentSite.DB.Model(admin).UpdateColumn("login_time", time.Now().Unix())

		// 记录日志
		adminLog := model.AdminLoginLog{
			AdminId:  admin.Id,
			Ip:       ctx.RemoteAddr(),
			Status:   1,
			UserName: req.UserName,
			Password: "",
		}
		currentSite.DB.Create(&adminLog)
		admin.SiteId = currentSite.Id

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("LoginSuccessful"),
			"data": admin,
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

	if req.UserName == "" || req.Password == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseEnterUsername"),
		})
		return
	}

	// 如果连续错了5次，则只能10分钟后再试
	// 如果IP被封了，则不再检查
	keyPrefix := "forbidden-admin-"
	storeKey := keyPrefix + ctx.RemoteAddr()
	var loginError response.LoginError
	err := currentSite.Cache.Get(storeKey, &loginError)
	if err == nil && loginError.Times >= 5 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("AdministratorHasBeenTemporarilyLocked"),
		})
		return
	}
	// 先验证账号对不对，如果账号对，那就封账号
	admin, err := currentSite.GetAdminByUserName(req.UserName)
	if err == nil {
		// 如果密码错误,封账号
		storeKey = keyPrefix + admin.UserName
		err = currentSite.Cache.Get(storeKey, &loginError)
		if err == nil && loginError.Times >= 5 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("AdministratorHasBeenTemporarilyLocked"),
			})
			return
		}
	} else {
		loginError.Times++
		loginError.LastTime = time.Now().Unix()
		// 保存 store, 封禁10分钟
		_ = currentSite.Cache.Set(storeKey, loginError, 600)
		// 记录日志
		adminLog := model.AdminLoginLog{
			AdminId:  0,
			Ip:       ctx.RemoteAddr(),
			Status:   0,
			UserName: req.UserName,
		}
		currentSite.DB.Create(&adminLog)

		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("AdministratorAccountOrPasswordIsIncorrect"),
		})
		return
	}
	// 账号被禁用
	if admin.Status != 1 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("AdministratorHasBeenTemporarilyLocked"),
		})
		return
	}
	// 到这里的时候，账号对了，还需要验证密码
	if !admin.CheckPassword(req.Password) {
		loginError.Times++
		loginError.LastTime = time.Now().Unix()
		// 保存 store, 封禁10分钟
		_ = currentSite.Cache.Set(storeKey, loginError, 600)

		// 同时累加 IP 封禁计数，防止轮换用户名绕过
		ipStoreKey := keyPrefix + ctx.RemoteAddr()
		var ipLoginError response.LoginError
		if err := currentSite.Cache.Get(ipStoreKey, &ipLoginError); err == nil {
			ipLoginError.Times++
		} else {
			ipLoginError.Times = 1
		}
		ipLoginError.LastTime = time.Now().Unix()
		_ = currentSite.Cache.Set(ipStoreKey, ipLoginError, 600)

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

	// 登录成功，重置管理员登录失败次数
	currentSite.Cache.Delete(keyPrefix + ctx.RemoteAddr())
	currentSite.Cache.Delete(keyPrefix + admin.UserName)
	// 更新token
	admin.Token = currentSite.GetAdminAuthToken(admin.Id, req.Remember)
	admin.IsSuper = currentSite.Id == 1 && admin.GroupId == 1
	// 记录用户登录时间
	currentSite.DB.Model(admin).UpdateColumn("login_time", time.Now().Unix())

	// 密码成本因子升级：旧密码使用低成本因子时自动升级
	cost, _ := bcrypt.Cost([]byte(admin.Password))
	if cost < model.BcryptCost {
		admin.EncryptPassword(req.Password)
		currentSite.DB.Model(admin).UpdateColumn("password", admin.Password)
	}

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

// AdminLogout 管理员退出登录，直接返回退出成功。
func AdminLogout(ctx iris.Context) {
	// todo
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LoggedOut"),
	})
}

// AdminList 分页获取管理员列表，支持按 ID、用户组、用户名筛选。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "id": 管理员 ID，精确匹配。
//   - 查询参数 "group_id": 用户组 ID。
//   - 查询参数 "user_name": 用户名，模糊匹配。
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

// AdminDetail 获取指定管理员的详细信息，未指定 ID 时返回当前登录管理员的信息。
//
// 参数说明：
//   - 查询参数 "id": 管理员 ID，默认为 0 表示当前登录管理员。
func AdminDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	adminId := ctx.Values().GetUintDefault("adminId", 0)
	// Admin ID, empty means current admin
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
	admin.IsSuper = currentSite.Id == 1 && admin.GroupId == 1

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": admin,
	})
}

// AdminDetailForm 新增或更新管理员信息，包括用户名、密码、状态和所属用户组。
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

// AdminDetailDelete 删除指定管理员，不能删除超级管理员和自己。
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

// GetAdminLoginLog 分页获取管理员登录日志列表。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
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

// GetAdminLog 分页获取管理员操作日志列表。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
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

// AdminGroupList 获取全部管理员分组列表。
func AdminGroupList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	groups := currentSite.GetAdminGroups()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": groups,
	})
}

// AdminGroupDetail 根据指定 ID 获取管理员分组详情。
//
// 参数说明：
//   - 查询参数 "id": 分组 ID。
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

// AdminGroupDetailForm 新增或更新管理员分组，包括名称、描述、权限和配置。
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

// AdminGroupDelete 删除指定管理员分组。
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

// AdminMenus 获取后台操作按钮列表
func AdminMenus(ctx iris.Context) {

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": config.DefaultMenuGroups,
	})
}

// FindPasswordChooseWay 选择管理员找回密码的验证方式（文件上传验证或 DNS 解析验证），生成限时验证令牌。
//
// 参数说明：
//   - 请求体 "way": 验证方式，"file" 为文件验证，"dns" 为域名解析验证。
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
		w2 := provider.GetWebsite(currentSite.Id)
		w2.FindPasswordInfo = &response.FindPasswordInfo{
			Token: library.Md5(currentSite.TokenSecret + strconv.FormatInt(time.Now().UnixNano(), 10)),
		}
		currentSite.FindPasswordInfo = w2.FindPasswordInfo
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

// FindPasswordVerify 校验找回密码的验证结果：文件方式检查站点根目录下令牌文件内容，DNS 方式检查 TXT 解析记录是否与令牌一致。
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

// FindPasswordReset 找回密码验证通过后，重置超级管理员的账号和密码。
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
			Id: 1,
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
