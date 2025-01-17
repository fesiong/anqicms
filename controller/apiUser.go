package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strconv"
	"strings"
	"time"
)

func ApiRegister(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ApiRegisterRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	tmpInvite := ctx.GetCookie("invite")
	if tmpInvite != "" {
		tmpId, _ := strconv.Atoi(tmpInvite)
		req.InviteId = uint(tmpId)
	}
	req.UserName = strings.TrimSpace(req.UserName)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)
	user, err := currentSite.RegisterUser(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// set token to cookie
	t := iris.CookieExpires(24 * time.Hour)
	ctx.SetCookieKV("token", user.Token, iris.CookiePath("/"), t)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": user,
	})
}

func ApiLogin(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ApiLoginRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	tmpInvite := ctx.GetCookie("invite")
	if tmpInvite != "" {
		tmpId, _ := strconv.Atoi(tmpInvite)
		req.InviteId = uint(tmpId)
	}

	var user *model.User

	if req.Platform == config.PlatformTT {
		//头条的登录逻辑
		// todo
	} else if req.Platform == config.PlatformSwan {
		//百度的登录逻辑
		//todo
	} else if req.Platform == config.PlatformAlipay {
		//支付宝的登录逻辑
		// todo
	} else if req.Platform == config.PlatformQQ {
		//QQ的登录逻辑
		// todo
	} else if req.Platform == config.PlatformWeapp {
		//weapp  login
		user, err = currentSite.LoginViaWeapp(&req)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else if req.Platform == config.PlatformWechat {
		// WeChat official account login
		user, err = currentSite.LoginViaWechat(&req)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		// login via user_name/email/cellphone and password
		if currentSite.Safe.Captcha == 1 {
			// 验证 captcha
			if req.CaptchaId == "" {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  currentSite.TplTr("GraphicCodeIncorrect"),
				})
				return
			}
			if ok := Store.Verify(req.CaptchaId, req.Captcha, true); !ok {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  currentSite.TplTr("GraphicCodeIncorrect"),
				})
				return
			}
		}
		req.UserName = strings.TrimSpace(req.UserName)
		req.Password = strings.TrimSpace(req.Password)

		if req.UserName == "" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("PleaseEnterAccount"),
			})
			return
		}
		//验证密码
		if len(req.Password) < 6 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("PleaseEnterAPasswordOf6CharactersOrMore"),
			})
			return
		}

		//开始登录用户
		user, err = currentSite.LoginViaPassword(&req)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("LoginFailed"),
			})
			return
		}
	}

	if user == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("LoginFailed"),
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
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)

	user, err := currentSite.GetUserInfoById(userId)
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

func ApiUpdateUserDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req.UserName = strings.TrimSpace(req.UserName)
	req.RealName = strings.TrimSpace(req.RealName)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.TrimSpace(req.Email)
	req.Introduce = strings.TrimSpace(req.Introduce)
	userId := ctx.Values().GetUintDefault("userId", 0)

	err := currentSite.UpdateUserInfo(userId, &req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("SaveSuccessfully"),
	})
}

func ApiUpdateUserAvatar(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)
	file, _, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()

	avatarUrl, err := currentSite.UploadUserAvatar(userId, file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("FileSaveFailed"),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("FileUploadCompleted"),
		"data": iris.Map{
			"avatar_url": avatarUrl,
		},
	})
}

func ApiGetUserGroups(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	groups := currentSite.GetUserGroups()
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId > 0 {
		userInfo, _ := ctx.Values().Get("userInfo").(*model.User)
		discount := currentSite.GetUserDiscount(userId, userInfo)
		for i := range groups {
			if groups[i].Price > 0 {
				if discount > 0 {
					groups[i].FavorablePrice = groups[i].Price * discount / 100
				}
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": groups,
	})
}

func ApiGetUserGroupDetail(ctx iris.Context) {
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
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId > 0 {
		userInfo, _ := ctx.Values().Get("userInfo").(*model.User)
		discount := currentSite.GetUserDiscount(userId, userInfo)
		if group.Price > 0 {
			if discount > 0 {
				group.FavorablePrice = group.Price * discount / 100
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": group,
	})
}

func ApiUpdateUserPassword(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserPasswordRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req.Password = strings.TrimSpace(req.Password)
	req.OldPassword = strings.TrimSpace(req.OldPassword)
	if len(req.Password) < 6 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("PleaseFillInAPasswordOfMoreThan6Digits"),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	user, err := currentSite.GetUserInfoById(userId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("PleaseLogIn"),
		})
		return
	}

	// 如果初次设置密码，则不需要检查
	if user.Password != "" {
		if !user.CheckPassword(req.OldPassword) {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("OldPasswordIsWrong"),
			})
			return
		}
	}
	err = user.EncryptPassword(req.Password)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DB.Save(user)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("PasswordChangedSuccessfully"),
	})
}
