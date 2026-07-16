package model

// AiAgentLog 记录 AI 智能体每次执行的日志。
type AiAgentLog struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	AgentId     uint   `json:"agent_id" gorm:"column:agent_id;type:int(10) unsigned not null;default:0;index:idx_agent_id;comment:智能体ID"`
	SessionId   string `json:"session_id" gorm:"column:session_id;type:varchar(64) not null;default:'';comment:本次执行的会话ID"`
	Status      int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0;comment:0执行中 1成功 2失败"`
	Summary     string `json:"summary" gorm:"column:summary;type:text;comment:执行摘要"`
	Error       string `json:"error" gorm:"column:error;type:text;comment:错误信息"`
	ToolCalls   int    `json:"tool_calls" gorm:"column:tool_calls;type:int(10) not null;default:0;comment:工具调用次数"`
	StartedAt   int64  `json:"started_at" gorm:"column:started_at;type:bigint(20) not null;default:0;comment:开始时间"`
	FinishedAt  int64  `json:"finished_at" gorm:"column:finished_at;type:bigint(20) not null;default:0;comment:结束时间"`
}
