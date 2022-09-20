package request

import "kandaoni.com/anqicms/model"

type UserRequest struct {
	Id         uint   `json:"id"`
	UserName   string `json:"user_name"`
	RealName   string `json:"real_name"`
	AvatarURL  string `json:"avatar_url"`
	Phone      string `json:"phone"`
	GroupId    uint   `json:"group_id"`
	Status     int    `json:"status"`
	Balance    int64  `json:"balance"`
	IsRetailer int    `json:"is_retailer"`
}

type UserGroupRequest struct {
	Id          uint                   `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Level       int                    `json:"level"`
	Status      int                    `json:"status"`
	Setting     model.UserGroupSetting `json:"setting"` //配置
}

type ApiLoginRequest struct {
	InviteId      uint   `json:"invite_id"` // 邀请用户ID
	Code          string `json:"code"`      //微信临时凭证
	AnonymousCode string `json:"anonymousCode"`
	Platform      string `json:"platform"`
	Avatar        string `json:"avatar"`
	NickName      string `json:"nick_name"`
	Gender        uint   `json:"gender"`
	Province      string `json:"province"`
	City          string `json:"city"`
	County        string `json:"county"`
	EncryptedData string `json:"encryptedData"`
	Iv            string `json:"iv"`
	Signature     string `json:"signature"`
	RawData       string `json:"rawData"`

	Remember  bool   `json:"remember"` // keep login state
	UserName  string `json:"user_name"`
	Password  string `json:"password"`
	CaptchaId string `json:"captcha_id"`
	Captcha   string `json:"captcha"`
}

type PluginRetailerConfig struct {
	GoodsPrice     int64 `json:"goods_price"`
	ShareReward    int64 `json:"share_reward"`    // 分销佣金比例
	ParentReward   int64 `json:"parent_reward"`   // 邀请奖励比例
	AllowSelf      int64 `json:"allow_self"`      // 允许自购 0,1
	BecomeRetailer int64 `json:"become_retailer"` // 成为分销员方式， 0 审核，1 自动
}
