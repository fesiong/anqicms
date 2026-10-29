package request

import (
	"kandaoni.com/anqicms/model"
)

// UserRequest 用户保存请求。
//
// 字段用指针表达「未传」与「显式传了零值」的区别，理由与 request.Archive 相同：
// Go 的零值让 `status=0`（待审核）与 `status=-1`（禁用）无法与「没这个字段」区分，
// 而 SaveUser 又靠「零值=没传」判断要不要写库，结果就是这两个状态永远写不进去。
//
// 语义约定：
//   - 指针为 nil      → 未传，保持库里的原值（PATCH 语义）
//   - 指针非 nil      → 显式写入，含 0 / false / 空串（可用于清空或置零）
//   - UpdateAll=true  → 未传的指针按零值处理（表单全量覆盖语义）
//
// 例外（保留值类型，破坏这些语义就是 bug）：
//   - Id / UpdateAll / Partial：控制位，不是业务字段。
//   - UserName：新建时必填（空名会让「按用户名登录」串号），空串只有「没传」一种解释。
//   - Password：空串的语义是「不改密码」，绝不能解释成「把密码清空」。
//   - Extra：map，nil 已能表达「未传」。
type UserRequest struct {
	Id         uint    `json:"id"`              // 用户 ID
	UserName   string  `json:"user_name"`       // 用户名
	RealName   *string `json:"real_name"`       // 真实姓名
	FirstName  *string `json:"first_name"`      // First Name
	LastName   *string `json:"last_name"`       // Last Name
	Birthday   *int64  `json:"birthday"`        // 生日（时间戳）
	AvatarURL  *string `json:"avatar_url"`      // 用户头像地址
	Introduce  *string `json:"introduce"`       // 用户简介
	Phone      *string `json:"phone"`           // 手机号
	Email      *string `json:"email"`           // 邮箱
	GroupId    *uint   `json:"group_id"`        // 用户组 ID
	Status     *int    `json:"status"`          // 用户状态：0=待审核，1=正常，-1=禁用
	Balance    *int64  `json:"balance" ast:"-"` // 余额不在这里写，走 PluginUserChangeBalance，保留字段仅为兼容旧请求体
	IsRetailer *int    `json:"is_retailer"`     // 是否是分销员
	ParentId   *uint   `json:"parent_id"`       // 上级用户 ID
	Password   string  `json:"password"`        // 用户密码，需更新密码时传递
	InviteCode *string `json:"invite_code"`     // 邀请码
	ExpireTime *int64  `json:"expire_time"`     // 用户组过期时间（一般用于VIP过期）
	UpdateAll  bool    `json:"update_all" ast:"-"`
	// Partial 走 PATCH 语义：只覆盖显式传入的字段，未传的一律保持原值。
	// 与 UpdateAll 互为反向开关，且优先——控制器用 req.UpdateAll = !req.Partial 归一。
	Partial bool `json:"partial" ast:"-"`

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

// UserGroupRequest 用户组保存请求。
//
// 指针语义同 UserRequest：Status 的 0 是合法业务值（停用），Level 的 0 也是（最低等级），
// 用值类型就永远写不进去——SaveUserGroupInfo 原本甚至硬编码 Status: 1，
// 等于用户组一旦创建就无法停用。
type UserGroupRequest struct {
	Id          uint                    `json:"id"`                 // 用户组 ID
	Title       *string                 `json:"title"`              // 用户组名称
	Description *string                 `json:"description"`        // 用户组介绍
	Level       *int                    `json:"level"`              // group level：0,1,2,3...
	Price       *int64                  `json:"price"`              // 用户组售价，单位：分，用于VIP
	Status      *int                    `json:"status"`             // 用户组状态：0=停用，1=启用
	Setting     *model.UserGroupSetting `json:"setting"`            //用户组配置
	UpdateAll   bool                    `json:"update_all" ast:"-"` // 全量覆盖：未传字段按零值写入
	Partial     bool                    `json:"partial" ast:"-"`    // PATCH：只写显式传入的字段
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
