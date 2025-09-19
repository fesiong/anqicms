package provider

import (
	"errors"
	"fmt"
	"github.com/esap/wechat"
	"github.com/esap/wechat/util"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"strings"
	"time"
)

func (w *Website) GetWechatServer(focus bool) *wechat.Server {
	cfg := &wechat.WxConfig{
		Token:          w.PluginWechat.Token,
		AppId:          w.PluginWechat.AppID,
		Secret:         w.PluginWechat.AppSecret,
		EncodingAESKey: w.PluginWechat.EncodingAESKey,
		AppType:        0,
		DataFormat:     "XML",
	}

	if w.wechatServer == nil || focus {
		w.wechatServer = wechat.New(cfg)
		w2 := GetWebsite(w.Id)
		w2.wechatServer = w.wechatServer
	}

	return w.wechatServer
}

func (w *Website) ResponseWechatMsg(ctx *wechat.Context) {
	verifyKey := w.PluginWechat.VerifyKey
	if verifyKey == "" {
		verifyKey = "验证码"
	}
	var content = ctx.Msg.Content
	// 只有点击事件需要处理
	if ctx.Msg.MsgType == "event" {
		if ctx.Msg.Event == "CLICK" {
			content = ctx.Msg.EventKey
		} else {
			ctx.Writer.Write([]byte("success"))
			return
		}
	}
	message := model.WechatMessage{
		Openid:    ctx.Msg.FromUserName,
		Content:   content,
		Reply:     "",
		ReplyTime: time.Now().Unix(),
	}

	// 获取验证码
	if strings.EqualFold(content, verifyKey) {
		userWechat, err := w.GetUserWechatByOpenid(ctx.Msg.FromUserName)
		if err != nil {
			if err != nil {
				redirectUri := strings.TrimRight(w.System.BaseUrl, "/") + "/api/wechat/auth?state=code"
				txt := "欢迎您，<a href=\"" + redirectUri + "\">请点击此处完成授权</a>"
				message.Reply = txt
				w.DB.Save(&message)
				ctx.NewText(txt).Reply()
				return
			}
		}
		verifyMsg := w.PluginWechat.VerifyMsg
		if !strings.Contains(verifyMsg, "{code}") {
			verifyMsg = "验证码：{code}，30分钟内有效" + verifyMsg
		}
		code := library.CodeCache.Generate(userWechat.Openid)
		verifyMsg = strings.Replace(verifyMsg, "{code}", code, 1)
		message.Reply = verifyMsg
		w.DB.Save(&message)
		ctx.NewText(verifyMsg).Reply()
		return
	}
	// 自定义关键词回复
	rule, err := w.GetWechatReplyRuleByKeyword(content)
	if err == nil {
		message.Reply = rule.Content
		w.DB.Save(&message)
		ctx.NewText(rule.Content).Reply()
		return
	}
	// 未回复内容
	message.ReplyTime = 0
	w.DB.Save(&message)

	ctx.Writer.Write([]byte("success"))
}

type AuthAccessToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Openid       string `json:"openid"`
	Scope        string `json:"scope"`
	Errorcode    int64  `json:"errorcode"`
	Errmsg       string `json:"errmsg"`
}

// GetAccessTokenByCode 通过 code 换取网页授权access_token
func (w *Website) GetAccessTokenByCode(code string) (res *AuthAccessToken, err error) {
	res = new(AuthAccessToken)
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code", w.PluginWechat.AppID, w.PluginWechat.AppSecret, code)
	if err = util.GetJson(url, &res); err != nil {
		return
	}
	return
}

// GetSNSUserInfo 拉取用户信息(需 scope 为 snsapi_userinfo)
func (w *Website) GetSNSUserInfo(accessToken, openid string) (user *wechat.MpUserInfo, err error) {
	user = new(wechat.MpUserInfo)
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s&lang=zh_CN", accessToken, openid)
	if err = util.GetJson(url, &user); err != nil {
		return
	}
	return
}

func (w *Website) GetWechatMessage(id uint) (*model.WechatMessage, error) {
	var message model.WechatMessage
	err := w.DB.Where("`id` = ?", id).Take(&message).Error
	if err != nil {
		return nil, err
	}

	return &message, nil
}

func (w *Website) GetWechatMessages(page, pageSize int) ([]*model.WechatMessage, int64) {
	var messages []*model.WechatMessage
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.WechatMessage{}).Order("id desc")
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&messages)

	return messages, total
}

func (w *Website) DeleteWechatMessage(id uint) error {
	message, err := w.GetWechatMessage(id)
	if err != nil {
		return err
	}

	err = w.DB.Unscoped().Delete(message).Error

	return err
}

func (w *Website) ReplyWechatMessage(req *request.WechatMessageRequest) error {
	message, err := w.GetWechatMessage(req.Id)
	if err != nil {
		return err
	}

	if req.Reply == "" {
		return errors.New(w.Tr("PleaseFillInTheReplyContent"))
	}

	err2 := w.GetWechatServer(false).SendText(message.Openid, req.Reply)
	if err2.ErrCode == -1 {
		// error
		return errors.New(err2.ErrMsg)
	}
	message.Reply = req.Reply
	message.ReplyTime = time.Now().Unix()

	return nil
}

func (w *Website) GetWechatReplyRules(page, pageSize int) ([]*model.WechatReplyRule, int64) {
	var rules []*model.WechatReplyRule
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.WechatReplyRule{}).Order("id desc")
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&rules)

	return rules, total
}

func (w *Website) GetWechatReplyRuleByKeyword(keyword string) (*model.WechatReplyRule, error) {
	var rule model.WechatReplyRule
	err := w.DB.Where("`keyword` = ?", keyword).Take(&rule).Error
	if err != nil {
		// 尝试获取 default
		err = w.DB.Where("`is_default` = 1").Take(&rule).Error
		if err != nil {
			return nil, err
		}
	}

	return &rule, nil
}

func (w *Website) DeleteWechatReplyRule(id uint) error {
	var rule model.WechatReplyRule
	err := w.DB.Where("`id` = ?", id).Take(&rule).Error
	if err != nil {
		return err
	}

	err = w.DB.Unscoped().Delete(&rule).Error

	return err
}

func (w *Website) SaveWechatReplyRule(req *request.WechatReplyRuleRequest) error {
	var rule model.WechatReplyRule
	if req.Id > 0 {
		err := w.DB.Where("`id` = ?", req.Id).Take(&rule).Error
		if err != nil {
			return err
		}
	}
	rule.Keyword = req.Keyword
	rule.Content = req.Content
	rule.IsDefault = req.IsDefault
	err := w.DB.Save(&rule).Error
	if err != nil {
		return err
	}
	if rule.IsDefault == 1 {
		w.DB.Model(&model.WechatReplyRule{}).Where("`id` != ?", rule.Id).UpdateColumn("is_default", 0)
	}
	return nil
}

func (w *Website) GetWechatMenus() []*model.WechatMenu {
	var tmpMenus []*model.WechatMenu
	w.DB.Order("sort asc").Find(&tmpMenus)
	var menus []*model.WechatMenu
	for i := range tmpMenus {
		if tmpMenus[i].ParentId == 0 {
			for j := range tmpMenus {
				if tmpMenus[j].ParentId == tmpMenus[i].Id {
					tmpMenus[i].Children = append(tmpMenus[i].Children, tmpMenus[j])
				}
			}
			menus = append(menus, tmpMenus[i])
		}
	}

	return menus
}

func (w *Website) DeleteWechatMenu(id uint) error {
	var menu model.WechatMenu
	err := w.DB.Where("`id` = ?", id).Take(&menu).Error
	if err != nil {
		return err
	}

	w.DB.Unscoped().Where("`parent_id` = ?", menu.Id).Delete(model.WechatMenu{})
	err = w.DB.Unscoped().Delete(&menu).Error

	return err
}

func (w *Website) SaveWechatMenu(req *request.WechatMenuRequest) error {
	var menu model.WechatMenu
	if req.Id > 0 {
		err := w.DB.Where("`id` = ?", req.Id).Take(&menu).Error
		if err != nil {
			return err
		}
	}
	menu.Name = req.Name
	menu.Type = req.Type
	menu.Value = req.Value
	menu.Sort = req.Sort
	menu.ParentId = req.ParentId

	err := w.DB.Save(&menu).Error

	return err
}

func (w *Website) SyncWechatMenu() error {
	menus := w.GetWechatMenus()
	postMenu := new(wechat.Menu)

	for i, v := range menus {
		button := wechat.Button{
			Name: v.Name,
			Type: v.Type,
		}
		if v.Type == "view" {
			button.Url = v.Value
		} else {
			button.Key = v.Value
		}
		if len(v.Children) > 0 {
			for j, v2 := range v.Children {
				subButton := struct {
					Name     string `json:"name"`
					Type     string `json:"type"`
					Key      string `json:"key"`
					Url      string `json:"url"`
					AppId    string `json:"appid"`
					PagePath string `json:"pagepath"`
				}{
					Name: v2.Name,
					Type: v2.Type,
				}
				if v2.Type == "view" {
					subButton.Url = v2.Value
				} else {
					subButton.Key = v2.Value
				}
				button.SubButton = append(button.SubButton, subButton)
				if j >= 5 {
					break
				}
			}
		}
		postMenu.Button = append(postMenu.Button, button)
		if i >= 3 {
			break
		}
	}
	err := w.GetWechatServer(false).AddMenu(postMenu)
	return err
}
