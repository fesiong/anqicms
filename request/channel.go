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
	Revision    int    `json:"revision"`     // 配置修订号（仅后台保存的乐观锁），更新时必须与当前值一致
}

// ChannelDeleteRequest 是删除渠道的请求体。
type ChannelDeleteRequest struct {
	Id uint `json:"id"` // 渠道ID
}

// ChannelBindingTokenRequest 是生成一次性绑定 token 的请求体。
// 管理员身份只取登录态，不接受请求体传入（R89）。
type ChannelBindingTokenRequest struct {
	ChannelId uint `json:"channel_id"` // 渠道ID
}

// ChannelBindingRevokeRequest 是解除绑定的请求体。
type ChannelBindingRevokeRequest struct {
	Id uint `json:"id"` // 绑定ID
}

// ChannelTargetRequest 是新建/更新通知目标的请求体（id=0 表示新建）。
//
// kind 只接受 group：私聊目标只能由绑定流程创建（R90）。
// 取消批准用 approved=0，不提供删除端点。
type ChannelTargetRequest struct {
	Id                uint   `json:"id"`                 // 目标ID，0 表示新建
	ChannelId         uint   `json:"channel_id"`         // 所属渠道ID
	Kind              string `json:"kind"`               // 目标类型：只接受 group
	PlatformTargetId  string `json:"platform_target_id"` // 平台目标标识（群 chat_id）
	Approved          int    `json:"approved"`           // 1已批准 0未批准
	ManagementEnabled int    `json:"management_enabled"` // 1允许群管理 0仅通知
}

// ChannelRuleRequest 是新建/更新通知规则的请求体（id=0 表示新建）。
type ChannelRuleRequest struct {
	Id         uint   `json:"id"`          // 规则ID，0 表示新建
	Name       string `json:"name"`        // 规则名称
	EventType  string `json:"event_type"`  // 事件类型：四个固定常量之一
	Enabled    int    `json:"enabled"`     // 1启用 0停用
	TargetIds  []uint `json:"target_ids"`  // 接收目标ID列表（必须属于本站点）
	TemplateId uint   `json:"template_id"` // 通知模板ID（0 表示用默认模板）
}

// ChannelRuleDeleteRequest 是删除通知规则的请求体。
type ChannelRuleDeleteRequest struct {
	Id uint `json:"id"` // 规则ID
}

// ChannelTemplateRequest 是新建/更新通知模板的请求体（id=0 表示新建）。
type ChannelTemplateRequest struct {
	Id                    uint   `json:"id"`                      // 模板ID，0 表示新建
	Key                   string `json:"key"`                     // 模板键（站点内唯一）
	Name                  string `json:"name"`                    // 模板名称
	Subject               string `json:"subject"`                 // 标题模板
	Content               string `json:"content"`                 // 正文模板（受限 Pongo2 语法）
	PlatformOverridesJson string `json:"platform_overrides_json"` // 按平台覆盖的标题与正文JSON
}

// ChannelTemplatePreviewRequest 是按目标安全级别预览模板的请求体（R92）。
type ChannelTemplatePreviewRequest struct {
	Id          uint   `json:"id"`           // 已保存模板ID；>0 时忽略 subject/content
	Subject     string `json:"subject"`      // 标题模板（未保存的草稿）
	Content     string `json:"content"`      // 正文模板（未保存的草稿）
	TargetKind  string `json:"target_kind"`  // group / private
	PayloadJson string `json:"payload_json"` // 示例载荷（可选）
}

// ChannelTestSendRequest 是配置验证/示例消息试发的请求体。
type ChannelTestSendRequest struct {
	Id       uint `json:"id"`        // 渠道ID
	TargetId uint `json:"target_id"` // 目标ID（可选；0 表示只做配置校验）
}

// ChannelDeliveryRetryRequest 是人工重试投递的请求体。
type ChannelDeliveryRetryRequest struct {
	Id uint `json:"id"` // 投递ID
}
