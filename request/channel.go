package request

// Channel 是后台保存渠道连接（企业 IM 接入）的请求体。
//
// 字段的 json tag 同时是 apicatalog 生成 schema 的来源，改 tag 会直接改动模型可见的
// 参数名，必须与 provider/channelAdmin.go 的读取口径同步。
//
// 注意 AppSecret 的语义（R84）：传掩码哨兵或空串表示「沿用库中真值」，
// 只有传新值才会覆盖——后台表单整份提交时不会回填真值，这条规则是必需的。
type Channel struct {
	Id          uint   `json:"id"`           // 渠道ID，0 表示新建
	Platform    string `json:"platform"`     // 平台：feishu（飞书）/ dingtalk（钉钉）
	Name        string `json:"name"`         // 渠道名称
	AppId       string `json:"app_id"`       // 平台应用ID
	OrgId       string `json:"org_id"`       // 组织标识（飞书 tenant_key / 钉钉 corp_id），可为空
	AppSecret   string `json:"app_secret"`   // 应用密钥：掩码或空串表示沿用现有值
	Enabled     int    `json:"enabled"`      // 是否启用连接：1=启用，0=停用
	ChatEnabled int    `json:"chat_enabled"` // 是否开启聊天管理：1=开启，0=仅通知
	AgentId     uint   `json:"agent_id"`     // 接待智能体ID，开启聊天管理时必填
	PolicyJson  string `json:"policy_json"`  // 能力动作策略JSON对象（intent+action → allow/confirm/deny）
	Revision    int    `json:"revision"`     // 配置修订号（乐观锁），更新时必须与当前值一致
}

// ChannelDeleteRequest 是删除渠道的请求体。
type ChannelDeleteRequest struct {
	Id uint `json:"id"` // 渠道ID
}
