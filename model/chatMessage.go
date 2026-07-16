package model

// AiChatMessage stores a single chat message in the database.
type AiChatMessage struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	SessionId   string `json:"session_id" gorm:"column:session_id;type:varchar(64) not null;default:'';index:idx_session_id"`
	Role        string `json:"role" gorm:"column:role;type:varchar(16) not null;default:''"`
	Content     string `json:"content" gorm:"column:content;type:longtext"`
	// ToolCallID 用于 tool 角色时记录工具调用ID，与 AI 返回的 tool_call 配对
	ToolCallID string `json:"tool_call_id" gorm:"column:tool_call_id;type:varchar(64) not null;default:'';index;comment:工具调用ID"`
	// ToolName 记录调用的工具名称，用于压缩时智能精简（如 read_file 保留骨架、bash 取首行）
	ToolName string `json:"tool_name" gorm:"column:tool_name;type:varchar(64) not null;default:'';comment:工具名称"`
	// ToolCalls 用于 assistant 角色时记录工具调用 JSON（多轮对话重建用）
	ToolCalls string `json:"tool_calls" gorm:"column:tool_calls;type:longtext;comment:工具调用 JSON"`
	// TurnID 归属回合ID，同一个用户请求及其后续 AI 响应/工具调用属于同一回合
	TurnID uint `json:"turn_id" gorm:"column:turn_id;type:int(10) unsigned not null;default:0;index:idx_turn_id;comment:回合ID"`
	// IsSummary 标记是否为历史摘要消息（压缩后生成）
	IsSummary bool `json:"is_summary" gorm:"column:is_summary;type:tinyint(1) not null;default:0;comment:是否摘要消息"`
	// SummaryOf 若为摘要消息，记录被覆盖的起始消息ID
	SummaryOf uint `json:"summary_of" gorm:"column:summary_of;type:int(10) unsigned not null;default:0;comment:摘要覆盖起始ID"`
	// Files 用户上传的文件列表（JSON 数组）
	Files string `json:"files" gorm:"column:files;type:text;comment:上传文件列表 JSON"`
}
