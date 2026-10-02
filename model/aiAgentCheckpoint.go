package model

// AiAgentCheckpoint 记录 Agent 单次执行的断点，使进程崩溃/重启后能从断点续跑，
// 而不是从头再来一遍（长任务尤其重要）。
//
// 语义：
//   - 每次 ExecuteAgent 启动时查询本表：若存在 status=Running 的记录，说明上一次执行
//     在进程被杀时还没结束 → 从 checkpoint.Round 继续（已持久化的会话历史已包含此前所有轮次）。
//   - 每完成一轮就 upsert 一次，round = 已完成轮数。
//   - 正常结束/出错都置为终态（Done/Failed），下次调度触发即为全新一次执行。
type AiAgentCheckpoint struct {
	Id        uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	AgentId   uint   `json:"agent_id" gorm:"column:agent_id;type:int(10) unsigned not null;index:idx_agent_id;comment:Agent ID"`
	SessionId string `json:"session_id" gorm:"column:session_id;type:varchar(64) not null;default:'';comment:会话ID"`
	Round     int    `json:"round" gorm:"column:round;type:int(10) not null;default:0;comment:已完成轮数（下次从该轮继续）"`
	MaxRounds int    `json:"max_rounds" gorm:"column:max_rounds;type:int(10) not null;default:0;comment:本次执行允许的最大轮数"`
	Status    int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:1;index:idx_status;comment:1执行中 2完成 3失败"`
	CreatedAt int64  `json:"created_at" gorm:"column:created_at;type:bigint(20);autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"column:updated_at;type:bigint(20);autoUpdateTime"`
}

// AgentCheckpoint 状态常量
const (
	AgentCheckpointRunning = 1
	AgentCheckpointDone    = 2
	AgentCheckpointFailed  = 3
)

func (AiAgentCheckpoint) TableName() string {
	return "ai_agent_checkpoint"
}
