package request

type WechatMessageRequest struct {
	Id        uint   `json:"id"`                 // 消息 ID
	Openid    string `json:"openid"`             // 用户的 openid
	Content   string `json:"content"`            // 用户发送的消息
	Reply     string `json:"reply"`              // 自动（后台）回复的消息
	ReplyTime int    `json:"reply_time" ast:"-"` // 回复时间
}

type WechatMessageReplyRequest struct {
	Id    uint   `json:"id"`    // 消息 ID
	Reply string `json:"reply"` // 自动（后台）回复的消息
}

type WechatMessageDeleteRequest struct {
	Id uint `json:"id"` // 消息 ID
}

type WechatReplyRuleRequest struct {
	Id        uint   `json:"id"`         // 规则 ID
	Keyword   string `json:"keyword"`    // 触发关键词
	Content   string `json:"content"`    // 回复内容
	IsDefault int    `json:"is_default"` // 是否是默认回复内容
}

type WechatReplyRuleDeleteRequest struct {
	Id uint `json:"id"` // 规则 ID
}

type WechatMenuRequest struct {
	Id       uint   `json:"id"`        // 菜单 ID
	ParentId uint   `json:"parent_id"` // 上级菜单ID
	Name     string `json:"name"`      // 菜单名称
	Type     string `json:"type"`      // 菜单类型
	Value    string `json:"value"`     // 菜单只
	Sort     uint   `json:"sort"`      // 排序，小的在前
}

type WechatMenuDeleteRequest struct {
	Id uint `json:"id"` // 菜单 ID
}
