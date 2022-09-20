package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
	"time"
)

func ApiLogin(ctx iris.Context) {
	var req request.ApiLoginRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		body, _ := ctx.GetBody()
		library.DebugLog("error", err.Error(), string(body))
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var user *model.User

	if req.Platform == "tt" {
		//头条的登录逻辑
		// todo
	} else if req.Platform == "swan" {
		//百度的登录逻辑
		//todo
	} else if req.Platform == "alipay" {
		//支付宝的登录逻辑
		// todo
	} else if req.Platform == "qq" {
		//QQ的登录逻辑
		// todo
	} else if req.Platform == "weapp" {
		//wechat  login
		user, err = provider.LoginByWeixin(&req)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		// login via user_name/email/cellphone and password
		if config.JsonData.Safe.Captcha == 1 {
			// 验证 captcha
			if req.CaptchaId == "" {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  "图形码不正确",
				})
				return
			}
			if ok := Store.Verify(req.CaptchaId, req.Captcha, true); !ok {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  "图形码不正确",
				})
				return
			}
		}
		req.UserName = strings.TrimSpace(req.UserName)
		req.Password = strings.TrimSpace(req.Password)

		if req.UserName == "" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "请输入账号",
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

		//开始登录用户
		user, err = provider.LoginByPassword(&req)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "登录失败",
			})
			return
		}
	}

	if user == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "登录失败",
		})
		return
	}

	// set token to cookie
	t := iris.CookieExpires(24 * time.Hour)
	// 记住会记住30天
	if req.Remember {
		t = iris.CookieExpires(30 * 24 * time.Hour)
	}
	ctx.SetCookieKV("token", user.Token, iris.CookiePath("/"), t)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": user,
	})
}

func ApiGetUserDetail(ctx iris.Context) {
	userId := ctx.Values().GetUintDefault("userId", 0)

	user, err := provider.GetUserInfoById(userId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  nil,
		"data": user,
	})
}
