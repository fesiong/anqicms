package request

// ApiSmsSendRequest 是 POST /api/sms/send 的请求体（规格 §7.1）。
// json tag 逐字取自规格 §7.1 :708-714 参数表。
type ApiSmsSendRequest struct {
	Scene        string `json:"scene"`         // login / register / reset / bind_old / bind_new
	PhoneCountry string `json:"phone_country"` // 区号，纯数字，默认 86
	Phone        string `json:"phone"`         // 本地号码
	CaptchaId    string `json:"captcha_id"`    // Safe.Captcha == 1 时必填
	Captcha      string `json:"captcha"`       // 同上
}

// ApiSmsPasswordResetRequest 是 POST /api/sms/password/reset 的请求体（规格 §7.1）。
// json tag 逐字取自规格 §7.1 :742-749 参数表。
type ApiSmsPasswordResetRequest struct {
	PhoneCountry string `json:"phone_country"` // 区号，默认 86
	Phone        string `json:"phone"`         // 本地号码
	Code         string `json:"code"`          // 短信验证码（scene = reset）
	Password     string `json:"password"`      // 新密码，≥ 6 位
}

// ApiSmsPhoneBindRequest 是 POST /api/sms/phone/bind 的请求体（规格 §7.1）。
// json tag 逐字取自规格 §7.1 :755-764 参数表。
type ApiSmsPhoneBindRequest struct {
	OldCode         string `json:"old_code"`          // 旧号验证码（scene = bind_old）
	NewPhoneCountry string `json:"new_phone_country"` // 区号，默认 86
	NewPhone        string `json:"new_phone"`         // 新号码
	NewCode         string `json:"new_code"`          // 新号验证码（scene = bind_new）
}
