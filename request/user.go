package request

import (
	"kandaoni.com/anqicms/model"
)

type UserRequest struct {
	Id         uint   `json:"id"`         // 用户 ID
	UserName   string `json:"user_name"`  // 用户名
	RealName   string `json:"real_name"`  // 真实姓名
	FirstName  string `json:"first_name"` // First Name
	LastName   string `json:"last_name"`  // Last Name
	Birthday   int64  `json:"birthday"`   // 生日（时间戳）
	AvatarURL  string `json:"avatar_url"` // 用户头像地址
	Introduce  string `json:"introduce"`  // 用户简介
	Phone      string `json:"phone"`      // 手机号
	Email      string `json:"email"`      // 邮箱
	GroupId    uint   `json:"group_id"`   // 用户组 ID
	Status     int    `json:"status"`     // 用户状态：0=待审核，1=正常，-1=禁用
	Balance    int64  `json:"balance" ast:"-"`
	IsRetailer int    `json:"is_retailer"` // 是否是分销员
	ParentId   uint   `json:"parent_id"`   // 上级用户 ID
	Password   string `json:"password"`    // 用户密码，需更新密码时传递
	InviteCode string `json:"invite_code"` // 邀请码
	ExpireTime int64  `json:"expire_time"` // 用户组过期时间（一般用于VIP过期）
	UpdateAll  bool   `json:"update_all" ast:"-"`

	Extra map[string]interface{} `json:"extra"` // 用户的扩展字段内容
}

type UserDeleteRequest struct {
	Id uint `json:"id"` // 用户 ID
}

type RetailerRequest struct {
	Id       uint   `json:"id"`        // 用户 ID
	RealName string `json:"real_name"` // 真实姓名
}

type RetailerApplyRequest struct {
	Id         uint `json:"id"`          // 用户 ID
	IsRetailer int  `json:"is_retailer"` // 是否成为分销员：0=否，1=是
}

type UserPasswordRequest struct {
	OldPassword string `json:"old_password"`
	Password    string `json:"password"`

	Email string `json:"email"`
	Token string `json:"token"`
	Code  string `json:"code"`
}

type UserGroupRequest struct {
	Id          uint                   `json:"id"`          // 用户组 ID
	Title       string                 `json:"title"`       // 用户组名称
	Description string                 `json:"description"` // 用户组介绍
	Level       int                    `json:"level"`       // group level：0,1,2,3...
	Price       int64                  `json:"price"`       // 用户组售价，单位：分，用于VIP
	Status      int                    `json:"status"`      // 用户组状态
	Setting     model.UserGroupSetting `json:"setting"`     //用户组配置
}

type UserGroupDeleteRequest struct {
	Id uint `json:"id"` // 用户组 ID
}

type ApiRegisterRequest struct {
	InviteId      uint   `json:"invite_id"` // 邀请用户ID
	UserName      string `json:"user_name"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Password      string `json:"password"`
	RealName      string `json:"real_name"`
	AvatarURL     string `json:"avatar_url"`
	Birthday      int64  `json:"birthday"`
	Introduce     string `json:"introduce"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	CaptchaId     string `json:"captcha_id"`
	Captcha       string `json:"captcha"`
	Code          string `json:"code"`  //phone verify code
	State         string `json:"state"` // 注册状态
	ResetPassword bool   `json:"reset_password"`

	Extra map[string]interface{} `json:"extra"`
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
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Password  string `json:"password"`
	CaptchaId string `json:"captcha_id"`
	Captcha   string `json:"captcha"`
}

type ApiUserBalanceRequest struct {
	UserId uint   `json:"user_id"` // 用户 ID
	Amount int64  `json:"amount"`  // 金额，单位：分
	Remark string `json:"remark"`  // 理由备注
}
