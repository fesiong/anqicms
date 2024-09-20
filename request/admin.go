package request

import "kandaoni.com/anqicms/model"

type AdminInfoRequest struct {
	Id          uint   `json:"id"`
	UserName    string `json:"user_name"`
	Password    string `json:"password"`
	CaptchaId   string `json:"captcha_id"`
	Captcha     string `json:"captcha"`
	Remember    bool   `json:"remember"`
	Status      uint   `json:"status"`
	GroupId     uint   `json:"group_id"`
	OldPassword string `json:"old_password"`
	RePassword  string `json:"re_password"`
}

type GroupRequest struct {
	Id          uint               `json:"id"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Status      int                `json:"status"`
	Setting     model.GroupSetting `json:"setting"` //配置
}

type FindPasswordChooseRequest struct {
	Way string `json:"way"`
}

type FindPasswordReset struct {
	UserName string `json:"user_name"`
	Password string `json:"password"`
}
