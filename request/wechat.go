package request

type WechatMessageRequest struct {
	Id        uint   `json:"id"`
	Openid    string `json:"openid"`
	Content   string `json:"content"`
	Reply     string `json:"reply"`
	ReplyTime int    `json:"reply_time"`
}

type WechatReplyRuleRequest struct {
	Id        uint   `json:"id"`
	Keyword   string `json:"keyword"`
	Content   string `json:"content"`
	IsDefault int    `json:"is_default"`
}

type WechatMenuRequest struct {
	Id       uint   `json:"id"`
	ParentId uint   `json:"parent_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Sort     uint   `json:"sort"`
}
