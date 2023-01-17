package provider

import (
	"errors"
	"fmt"
	"github.com/esap/wechat"
	"github.com/esap/wechat/util"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"strings"
	"time"
)

var wechatServer *wechat.Server

func GetWechatServer(focus bool) *wechat.Server {
	cfg := &wechat.WxConfig{
		Token:          config.JsonData.PluginWechat.Token,
		AppId:          config.JsonData.PluginWechat.AppID,
		Secret:         config.JsonData.PluginWechat.AppSecret,
		EncodingAESKey: config.JsonData.PluginWechat.EncodingAESKey,
		AppType:        0,
		DateFormat:     "XML",
	}

	if wechatServer == nil || focus {
		wechatServer = wechat.New(cfg)
	}

	return wechatServer
}

func ResponseWechatMsg(ctx *wechat.Context) {
	verifyKey := config.JsonData.PluginWechat.VerifyKey
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
		userWechat, err := GetUserWechatByOpenid(ctx.Msg.FromUserName)
		if err != nil {
			if err != nil {
				redirectUri := strings.TrimRight(config.JsonData.System.BaseUrl, "/") + "/api/wechat/auth?state=code"
				txt := "欢迎您，<a href=\"" + redirectUri + "\">请点击此处完成授权</a>"
				message.Reply = txt
				dao.DB.Save(&message)
				ctx.NewText(txt).Reply()
				return
			}
		}
		verifyMsg := config.JsonData.PluginWechat.VerifyMsg
		if !strings.Contains(verifyMsg, "{code}") {
			verifyMsg = "验证码：{code}，30分钟内有效" + verifyMsg
		}
		code := library.CodeCache.Generate(userWechat.Openid)
		verifyMsg = strings.Replace(verifyMsg, "{code}", code, 1)
		message.Reply = verifyMsg
		dao.DB.Save(&message)
		ctx.NewText(verifyMsg).Reply()
		return
	}
	// 自定义关键词回复
	rule, err := GetWechatReplyRuleByKeyword(content)
	if err == nil {
		message.Reply = rule.Content
		dao.DB.Save(&message)
		ctx.NewText(rule.Content).Reply()
		return
	}
	// 未回复内容
	message.ReplyTime = 0
	dao.DB.Save(&message)

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
func GetAccessTokenByCode(code string) (res *AuthAccessToken, err error) {
	res = new(AuthAccessToken)
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code", config.JsonData.PluginWechat.AppID, config.JsonData.PluginWechat.AppSecret, code)
	if err = util.GetJson(url, &res); err != nil {
		return
	}
	return
}

// GetSNSUserInfo 拉取用户信息(需 scope 为 snsapi_userinfo)
func GetSNSUserInfo(accessToken, openid string) (user *wechat.MpUserInfo, err error) {
	user = new(wechat.MpUserInfo)
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s&lang=zh_CN", accessToken, openid)
	if err = util.GetJson(url, &user); err != nil {
		return
	}
	return
}

func GetWechatMessage(id uint) (*model.WechatMessage, error) {
	var message model.WechatMessage
	err := dao.DB.Where("`id` = ?", id).Take(&message).Error
	if err != nil {
		return nil, err
	}

	return &message, nil
}

func GetWechatMessages(page, pageSize int) ([]*model.WechatMessage, int64) {
	var messages []*model.WechatMessage
	var total int64
	offset := (page - 1) * pageSize
	tx := dao.DB.Model(&model.WechatMessage{}).Order("id desc")
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&messages)

	return messages, total
}

func DeleteWechatMessage(id uint) error {
	message, err := GetWechatMessage(id)
	if err != nil {
		return err
	}

	err = dao.DB.Unscoped().Delete(message).Error

	return err
}

func ReplyWechatMessage(req *request.WechatMessageRequest) error {
	message, err := GetWechatMessage(req.Id)
	if err != nil {
		return err
	}

	if req.Reply == "" {
		return errors.New(config.Lang("请填写回复内容"))
	}

	err2 := GetWechatServer(false).SendText(message.Openid, req.Reply)
	if err2.ErrCode == -1 {
		// error
		return errors.New(err2.ErrMsg)
	}
	message.Reply = req.Reply
	message.ReplyTime = time.Now().Unix()

	return nil
}

func GetWechatReplyRules(page, pageSize int) ([]*model.WechatReplyRule, int64) {
	var rules []*model.WechatReplyRule
	var total int64
	offset := (page - 1) * pageSize
	tx := dao.DB.Model(&model.WechatReplyRule{}).Order("id desc")
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&rules)

	return rules, total
}

func GetWechatReplyRuleByKeyword(keyword string) (*model.WechatReplyRule, error) {
	var rule model.WechatReplyRule
	err := dao.DB.Where("`keyword` = ?", keyword).Take(&rule).Error
	if err != nil {
		// 尝试获取 default
		err = dao.DB.Where("`is_default` = 1").Take(&rule).Error
		if err != nil {
			return nil, err
		}
	}

	return &rule, nil
}

func DeleteWechatReplyRule(id uint) error {
	var rule model.WechatReplyRule
	err := dao.DB.Where("`id` = ?", id).Take(&rule).Error
	if err != nil {
		return err
	}

	err = dao.DB.Unscoped().Delete(&rule).Error

	return err
}

func SaveWechatReplyRule(req *request.WechatReplyRuleRequest) error {
	var rule model.WechatReplyRule
	if req.Id > 0 {
		err := dao.DB.Where("`id` = ?", req.Id).Take(&rule).Error
		if err != nil {
			return err
		}
	}
	rule.Keyword = req.Keyword
	rule.Content = req.Content
	rule.IsDefault = req.IsDefault
	err := dao.DB.Save(&rule).Error
	if err != nil {
		return err
	}
	if rule.IsDefault == 1 {
		dao.DB.Model(&model.WechatReplyRule{}).Where("`id` != ?", rule.Id).UpdateColumn("is_default", 0)
	}
	return nil
}

func GetWechatMenus() []*model.WechatMenu {
	var tmpMenus []*model.WechatMenu
	dao.DB.Order("sort asc").Find(&tmpMenus)
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

func DeleteWechatMenu(id uint) error {
	var menu model.WechatMenu
	err := dao.DB.Where("`id` = ?", id).Take(&menu).Error
	if err != nil {
		return err
	}

	dao.DB.Unscoped().Where("`parent_id` = ?", menu.Id).Delete(model.WechatMenu{})
	err = dao.DB.Unscoped().Delete(&menu).Error

	return err
}

func SaveWechatMenu(req *request.WechatMenuRequest) error {
	var menu model.WechatMenu
	if req.Id > 0 {
		err := dao.DB.Where("`id` = ?", req.Id).Take(&menu).Error
		if err != nil {
			return err
		}
	}
	menu.Name = req.Name
	menu.Type = req.Type
	menu.Value = req.Value
	menu.Sort = req.Sort
	menu.ParentId = req.ParentId

	err := dao.DB.Save(&menu).Error

	return err
}

func SyncWechatMenu() error {
	menus := GetWechatMenus()
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
	err := GetWechatServer(false).AddMenu(postMenu)
	return err
}
