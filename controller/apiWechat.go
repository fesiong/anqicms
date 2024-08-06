package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"net/url"
	"strings"
)

func WechatApi(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	wechatServer := currentSite.GetWechatServer(false)
	resp := wechatServer.VerifyURL(ctx.ResponseWriter(), ctx.Request())

	if ctx.Method() != "GET" {
		currentSite.ResponseWechatMsg(resp)
	}
}

func WechatAuthApi(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	s := strings.ToLower(ctx.GetHeader("User-Agent"))
	if !strings.Contains(s, "micromessenger") {
		// not in weChat browser
		ShowMessage(ctx, currentSite.TplTr("PleaseOpenInWechat"), nil)
		return
	}
	state := ctx.URLParam("state")
	code := ctx.URLParam("code")
	if code != "" {
		accessToken, err := currentSite.GetAccessTokenByCode(code)
		if err != nil {
			ShowMessage(ctx, err.Error(), nil)
			return
		}
		if accessToken.Errmsg != "" {
			ShowMessage(ctx, accessToken.Errmsg, nil)
			return
		}

		// 再换取用户信息
		mpUserInfo, err := currentSite.GetSNSUserInfo(accessToken.AccessToken, accessToken.Openid)
		if err != nil {
			if err != nil {
				ShowMessage(ctx, err.Error(), nil)
				return
			}
		}
		userWechat, err := currentSite.GetUserWechatByOpenid(mpUserInfo.OpenId)
		if err != nil {
			// register user if user is not in the database.
			userWechat = &model.UserWechat{
				Nickname:  mpUserInfo.NickName,
				AvatarURL: mpUserInfo.HeadImgUrl,
				Gender:    mpUserInfo.Sex,
				Openid:    mpUserInfo.OpenId,
				UnionId:   mpUserInfo.UnionId,
				Platform:  config.PlatformWechat,
				Status:    1,
			}
			var tmpUser *model.User
			if userWechat.UnionId != "" {
				tmpUser, _ = currentSite.GetUserByUnionId(userWechat.UnionId)
			}
			if tmpUser == nil {
				tmpUser = &model.User{
					UserName:  userWechat.Nickname,
					AvatarURL: userWechat.AvatarURL,
					GroupId:   currentSite.PluginUser.DefaultGroupId,
					Password:  "",
					Status:    1,
				}
				currentSite.DB.Save(tmpUser)
			}
			userWechat.UserId = tmpUser.Id
			currentSite.DB.Save(userWechat)

			go currentSite.DownloadAvatar(tmpUser.AvatarURL, tmpUser)
		}
		if state == "code" {
			verifyMsg := currentSite.PluginWechat.VerifyMsg
			if !strings.Contains(verifyMsg, "{code}") {
				verifyMsg = currentSite.TplTr("VerificationCodeValidFor30Minutes") + verifyMsg
			}
			verifyCode := library.CodeCache.Generate(userWechat.Openid)
			verifyMsg = strings.Replace(verifyMsg, "{code}", verifyCode, 1)
			currentSite.GetWechatServer(false).SendText(userWechat.Openid, verifyMsg)
			ShowMessage(ctx, verifyMsg, nil)
		}

		return
	}

	redirectUri := strings.TrimRight(currentSite.System.BaseUrl, "/") + "/api/wechat/auth"
	ctx.Redirect("https://open.weixin.qq.com/connect/oauth2/authorize?appid=" + currentSite.PluginWechat.AppID + "&redirect_uri=" + url.PathEscape(redirectUri) + "&response_type=code&scope=snsapi_userinfo&state=" + state + "#wechat_redirect")
}
