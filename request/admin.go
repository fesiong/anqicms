package request

import "kandaoni.com/anqicms/model"

type AdminInfoRequest struct {
	// Id 管理员 ID。
	Id          uint   `json:"id"`
	// UserName 登录用户名。
	UserName    string `json:"user_name"`
	// Password 登录密码（新增/修改时填写，接口不返回明文）。
	Password    string `json:"password"`
	// CaptchaId 验证码标识，配合 Captcha 使用。
	CaptchaId   string `json:"captcha_id"`
	// Captcha 图形验证码内容。
	Captcha     string `json:"captcha"`
	// Remember 是否记住登录状态（true=是）。
	Remember    bool   `json:"remember"`
	// Status 账户状态：0=禁用，1=启用。
	Status      uint   `json:"status"`
	// GroupId 所属管理员分组的 ID。
	GroupId     uint   `json:"group_id"`
	// OldPassword 修改密码时的旧密码。
	OldPassword string `json:"old_password"`
	// RePassword 修改密码时的确认密码。
	RePassword  string `json:"re_password"`
	// 站点切换登录参数，二者必须成对出现，且都只能来自服务端签发的跳转链接
	// Sign 跳转登录票据签名，由目标站点密钥对 Nonce 计算。
	Sign   string `json:"sign"`
	// SiteId 多站点时指定的站点 ID。
	SiteId uint   `json:"site_id"`
	// Nonce 服务端签发的短期有效随机串，使用一次即失效。
	Nonce  string `json:"nonce"`
}

type WebsiteLoginRequest struct {
	// SiteId 要登录（切换）到的子站点 ID。
	SiteId uint `json:"site_id"`
}

type GroupRequest struct {
	// Id 分组 ID（编辑时必填，新增时忽略）。
	Id          uint               `json:"id"`
	// Title 分组名称。
	Title       string             `json:"title"`
	// Description 分组描述。
	Description string             `json:"description"`
	// Status 分组状态：0=禁用，1=启用。
	Status      int                `json:"status"`
	Setting     model.GroupSetting `json:"setting"` //配置
}

type FindPasswordChooseRequest struct {
	Way string `json:"way"`
}

type FindPasswordReset struct {
	// UserName 管理员账号
	UserName string `json:"user_name"`
	// Password 新密码，服务端要求至少 6 位
	Password string `json:"password"`
}
