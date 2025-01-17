package request

import (
	"kandaoni.com/anqicms/model"
)

type UserRequest struct {
	Id         uint   `json:"id"`
	UserName   string `json:"user_name"`
	RealName   string `json:"real_name"`
	AvatarURL  string `json:"avatar_url"`
	Introduce  string `json:"introduce"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	GroupId    uint   `json:"group_id"`
	Status     int    `json:"status"`
	Balance    int64  `json:"balance"`
	IsRetailer int    `json:"is_retailer"`
	ParentId   uint   `json:"parent_id"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
	ExpireTime int64  `json:"expire_time"`

	Extra map[string]interface{} `json:"extra"`
}

type UserPasswordRequest struct {
	OldPassword string `json:"old_password"`
	Password    string `json:"password"`
}

type UserGroupRequest struct {
	Id          uint                   `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Level       int                    `json:"level"` // group level
	Price       int64                  `json:"price"`
	Status      int                    `json:"status"`
	Setting     model.UserGroupSetting `json:"setting"` //配置
}

type ApiRegisterRequest struct {
	InviteId  uint   `json:"invite_id"` // 邀请用户ID
	UserName  string `json:"user_name"`
	Password  string `json:"password"`
	RealName  string `json:"real_name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	CaptchaId string `json:"captcha_id"`
	Captcha   string `json:"captcha"`
	Code      string `json:"code"` //phone verify code
}

type ApiLoginRequest struct {
	InviteId      uint   `json:"invite_id"` // 邀请用户ID
	Code          string `json:"code"`      //微信临时凭证,或者是验证码
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
