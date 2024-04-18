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
			"msg":  "无效的验证方式",
		})
		return
	}
	var host = ""
	if req.Way == config.PasswordFindWayDNS {
		parsed, err := url.Parse(currentSite.System.BaseUrl)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "域名解析失败",
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
			"msg":  "验证已失效",
		})
		return
	}

	if currentSite.FindPasswordInfo.Way == config.PasswordFindWayFile {
		filePath := currentSite.PublicPath + currentSite.FindPasswordInfo.Token + ".txt"
		buf, err := os.ReadFile(filePath)

		if err != nil || strings.TrimSpace(string(buf)) != currentSite.FindPasswordInfo.Token {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "文件不存在或内容不正确",
			})
			return
		}
	} else {
		txt, err := net.LookupTXT(currentSite.FindPasswordInfo.Host)
		if err != nil || len(txt) == 0 || txt[0] != currentSite.FindPasswordInfo.Token {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "DNS解析不存在或内容不正确",
			})
			return
		}
	}
	currentSite.FindPasswordInfo.Verified = true

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "验证成功",
		"data": currentSite.FindPasswordInfo,
	})
}

func FindPasswordReset(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if currentSite.FindPasswordInfo == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "验证已失效",
		})
		return
	}
	if !currentSite.FindPasswordInfo.Verified {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "权限验证失败",
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
			"msg":  "请填写管理员账号和6位以上的密码",
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
			"msg":  "密码设置失败",
		})
		return
	}
	err = currentSite.DB.Save(admin).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "更新信息出错",
		})
		return
	}
	currentSite.FindPasswordInfo.Timer.Stop()
	currentSite.FindPasswordInfo = nil

	currentSite.AddAdminLog(ctx, fmt.Sprintf("重置管理员账号和密码：%d => %s", admin.Id, admin.UserName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "管理员账号和密码已重置",
	})
}
