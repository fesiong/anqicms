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
