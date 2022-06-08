package request

type AdminInfoRequest struct {
	Id        int    `json:"id"`
	UserName  string `json:"user_name"`
	Password  string `json:"password"`
	CaptchaId string `json:"captcha_id"`
	Captcha   string `json:"captcha"`
}

type ChangeAdmin struct {
	UserName    string `json:"user_name" validate:"required"`
	OldPassword string `json:"old_password"`
	Password    string `json:"password"`
	RePassword  string `json:"re_password"`
}
