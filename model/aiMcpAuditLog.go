package model

// AiMcpAuditLog 记录每一次 MCP tools/call 调用的审计日志。
// 仅记录 token 掩码、参数摘要与结果，绝不落库完整 token 或敏感参数原文。
type AiMcpAuditLog struct {
	Id         uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	// SiteId 为被操作的目标站点（site_id 切换后的站点）
	SiteId uint `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:目标站点ID"`
	// TokenMask 为调用方 token 的掩码（前后各 4 位，中间打码）
	TokenMask string `json:"token_mask" gorm:"column:token_mask;type:varchar(64) not null;default:'';comment:Token掩码"`
	Tool      string `json:"tool" gorm:"column:tool;type:varchar(64) not null;default:'';index:idx_tool;comment:工具名"`
	// Risk: read / write / destructive / system
	Risk       string `json:"risk" gorm:"column:risk;type:varchar(16) not null;default:'read';comment:风险等级"`
	ArgsDigest string `json:"args_digest" gorm:"column:args_digest;type:varchar(64) not null;default:'';comment:参数SHA256"`
	ArgsPreview string `json:"args_preview" gorm:"column:args_preview;type:text;comment:参数前500字符预览"`
	Ok         int    `json:"ok" gorm:"column:ok;type:tinyint(1) not null;default:1;comment:1成功 0失败"`
	ErrMsg     string `json:"err_msg" gorm:"column:err_msg;type:text;comment:错误信息"`
	CostMs     int64  `json:"cost_ms" gorm:"column:cost_ms;type:bigint(20) not null;default:0;comment:耗时毫秒"`
	ClientIp   string `json:"client_ip" gorm:"column:client_ip;type:varchar(64) not null;default:'';comment:调用方IP"`
	UserAgent  string `json:"user_agent" gorm:"column:user_agent;type:varchar(255) not null;default:'';comment:User-Agent"`
}
