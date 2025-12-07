package controller

import (
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
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

	if currentSite.PluginSendmail.SignupVerify && user.EmailVerified == false {
		// 提示正在验证, 并且不登录
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("PleaseVerifyEmail"),
			"data": user,
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
	} else if req.Platform == config.PlatformGoogle {
		user, err = currentSite.LoginViaGoogle(&req)
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
		if req.Email != "" {
			req.UserName = req.Email
		} else if req.Phone != "" {
			req.UserName = req.Phone
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

func ApiSendVerifyEmail(ctx iris.Context) {
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
	if !currentSite.VerifyEmailFormat(req.Email) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("invalidParameter"),
		})
		return
	}

	user, err := currentSite.GetUserInfoByEmail(req.Email)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UserDoesNotExist"),
		})
	}
	_ = currentSite.SendVerifyEmail(user, req.State)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("PleaseVerifyEmail"),
	})
}

func ApiVerifyEmail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	token := ctx.URLParam("token")
	code := ctx.URLParam("code")
	email := ctx.URLParam("email")
	state := ctx.URLParam("state")
	returnType := ctx.URLParam("return")
	if !currentSite.VerifyEmailFormat(email) || (len(token) == 0 && state != "verify") {
		if returnType == "json" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("invalidParameter"),
			})
		} else {
			ShowMessage(ctx, currentSite.TplTr("invalidParameter"), []Button{{Name: currentSite.TplTr("Home"), Link: "/"}})
		}
		return
	}
	user, err := currentSite.GetUserInfoByEmail(email)
	if err != nil {
		if returnType == "json" || state == "verify" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("UserDoesNotExist"),
			})
		} else {
			ShowMessage(ctx, currentSite.TplTr("UserDoesNotExist"), []Button{{Name: currentSite.TplTr("Home"), Link: "/"}})
		}
		return
	}
	// 验证用户状态
	if state == "verify" {
		// 只能是JSON格式返回
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": iris.Map{
				"id":    user.Id,
				"email": user.Email,
			},
		})
		return
	}
	// 验证Token
	verifyCode := library.CodeCache.Get(token, true)
	if verifyCode != code {
		// 暂时不做验证
	}
	verifyToken := library.Md5(user.Email + user.Password)
	if verifyToken != token {
		if returnType == "json" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("InvalidToken"),
			})
		} else {
			ShowMessage(ctx, currentSite.TplTr("InvalidToken"), []Button{{Name: currentSite.TplTr("Home"), Link: "/"}})
		}
		return
	}
	if state == "reset" {
		// 重置密码
		// 跳到重置密码页面
		ctx.Redirect(currentSite.System.BaseUrl + "/account/password/reset?token=" + token + "&code=" + code + "&email=" + user.Email)
		return
	}
	// 验证通过
	user.EmailVerified = true
	currentSite.DB.Model(user).UpdateColumn("email_verified", true)
	// 生成登录Token
	user.Token = currentSite.GetUserAuthToken(user.Id, true)
	_ = user.LogLogin(currentSite.DB)
	// set token to cookie
	t := iris.CookieExpires(24 * time.Hour)
	ctx.SetCookieKV("token", user.Token, iris.CookiePath("/"), t)
	if returnType == "json" {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  currentSite.TplTr("verificationSuccessful"),
		})
	} else {
		ShowMessage(ctx, currentSite.TplTr("verificationSuccessful"), []Button{{Name: currentSite.TplTr("Home"), Link: "/"}})
	}
	return
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
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	if req.FirstName != "" {
		req.RealName = req.FirstName + " " + req.LastName
	}
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

func ApiResetUserPassword(ctx iris.Context) {
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
	if len(req.Password) < 6 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("PleaseFillInAPasswordOfMoreThan6Digits"),
		})
		return
	}
	if !currentSite.VerifyEmailFormat(req.Email) || len(req.Token) == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("invalidParameter"),
		})
		return
	}
	user, err := currentSite.GetUserInfoByEmail(req.Email)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UserDoesNotExist"),
		})
		return
	}
	// 验证Token
	verifyCode := library.CodeCache.Get(req.Token, true)
	if verifyCode != req.Code {
		// 暂时不做验证
	}
	verifyToken := library.Md5(user.Email + user.Password)
	if verifyToken != req.Token {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("InvalidToken"),
		})
		return
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
	// 不直接登录

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("PasswordChangedSuccessfully"),
	})
}
