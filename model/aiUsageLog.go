package model

// AiUsageLog 记录 AI 每轮调用的 token 用量与成本 (P8 遥测与成本归因)。
type AiUsageLog struct {
	Id               uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime      int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	SessionId        string `json:"session_id" gorm:"column:session_id;type:varchar(64) not null;default:'';index:idx_session_id;comment:会话ID"`
	AgentId          uint   `json:"agent_id" gorm:"column:agent_id;type:int(10) unsigned not null;default:0;index:idx_agent_id;comment:智能体ID (0=主会话)"`
	TurnId           int    `json:"turn_id" gorm:"column:turn_id;type:int(10) not null;default:0;comment:本轮轮次编号"`
	ModelName        string `json:"model_name" gorm:"column:model_name;type:varchar(128) not null;default:'';index:idx_model_name;comment:模型名称"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"column:prompt_tokens;type:int(10) not null;default:0;comment:输入token"`
	CompletionTokens int    `json:"completion_tokens" gorm:"column:completion_tokens;type:int(10) not null;default:0;comment:输出token"`
	TotalTokens      int    `json:"total_tokens" gorm:"column:total_tokens;type:int(10) not null;default:0;comment:总token"`
	Cost             float64 `json:"cost" gorm:"column:cost;type:decimal(12,6) not null;default:0.000000;comment:成本(USD)"`
	ToolCalls        int    `json:"tool_calls" gorm:"column:tool_calls;type:int(10) not null;default:0;comment:本轮工具调用次数"`
	DurationMs       int64  `json:"duration_ms" gorm:"column:duration_ms;type:bigint(20) not null;default:0;comment:本轮耗时(毫秒)"`
}
