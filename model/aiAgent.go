package model

// AiAgent 定义一个 AI 智能体，拥有独立的会话上下文，可定时执行任务。
type AiAgent struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	SessionId   string `json:"session_id" gorm:"column:session_id;type:varchar(64) not null;default:'';index:idx_session_id;comment:专属会话ID"`
	Name        string `json:"name" gorm:"column:name;type:varchar(200) not null;default:'';comment:智能体名称"`
	Strategy    string `json:"strategy" gorm:"column:strategy;type:text;comment:执行策略描述"`
	CronExpr    string `json:"cron_expr" gorm:"column:cron_expr;type:varchar(100) not null;default:'';comment:Cron表达式"`
	Enabled     int    `json:"enabled" gorm:"column:enabled;type:tinyint(1) not null;default:1;index:idx_enabled;comment:1启用 0暂停"`
	LastRunAt   int64  `json:"last_run_at" gorm:"column:last_run_at;type:bigint(20) not null;default:0;comment:上次执行时间"`
	NextRunAt   int64  `json:"next_run_at" gorm:"column:next_run_at;type:bigint(20) not null;default:0;index:idx_next_run_at;comment:下次执行时间"`
	RunCount    int    `json:"run_count" gorm:"column:run_count;type:int(10) not null;default:0;comment:已执行次数"`
	MaxRuns     int    `json:"max_runs" gorm:"column:max_runs;type:int(10) not null;default:0;comment:最大执行次数(0=不限)"`
	LastSummary string `json:"last_summary" gorm:"column:last_summary;type:text;comment:上次执行摘要"`
}
