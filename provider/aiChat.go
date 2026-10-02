package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/kataras/iris/v12"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/ai/eino"
	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// cronParser 支持 5 字段（标准）和 6 字段（带秒）的 cron 表达式
var cronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// ================================================================
// P4: 近期 AI 执行错误环形缓冲 (供 autoDiagnoseErrors 诊断使用)
//
// aiChat 主循环在用户消息命中错误关键词时，会调用 autoDiagnoseErrors()
// 把"近期错误上下文"回灌给模型。原先该函数恒返回空，诊断分支形同虚设。
// 这里维护一个进程级、有界、并发安全的环形缓冲，由 AI 执行链路在出现
// 错误时写入 (RecordAIError)，控制器侧读取 (GetRecentAIErrors) 做格式化。
// 不依赖全局 slog handler，也不读写日志文件，零外部依赖、重启即清空。
// ================================================================

// RecentAIError 一条近期 AI 执行错误记录
type RecentAIError struct {
	TS  time.Time `json:"ts"`
	Msg string    `json:"msg"`
}

const recentAIErrorCap = 64

var (
	recentAIErrorsMu   sync.Mutex
	recentAIErrorsBuf  []RecentAIError
	recentAIErrorsHead int
	recentAIErrorsSize int
)

// RecordAIError 写入一条 AI 执行链路错误到近期错误缓冲。
// 线程安全；缓冲满时覆盖最旧记录。
func RecordAIError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	recentAIErrorsMu.Lock()
	defer recentAIErrorsMu.Unlock()
	if recentAIErrorsBuf == nil {
		recentAIErrorsBuf = make([]RecentAIError, recentAIErrorCap)
	}
	recentAIErrorsBuf[recentAIErrorsHead] = RecentAIError{TS: time.Now(), Msg: msg}
	recentAIErrorsHead = (recentAIErrorsHead + 1) % recentAIErrorCap
	if recentAIErrorsSize < recentAIErrorCap {
		recentAIErrorsSize++
	}
}

// GetRecentAIErrors 返回近期错误记录，按时间从旧到新排列。
// 若缓冲为空，返回零长度切片。
func GetRecentAIErrors() []RecentAIError {
	recentAIErrorsMu.Lock()
	defer recentAIErrorsMu.Unlock()
	out := make([]RecentAIError, 0, recentAIErrorsSize)
	if recentAIErrorsSize == 0 {
		return out
	}
	start := 0
	if recentAIErrorsSize == recentAIErrorCap {
		// 满时 head 指向最旧记录
		start = recentAIErrorsHead
	}
	for i := 0; i < recentAIErrorsSize; i++ {
		out = append(out, recentAIErrorsBuf[(start+i)%recentAIErrorCap])
	}
	return out
}

// Turn represents a round of interaction: one user message + all following
// AI responses and tool calls/results.
type Turn struct {
	StartIdx int    `json:"start_idx"` // index in Messages slice
	MsgCount int    `json:"msg_count"` // number of messages in this turn
	Summary  string `json:"summary"`   // LLM-generated summary (after compression)
}

// ChatSession represents a user chat session
type ChatSession struct {
	ID                 string           `json:"id"`
	Messages           []ChatMessage    `json:"messages"`
	Turns              []Turn           `json:"turns"`
	CreatedAt          time.Time        `json:"created_at"`
	CachedSystemPrompt string           `json:"-"` // 每会话缓存一次，保持 prefix cache 稳定
	DeclaredPackages   []string         `json:"-"` // 模型声明的能力包列表
	ToolsFinalized     bool             `json:"-"` // true 表示已从声明阶段切换到执行阶段
	AllowOnce          *SessionAllowSet `json:"-"` // P0: 本会话已"本次允许"的工具集合
}

// ChatMessage represents a message in a conversation
type ChatMessage struct {
	Role        string        `json:"role"`
	Content     string        `json:"content"`
	CreatedTime int64         `json:"created_time"`
	ToolCallID  string        `json:"tool_call_id,omitempty"`
	ToolName    string        `json:"tool_name,omitempty"`
	TurnID      uint          `json:"turn_id,omitempty"`
	ToolCalls   string        `json:"tool_calls,omitempty"`
	Files       []ChatFileRef `json:"files,omitempty"`
}

// ChatFileRef represents a reference to an uploaded file
type ChatFileRef struct {
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
}

// AiChatService manages AI chat conversations
type AiChatService struct {
	sessions    map[string]*ChatSession
	mu          sync.RWMutex
	Logger      *slog.Logger
	db          *gorm.DB
	site        *Website
	projectRoot string

	// Tuning P1-6: 主对话循环的可调阈值。零值 = 用默认值（见 aiTuning.go），
	// 因此未做任何配置时行为与改造前完全一致。
	Tuning ChatTuning

	// Cached tool definitions and handlers (initialized once)
	Tools    []*schema.ToolInfo
	Handlers map[string]toolHandler

	// 意图层工具（E 阶段：AIChat 改用意图内核，与 MCP 共用 IntentCatalog）
	// 端点 handler 仅保留作意图委托的底层能力（caps），不再作为独立工具暴露给模型。
	intentTools    []*schema.ToolInfo
	intentHandlers map[string]toolHandler

	// capHandlers：纯底层能力表（端点 + 内置 handler），供意图委托调用。
	// 与 Handlers 分离，确保同名意图（web_fetch/web_search）委托到的是底层 cap 而非意图 handler 自身，
	// 从而避免无限递归。Handlers = capHandlers + intentHandlers（统一 dispatch 表）。
	capHandlers map[string]toolHandler

	// P0: 待审批工具调用注册表
	// key = toolCallID, value = decision channel
	// 主会话 write_gate 挂起时写入，前端 POST /ai/chat/confirm 唤醒
	pendingApprovals   map[string]chan string
	pendingApprovalsMu sync.Mutex

	// msgSeq: 全局单调递增序号，保证同一 session 内 DB 消息写入顺序
	// 解决异步并发写入导致 DB 乱序的问题
	msgSeq int64

	// Agent 调度器
	agents        map[uint]*model.AiAgent
	agentsMu      sync.RWMutex
	schedulerQuit chan struct{}

	// runningAgents 防止同一 agent 被并发重复执行
	// key = agent.Id, value = true 表示正在执行
	runningAgents   map[uint]bool
	runningAgentsMu sync.Mutex

	// P8: 遥测与成本归因记录器
	telemetryRecorder *TelemetryRecorder

	// P2-7: 结构化 Tracing 记录器（进程内环形缓冲，默认开启）
	trace *TraceRecorder

	// P8: 当前使用的模型名 (用于遥测记录)
	ModelName string

	// P4a: 技能 allowed-tools 约束
	// activeSkillTools: 当前激活的技能允许使用的工具集合
	// 为 nil 表示无技能激活，所有工具可用
	// 为非 nil (即使空 map) 表示只能使用该集合内的工具
	activeSkillTools map[string]bool
	activeSkillMu    sync.RWMutex
}

// NewAiChatService creates a new AI chat service
func (w *Website) NewAiChatService() *AiChatService {
	svc := &AiChatService{
		mu:               sync.RWMutex{},
		sessions:         make(map[string]*ChatSession),
		Logger:           slog.Default(),
		db:               w.DB,
		site:             w,
		pendingApprovals: make(map[string]chan string),
		agents:           make(map[uint]*model.AiAgent),
		runningAgents:    make(map[uint]bool),
		projectRoot:      w.RootPath,
		trace:            NewTraceRecorder(500),
	}

	// P8: 初始化遥测与成本归因记录器
	if w.DB != nil {
		svc.telemetryRecorder = NewTelemetryRecorder(w.DB)
	}

	// E 阶段：AIChat 改用意图内核（与 MCP 共用 IntentCatalog）。
	// 端点 + 内置 handler 仅作为意图委托的底层能力（caps），不再作为独立工具暴露给模型。
	endpointTools, endpointHandlers := svc.getEinoTools()
	builtinTools, builtinHandlers := svc.getBuiltinEinoTools()

	// capHandlers：纯底层能力表（端点 + 内置），供意图委托调用，不含意图 handler，
	// 以避免同名意图（web_fetch/web_search）经委托再次命中意图 handler 造成递归。
	capHandlers := make(map[string]toolHandler, len(endpointHandlers)+len(builtinHandlers))
	for n, h := range endpointHandlers {
		capHandlers[n] = h
	}
	for n, h := range builtinHandlers {
		capHandlers[n] = h
	}
	svc.capHandlers = capHandlers

	// G3：通用 REST 调用元能力。仅作为底层 cap 注册（不进模型可见的 Tools），
	// 由 api_list / api_schema / api_invoke 三个意图委托使用。
	capHandlers[capAPIList] = svc.capAPIList
	capHandlers[capAPISchema] = svc.capAPISchema
	capHandlers[capAPIInvoke] = svc.capAPIInvoke

	// 意图工具：模型面可见的工具集合（意图导向工作流）。
	intentTools, intentHandlers := svc.GetIntentTools()

	// 模型面工具 = 意图工具 + 未与意图同名的内置工具。
	// web_fetch/web_search 由意图提供，内置同名工具不再重复绑定（避免 BindTools 重复工具名）。
	intentNames := make(map[string]bool, len(intentTools))
	for _, t := range intentTools {
		intentNames[t.Name] = true
	}
	svc.Tools = make([]*schema.ToolInfo, 0, len(intentTools)+len(builtinTools))
	svc.Tools = append(svc.Tools, intentTools...)
	for _, bt := range builtinTools {
		if intentNames[bt.Name] {
			continue
		}
		svc.Tools = append(svc.Tools, bt)
	}

	// 统一 dispatch 表 = 底层能力 + 意图 handler。
	// 意图 handler 覆盖同名 cap：dispatch 命中意图 handler 后，由 capInvoker 经 capHandlers 调用真正的 cap。
	svc.Handlers = make(map[string]toolHandler, len(capHandlers)+len(intentHandlers))
	for n, h := range capHandlers {
		svc.Handlers[n] = h
	}
	for n, h := range intentHandlers {
		svc.Handlers[n] = h
	}

	_ = endpointTools // 端点工具信息不再暴露给模型，仅保留底层能力 handler
	svc.Logger.Info("AI tools initialized (intent mode)", "tools", len(svc.Tools), "caps", len(capHandlers))

	// P4a: 激活技能 allowed-tools 约束。skill_get 在加载带 allowed_tools 的技能时，
	// 通过本 hook 把约束注入服务；generateAIResponse 每轮开头会 ClearSkillToolScope，
	// 把约束限制在当前对话轮内，避免跨会话/跨轮泄漏。此前 hook 从未注册，
	// 导致技能声明的 allowed_tools 完全不生效（见 aiSkill.go 注释）。
	SetSkillScopeHook(svc.SetSkillToolScope)

	// 启动自检：REST 端点（route/manage.go）× 意图层声明的可达性审计。
	// 仅记录结论、不阻断启动——generic 缺口是预期状态，靠补齐意图逐步收敛。
	//
	// 口径说明：覆盖率 = (direct+gated)/端点总数，衡量"AI 有多少语义化入口"。
	// 早期口径拿 getEinoTools() 当能力真相源，H 阶段删掉 84 个平行实现后掉到 0.8%，
	// 那是"删得越干净指标越难看"的反向指标，已废弃。
	if rep, cerr := svc.AuditCapabilityCoverage(""); cerr == nil {
		svc.Logger.Info("AI 能力覆盖审计",
			"端点", rep.TotalEndpint, "声明可达", rep.TotalDeclared,
			"意图", rep.TotalIntent, "无端点能力", rep.TotalCap, "本地能力", rep.TotalBuiltin,
			"direct", rep.Direct, "gated", rep.Gated,
			"generic", rep.Generic, "blocked", rep.Blocked,
			"覆盖率", fmt.Sprintf("%.1f%%", float64(rep.Direct+rep.Gated)/float64(maxInt(rep.TotalEndpint, 1))*100),
			"悬空声明", len(rep.OrphanTargets))
		if len(rep.OrphanTargets) > 0 {
			svc.Logger.Warn("意图层声明了不存在的端点，调用必然 404",
				"数量", len(rep.OrphanTargets), "样例", strings.Join(rep.OrphanTargets, ", "))
		}
	} else {
		svc.Logger.Debug("跳过 AI 能力覆盖审计", "reason", cerr.Error())
	}
	// Load sessions from database on startup
	svc.loadSessionsFromDB()
	// Load agents and start scheduler
	svc.loadAgentsFromDB()
	svc.StartAgentScheduler()
	w.AiSrv = svc
	return svc
}

// loadSessionsFromDB loads all existing sessions from the database into memory
func (svc *AiChatService) loadSessionsFromDB() {
	if svc.db == nil {
		return
	}
	// Get distinct session IDs, ordered by first message time
	type sessionRow struct {
		SessionId   string
		CreatedTime int64
	}
	var rows []sessionRow
	svc.db.Model(&model.AiChatMessage{}).
		Select("session_id, MIN(created_time) as created_time").
		Group("session_id").
		Order("MIN(created_time) ASC").
		Scan(&rows)
	if len(rows) == 0 {
		return
	}
	for _, row := range rows {
		sess := &ChatSession{
			ID:        row.SessionId,
			Messages:  make([]ChatMessage, 0),
			CreatedAt: time.Unix(row.CreatedTime, 0),
		}
		// Load messages for this session
		var dbMessages []model.AiChatMessage
		svc.db.Model(&model.AiChatMessage{}).
			Where("session_id = ?", row.SessionId).
			Order("seq ASC").
			Find(&dbMessages)
		for _, dbm := range dbMessages {
			sess.Messages = append(sess.Messages, dbMessageToChatMessage(dbm))
		}
		rebuildTurns(sess)
		svc.sessions[row.SessionId] = sess
	}
	svc.Logger.Info("Loaded AI chat sessions from DB", "count", len(rows))
}

// GetOrCreateSession gets or creates a chat session
func (svc *AiChatService) GetOrCreateSession(sessionID string) *ChatSession {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if sess, exists := svc.sessions[sessionID]; exists {
		return sess
	}

	// Try loading from DB first
	if svc.db != nil {
		var dbMessages []model.AiChatMessage
		svc.db.Model(&model.AiChatMessage{}).
			Where("session_id = ?", sessionID).
			Order("seq ASC").
			Find(&dbMessages)
		if len(dbMessages) > 0 {
			sess := &ChatSession{
				ID:        sessionID,
				Messages:  make([]ChatMessage, 0, len(dbMessages)),
				CreatedAt: time.Unix(dbMessages[0].CreatedTime, 0),
			}
			for _, dbm := range dbMessages {
				sess.Messages = append(sess.Messages, dbMessageToChatMessage(dbm))
			}
			rebuildTurns(sess)
			svc.sessions[sessionID] = sess
			return sess
		}
	}

	sess := &ChatSession{
		ID:        sessionID,
		Messages:  make([]ChatMessage, 0),
		Turns:     make([]Turn, 0),
		CreatedAt: time.Now(),
	}
	svc.sessions[sessionID] = sess
	return sess
}

// AddMessage adds a message to a session and updates turn tracking.
func (svc *AiChatService) AddMessage(sessionID string, msg ChatMessage) {
	svc.mu.Lock()

	sess, exists := svc.sessions[sessionID]
	if !exists {
		sess = &ChatSession{
			ID:        sessionID,
			Messages:  make([]ChatMessage, 0),
			Turns:     make([]Turn, 0),
			CreatedAt: time.Now(),
		}
		svc.sessions[sessionID] = sess
	}
	now := time.Now().Unix()
	msg.CreatedTime = now

	// Turn tracking: user message starts a new turn
	if msg.Role == "user" {
		// Close previous active turn
		if len(sess.Turns) > 0 {
			last := &sess.Turns[len(sess.Turns)-1]
			last.MsgCount = len(sess.Messages) - last.StartIdx
		}
		// New turn starts at the upcoming message index
		sess.Turns = append(sess.Turns, Turn{
			StartIdx: len(sess.Messages),
			MsgCount: 1,
		})
	}

	sess.Messages = append(sess.Messages, msg)

	// Update turn count for non-user messages (extend current turn)
	if msg.Role != "user" && len(sess.Turns) > 0 {
		last := &sess.Turns[len(sess.Turns)-1]
		last.MsgCount = len(sess.Messages) - last.StartIdx
	}

	svc.mu.Unlock()

	// Persist to database synchronously with monotonic seq
	// 同步写入 + 单调递增 seq 保证 DB 消息顺序，解决异步并发写入乱序问题
	if svc.db != nil {
		// 原子递增 seq，保证全局顺序
		seq := atomic.AddInt64(&svc.msgSeq, 1)
		filesJSON, _ := json.Marshal(msg.Files)
		dbMsg := &model.AiChatMessage{
			SessionId:  sessionID,
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			ToolName:   msg.ToolName,
			TurnID:     msg.TurnID,
			ToolCalls:  msg.ToolCalls,
			Files:      string(filesJSON),
			Seq:        int(seq),
		}
		if err := svc.db.Create(dbMsg).Error; err != nil {
			svc.Logger.Error("Failed to persist chat message", "error", err)
		}
	}
}

// dbMessageToChatMessage converts a database AiChatMessage to a ChatMessage,
// parsing the Files JSON field if present.
func dbMessageToChatMessage(dbm model.AiChatMessage) ChatMessage {
	msg := ChatMessage{
		Role:        dbm.Role,
		Content:     dbm.Content,
		CreatedTime: dbm.CreatedTime,
		ToolCallID:  dbm.ToolCallID,
		ToolName:    dbm.ToolName,
		TurnID:      dbm.TurnID,
		ToolCalls:   dbm.ToolCalls,
	}
	if dbm.Files != "" {
		var files []ChatFileRef
		if err := json.Unmarshal([]byte(dbm.Files), &files); err == nil {
			msg.Files = files
		}
	}
	return msg
}

// rebuildTurns rebuilds the TurnTracker from the session's message list.
func rebuildTurns(sess *ChatSession) {
	sess.Turns = make([]Turn, 0)
	for i, msg := range sess.Messages {
		if msg.Role == "user" {
			sess.Turns = append(sess.Turns, Turn{
				StartIdx: i,
				MsgCount: 1,
			})
		} else if len(sess.Turns) > 0 {
			last := &sess.Turns[len(sess.Turns)-1]
			last.MsgCount = i - last.StartIdx + 1
		}
	}
}

// GetMessages returns messages from a session
func (svc *AiChatService) GetMessages(sessionID string) []ChatMessage {
	svc.mu.RLock()
	sess, exists := svc.sessions[sessionID]
	svc.mu.RUnlock()

	if !exists {
		// Try loading from DB
		svc.mu.Lock()
		// Double-check after acquiring write lock
		if sess, exists = svc.sessions[sessionID]; !exists && svc.db != nil {
			var dbMessages []model.AiChatMessage
			svc.db.Model(&model.AiChatMessage{}).
				Where("session_id = ?", sessionID).
				Order("seq ASC").
				Find(&dbMessages)
			if len(dbMessages) > 0 {
				sess = &ChatSession{
					ID:        sessionID,
					Messages:  make([]ChatMessage, 0, len(dbMessages)),
					CreatedAt: time.Unix(dbMessages[0].CreatedTime, 0),
				}
				for _, dbm := range dbMessages {
					sess.Messages = append(sess.Messages, dbMessageToChatMessage(dbm))
				}
				rebuildTurns(sess)
				svc.sessions[sessionID] = sess
			}
		}
		svc.mu.Unlock()
		if sess == nil {
			return nil
		}
	}

	svc.mu.RLock()
	defer svc.mu.RUnlock()
	result := make([]ChatMessage, len(sess.Messages))
	copy(result, sess.Messages)
	return result
}

// ListSessions returns a list of all sessions with summary info
func (svc *AiChatService) ListSessions() []iris.Map {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	var result []iris.Map
	for id, sess := range svc.sessions {
		lastMsg := ""
		title := ""
		if len(sess.Messages) > 0 {
			lastMsg = sess.Messages[len(sess.Messages)-1].Content
			if len([]rune(lastMsg)) > 100 {
				lastMsg = string([]rune(lastMsg)[:100]) + "..."
			}
			// Find first user message as title
			for _, msg := range sess.Messages {
				if msg.Role == "user" {
					title = msg.Content
					if len([]rune(title)) > 50 {
						title = string([]rune(title)[:50]) + "..."
					}
					break
				}
			}
		}
		result = append(result, iris.Map{
			"session_id":   id,
			"title":        title,
			"created_time": sess.CreatedAt.Unix(),
			"updated_at":   sess.Messages[len(sess.Messages)-1].CreatedTime,
			"msg_count":    len(sess.Messages),
			"last_message": lastMsg,
		})
	}
	return result
}

// CloseSession closes and removes a session
func (svc *AiChatService) CloseSession(sessionID string) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	delete(svc.sessions, sessionID)
}

// ================================================================
//  Agent 调度器
// ================================================================

// loadAgentsFromDB 从数据库加载所有启用的 Agent 到内存
func (svc *AiChatService) loadAgentsFromDB() {
	if svc.db == nil {
		return
	}
	var agents []model.AiAgent
	svc.db.Where("enabled = 1").Find(&agents)
	svc.agentsMu.Lock()
	for i := range agents {
		// 修复：如果 NextRunAt=0 或已过期（重启场景），重新计算下次执行时间
		if agents[i].CronExpr != "" && (agents[i].NextRunAt == 0 || agents[i].NextRunAt <= time.Now().Unix()) {
			scheduler, err := cronParser.Parse(agents[i].CronExpr)
			if err == nil {
				agents[i].NextRunAt = scheduler.Next(time.Now()).Unix()
				svc.db.Model(&agents[i]).Update("next_run_at", agents[i].NextRunAt)
			}
		}
		svc.agents[agents[i].Id] = &agents[i]
	}
	svc.agentsMu.Unlock()
	svc.Logger.Info("Loaded AI agents from DB", "count", len(agents))
}

// StartAgentScheduler 启动后台调度器 goroutine，每 30 秒检查一次到期 Agent
func (svc *AiChatService) StartAgentScheduler() {
	svc.schedulerQuit = make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				svc.checkDueAgents()
			case <-svc.schedulerQuit:
				return
			}
		}
	}()
	svc.Logger.Info("Agent scheduler started")
}

// StopAgentScheduler 停止后台调度器
func (svc *AiChatService) StopAgentScheduler() {
	if svc.schedulerQuit != nil {
		close(svc.schedulerQuit)
		svc.schedulerQuit = nil
	}
}

// checkDueAgents 检查并执行到期的 Agent
func (svc *AiChatService) checkDueAgents() {
	// 先检查是否配置了ai
	oldCfg := eino.GlobalConfig()
	if oldCfg == nil {
		// 未配置，直接返回
		// 需要检查默认配置
		defaultSite := CurrentSite(nil)
		aiSetting := defaultSite.LoadAiSetting("")
		if len(aiSetting.Configs) > 0 && !strings.HasPrefix(aiSetting.LastModel, "anqi") {
			// 进行配置
			var cfg *eino.Config
			if aiSetting.LastModel != "" {
				for _, v := range aiSetting.Configs {
					if v.Model == aiSetting.LastModel {
						cfg = v
						break
					}
				}
			}
			if cfg == nil {
				cfg = aiSetting.Configs[0]
			}
			if err := eino.SetGlobalConfig(cfg); err != nil {
				slog.Error("Failed to initialize AI client", "error", err)
				return
			} else {
				slog.Info("AI client initialized successfully")
			}
		} else if config.AnqiUser.AuthId > 0 {
			if err := eino.SetOfficialConfig(aiSetting.LastModel); err != nil {
				slog.Error("Failed to initialize AI client", "error", err)
				return
			} else {
				slog.Info("AI client initialized successfully")
			}
		} else {
			// 配置错误，直接返回
			return
		}
	}
	now := time.Now().Unix()
	svc.agentsMu.RLock()
	var dueList []*model.AiAgent
	for _, agent := range svc.agents {
		if agent.Enabled == 1 && agent.NextRunAt > 0 && agent.NextRunAt <= now {
			dueList = append(dueList, agent)
		}
	}
	svc.agentsMu.RUnlock()

	for _, agent := range dueList {
		// 防止同一 agent 被并发重复执行：
		// ExecuteAgent 异步执行，期间 NextRunAt 未更新，
		// 下一个 ticker tick 会再次把该 agent 加入 dueList。
		// 用 runningAgents 标记确保同一 agent 同时只有一个执行。
		svc.runningAgentsMu.Lock()
		if svc.runningAgents[agent.Id] {
			svc.runningAgentsMu.Unlock()
			svc.Logger.Info("Agent already running, skip", "id", agent.Id, "name", agent.Name)
			continue
		}
		svc.runningAgents[agent.Id] = true
		svc.runningAgentsMu.Unlock()

		svc.Logger.Info("Agent due, executing", "id", agent.Id, "name", agent.Name)
		go func(a *model.AiAgent) {
			defer func() {
				svc.runningAgentsMu.Lock()
				delete(svc.runningAgents, a.Id)
				svc.runningAgentsMu.Unlock()
			}()
			if _, err := svc.ExecuteAgent(a); err != nil {
				svc.Logger.Error("Agent execution failed", "id", a.Id, "error", err)
			}
		}(agent)
	}
}

// ExecuteAgent 执行 Agent 的任务（非流式 LLM 调用 + 工具循环）
func (svc *AiChatService) ExecuteAgent(agent *model.AiAgent) (finalResponse string, errResult error) {
	// 防重入：如果该 agent 已在执行中，直接返回，避免并发重复执行。
	// 这覆盖了调度器 tick 与手动 agent_run 并发的场景。
	svc.runningAgentsMu.Lock()
	if svc.runningAgents[agent.Id] {
		svc.runningAgentsMu.Unlock()
		return "", fmt.Errorf("agent #%d is already running", agent.Id)
	}
	svc.runningAgents[agent.Id] = true
	svc.runningAgentsMu.Unlock()
	defer func() {
		svc.runningAgentsMu.Lock()
		delete(svc.runningAgents, agent.Id)
		svc.runningAgentsMu.Unlock()
	}()

	// ── 断点续跑检查 (P1-4) ──
	// 进程崩溃/重启后再被调度触发时，runningAgents 守卫已随进程消失，
	// 但 DB 里若残留 status=Running 的 checkpoint，说明上一次执行没正常收尾，
	// 从 checkpoint.Round 继续（会话历史已持久化此前所有轮次的消息）。
	startRound := 0
	resumed := false
	if svc.db != nil {
		var cp model.AiAgentCheckpoint
		if err := svc.db.Where("agent_id = ? AND status = ?", agent.Id, model.AgentCheckpointRunning).
			Order("id DESC").First(&cp).Error; err == nil {
			resumed = true
			startRound = resumeRoundFromCheckpoint(&cp)
			slog.Info("resuming agent from checkpoint", "agentId", agent.Id, "round", startRound)
		}
	}

	// 创建执行日志
	logEntry := &model.AiAgentLog{
		AgentId:   agent.Id,
		SessionId: agent.SessionId,
		Status:    0, // 执行中
		StartedAt: time.Now().Unix(),
	}
	if svc.db != nil {
		svc.db.Create(logEntry)
	}

	updateLog := func(status int, summary, errMsg string) {
		logEntry.Status = status
		logEntry.Summary = summary
		logEntry.Error = errMsg
		logEntry.FinishedAt = time.Now().Unix()
		// P4: 把执行失败的诊断信息记入近期错误缓冲，供 autoDiagnoseErrors 回灌模型
		if status == 2 && errMsg != "" {
			RecordAIError("agent[%s] 执行失败: %s", agent.Name, errMsg)
		}
		if svc.db != nil {
			svc.db.Save(logEntry)
		}
	}

	// 确保 Agent 状态在任何退出路径下都更新 NextRunAt，防止重复执行
	defer func() {
		if finalResponse == "" {
			finalResponse = "执行结束，未获取到总结（可能已达到最大轮次或发生错误）。"
		}
		svc.agentsMu.Lock()
		agent.LastRunAt = time.Now().Unix()
		agent.LastSummary = finalResponse
		agent.RunCount++
		// 计算下次执行时间
		if agent.CronExpr != "" {
			scheduler, err := cronParser.Parse(agent.CronExpr)
			if err == nil {
				agent.NextRunAt = scheduler.Next(time.Now()).Unix()
			}
		}
		if agent.MaxRuns > 0 && agent.RunCount >= agent.MaxRuns {
			agent.Enabled = 0
		}
		svc.agentsMu.Unlock()

		// 持久化到 DB
		if svc.db != nil {
			svc.db.Model(&model.AiAgent{}).Where("id = ?", agent.Id).Updates(map[string]interface{}{
				"last_run_at":  agent.LastRunAt,
				"last_summary": finalResponse,
				"run_count":    agent.RunCount,
				"next_run_at":  agent.NextRunAt,
				"enabled":      agent.Enabled,
			})
		}
	}()

	// 构建系统提示
	systemPrompt := `你是一个 AnQiCMS 的 AI 智能体，需要独立完成任务。

## 任务策略
` + agent.Strategy + `

## 规则
1. 每次执行时，按照策略自主调用工具完成工作，不要询问用户意见
2. 执行完每个修改操作后，需要验证结果
3. 全部完成后，用中文总结你做了什么、结果如何`

	// 获取 Eino client
	client, err := eino.GetClient()
	if err != nil {
		updateLog(2, "", err.Error())
		return "", fmt.Errorf("AI client not available: %w", err)
	}

	// 绑定工具
	if len(svc.Tools) > 0 {
		if err := client.BindTools(svc.Tools); err != nil {
			updateLog(2, "", err.Error())
			return "", fmt.Errorf("failed to bind tools: %w", err)
		}
	}

	// 构建消息：历史上下文 + 执行指令
	messages := svc.BuildToolMessages(agent.SessionId, systemPrompt)
	triggerMsg := "现在开始执行任务。执行完毕后用中文总结。"
	messages = append(messages, schema.UserMessage(triggerMsg))

	maxRounds := agent.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 20 // 未配置时默认 20 轮
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 断点续跑：每完成一轮就落盘 round，进程被杀后下次触发可从此续跑。
	saveCheckpoint := func(round int, status int) {
		if svc.db == nil {
			return
		}
		rec := model.AiAgentCheckpoint{
			AgentId:   agent.Id,
			SessionId: agent.SessionId,
			Round:     round,
			MaxRounds: maxRounds,
			Status:    status,
		}
		var existing model.AiAgentCheckpoint
		if err := svc.db.Where("agent_id = ? AND status = ?", agent.Id, model.AgentCheckpointRunning).
			First(&existing).Error; err == nil {
			svc.db.Model(&existing).Updates(map[string]interface{}{
				"round":      round,
				"max_rounds": maxRounds,
				"status":     status,
			})
		} else {
			svc.db.Create(&rec)
		}
	}
	if !resumed {
		// 全新执行：先清掉任何历史终态残留，写入初始 Running 记录
		saveCheckpoint(0, model.AgentCheckpointRunning)
	}

	// 函数退出（正常结束或出错）时把 Running 的 checkpoint 置为终态，
	// 避免下次调度误判为「需要续跑」。
	defer func() {
		if svc.db == nil {
			return
		}
		var cp model.AiAgentCheckpoint
		if err := svc.db.Where("agent_id = ? AND status = ?", agent.Id, model.AgentCheckpointRunning).
			First(&cp).Error; err == nil {
			status := model.AgentCheckpointDone
			if errResult != nil {
				status = model.AgentCheckpointFailed
			}
			svc.db.Model(&cp).Update("status", status)
		}
	}()

	for round := startRound; round < maxRounds; round++ {
		// 检查超时
		select {
		case <-ctx.Done():
			updateLog(2, "", ctx.Err().Error())
			return "", ctx.Err()
		default:
		}

		// P2-7: 结构化 tracing —— 记录 Agent 每轮 LLM 调用
		if svc.trace != nil {
			svc.trace.RecordPhase(TraceAgent, "agent_round",
				fmt.Sprintf("agent #%d, round %d (resumed=%v)", agent.Id, round, resumed))
		}

		// 调用 LLM
		msg, err := client.Generate(ctx, messages)
		if err != nil {
			if IsContextOverflowError(err) {
				messages = CompactMessages(messages, 5)
				continue
			}
			updateLog(2, "", err.Error())
			return "", fmt.Errorf("AI generate failed: %w", err)
		}

		// 无工具调用 → 本轮为最终回复
		if len(msg.ToolCalls) == 0 {
			finalResponse = msg.Content
			break
		}

		messages = append(messages, msg)

		// 保存 assistant 消息到会话历史
		toolCallsJSON, _ := json.Marshal(msg.ToolCalls)
		svc.AddMessage(agent.SessionId, ChatMessage{
			Role:      "assistant",
			Content:   msg.Content,
			ToolCalls: string(toolCallsJSON),
		})

		// 执行每个工具
		for _, tc := range msg.ToolCalls {
			toolName := tc.Function.Name
			argsJSON := tc.Function.Arguments

			handler, exists := svc.Handlers[toolName]
			var result string
			if !exists {
				result = fmt.Sprintf("错误：未知工具 %s", toolName)
			} else {
				result, err = handler(ctx, argsJSON)
				if err != nil {
					// P4: 工具执行失败也记入近期错误缓冲
					result = fmt.Sprintf("工具执行错误: %s", err.Error())
					RecordAIError("agent[%s] 工具 %s 执行失败: %s", agent.Name, toolName, err.Error())
				}
			}

			toolMsg := schema.ToolMessage(result, tc.ID)
			messages = append(messages, toolMsg)
			logEntry.ToolCalls++

			// 保存工具结果到会话历史
			svc.AddMessage(agent.SessionId, ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				ToolName:   toolName,
			})
		}

		// 上下文压缩：每 3 轮压缩一次
		if len(messages) > 12 && round%3 == 2 {
			messages = CompactMessages(messages, 5)
		}

		// 断点续跑：本轮回合已完成，记录已完成轮数
		saveCheckpoint(round+1, model.AgentCheckpointRunning)
	}

	// 达到最大轮次时，LLM 仍在调用工具，设置默认响应
	if finalResponse == "" {
		finalResponse = fmt.Sprintf("已达到最大执行轮次（%d轮），任务可能未完全完成。", maxRounds)
	}

	updateLog(1, finalResponse, "")
	return finalResponse, nil
}

// GetAgent 根据 ID 获取 Agent
func (svc *AiChatService) GetAgent(id uint) *model.AiAgent {
	svc.agentsMu.RLock()
	defer svc.agentsMu.RUnlock()
	return svc.agents[id]
}

// Trace 返回结构化追踪记录器 (P2-7)。可能为 nil（极早期构造分支）。
func (svc *AiChatService) Trace() *TraceRecorder {
	return svc.trace
}

// resumeRoundFromCheckpoint 把 DB 里记录的「已完成轮数」转换为本次循环起始轮。
// 负数或异常值回落到 0（fail-safe，避免越界导致跳过 system prompt 或访问越界）。
func resumeRoundFromCheckpoint(cp *model.AiAgentCheckpoint) int {
	if cp == nil {
		return 0
	}
	if cp.Round < 0 {
		return 0
	}
	return cp.Round
}

// GetAgentBySessionID 根据 SessionID 查找 Agent
func (svc *AiChatService) GetAgentBySessionID(sessionID string) *model.AiAgent {
	svc.agentsMu.RLock()
	defer svc.agentsMu.RUnlock()
	for _, agent := range svc.agents {
		if agent.SessionId == sessionID {
			return agent
		}
	}
	return nil
}

// GetEinoTools returns all Eino tool definitions and their handlers, including
// built-in tools. This exposes the full tool set so callers (e.g. the chat
// controller) can bind every available tool to the model up-front.
func (svc *AiChatService) GetEinoTools() ([]*schema.ToolInfo, map[string]toolHandler) {
	tools, handlers := svc.getEinoTools()
	builtinTools, builtinHandlers := svc.getBuiltinEinoTools()
	tools = append(tools, builtinTools...)
	for name, handler := range builtinHandlers {
		handlers[name] = handler
	}
	return tools, handlers
}

// ResolveCap 取出可供意图委托调用的底层能力 handler。
//
// 必须优先查 capHandlers：它只含端点与内置 caps，不含意图 handler。
// 直接查 Handlers 存在真实风险 —— Handlers 里同名意图会覆盖同名 cap（web_fetch/web_search），
// 委托时会再次命中意图 handler 自身而形成无限递归。MCP 通道的 capInvoker 同样依赖本方法。
func (svc *AiChatService) ResolveCap(name string) (toolHandler, bool) {
	if svc == nil {
		return nil, false
	}
	if svc.capHandlers != nil {
		if h, ok := svc.capHandlers[name]; ok {
			return h, true
		}
	}
	h, ok := svc.Handlers[name]
	return h, ok
}

// GetIntentTools 返回意图导向工作流工具（与 MCP 共用 IntentCatalog），仅构建一次并缓存。
// 意图内核通过 CapInvoker 委托给本服务的端点 handler（底层能力 caps）；意图 handler 单独成表，
// 与端点 handler 分离，避免同名意图（如 web_fetch）委托到同名 cap 时产生递归。
func (svc *AiChatService) GetIntentTools() ([]*schema.ToolInfo, map[string]toolHandler) {
	if svc.intentTools != nil {
		return svc.intentTools, svc.intentHandlers
	}
	iconf := intent.Config{
		ExposedIntents: GetMcpConfig().ExposedIntents,
		ExposedTools:   GetMcpConfig().ExposedTools,
		ToolListMode:   GetMcpConfig().ToolListMode,
	}
	// CapInvoker：把意图参数序列化后调用对应底层能力 handler（端点/内置 caps）。读取 svc.capHandlers
	// 当前值（纯底层能力表，不含意图 handler），以支持运行期热重建且避免同名意图递归。
	capInvoker := func(ctx context.Context, name string, args map[string]any) (string, error) {
		h, ok := svc.ResolveCap(name)
		if !ok {
			return "", fmt.Errorf("底层能力 %s 不存在或未接入", name)
		}
		raw, err := json.Marshal(args)
		if err != nil {
			return "", fmt.Errorf("参数序列化失败: %w", err)
		}
		return h(ctx, string(raw))
	}
	auditFn := func(ctx context.Context, name, risk, argsJSON string, callErr error, start time.Time) {
		if svc.site != nil {
			svc.site.recordMcpAudit(ctx, name, risk, argsJSON, callErr, start)
		}
	}
	kernel := intent.NewKernel(iconf, capInvoker, auditFn)
	tools, handlers, _ := kernel.BuildEinoTools(iconf)
	out := make(map[string]toolHandler, len(handlers))
	for n, h := range handlers {
		out[n] = toolHandler(h)
	}
	svc.intentTools = tools
	svc.intentHandlers = out
	return svc.intentTools, svc.intentHandlers
}

// ── P4a: 技能 allowed-tools 约束 ──

// SetSkillToolScope 激活技能的工具约束。
// allowedTools 为空切片时清空约束 (所有工具可用)。
// allowedTools 非空时，只能使用该集合内的工具 + 始终允许的元工具。
func (svc *AiChatService) SetSkillToolScope(allowedTools []string) {
	svc.activeSkillMu.Lock()
	defer svc.activeSkillMu.Unlock()

	if len(allowedTools) == 0 {
		svc.activeSkillTools = nil
		return
	}

	svc.activeSkillTools = make(map[string]bool, len(allowedTools)+4)
	// 始终允许的元工具 (技能管理本身不应被锁)
	metaTools := []string{"skill_list", "skill_get", "skill_reload", "skill_save"}
	for _, t := range metaTools {
		svc.activeSkillTools[t] = true
	}
	for _, t := range allowedTools {
		svc.activeSkillTools[t] = true
	}
}

// IsToolAllowed 检查工具是否在当前技能的 allowed-tools 范围内。
// 无激活技能时始终返回 true。
func (svc *AiChatService) IsToolAllowed(toolName string) bool {
	svc.activeSkillMu.RLock()
	defer svc.activeSkillMu.RUnlock()
	if svc.activeSkillTools == nil {
		return true
	}
	return svc.activeSkillTools[toolName]
}

// ClearSkillToolScope 清除技能工具约束 (技能执行结束后调用)。
func (svc *AiChatService) ClearSkillToolScope() {
	svc.activeSkillMu.Lock()
	defer svc.activeSkillMu.Unlock()
	svc.activeSkillTools = nil
}

// GetAllTools returns all available MCP tools, built from the Eino tool definitions.
func (svc *AiChatService) GetAllTools() []*mcp.Tool {
	toolInfos, _ := svc.getEinoTools()
	// Also include built-in tools
	builtinInfos, _ := svc.getBuiltinEinoTools()
	toolInfos = append(toolInfos, builtinInfos...)
	tools := make([]*mcp.Tool, 0, len(toolInfos))
	for _, ti := range toolInfos {
		tools = append(tools, &mcp.Tool{
			Name:        ti.Name,
			Description: ti.Desc,
		})
	}
	return tools
}

// buildAIResponse builds an AI response based on the user message
func (svc *AiChatService) BuildAIResponse(message string, toolNames []string) string {
	msg := strings.ToLower(message)

	// 简单的基于关键词的路由规则
	// 生产环境中会使用大语言模型（LLM）

	if containsAny(msg, []string{"help", "tool", "capability"}) {
		response := "可用的 AnQiCMS MCP 工具：\n\n"
		response += formatTools(toolNames)
		return response
	}

	// 默认回复
	response := "你好！我是您的 AnQiCMS AI 助手，可以帮助您管理文章、分类、标签和附件。\n\n"
	if svc.site != nil {
		response += fmt.Sprintf("当前站点：%s\n\n", svc.site.System.SiteName)
	}
	response += "输入 'help' 查看可用的工具和命令。\n"
	return response
}

// BuildToolMessages builds the message array for tool-calling conversations
// Uses smart windowing with turn-aware compression: keeps recent turns full
// and compresses older turns into a summary. Uses TurnTracker to ensure
// tool_call ↔ tool_result pairs are never split across the compaction boundary.
func (svc *AiChatService) BuildToolMessages(sessionID string, systemPrompt string) []*schema.Message {
	var messages []*schema.Message

	sess := svc.GetOrCreateSession(sessionID)

	// Build the message list with turn-aware compaction
	compactHistory := CompactMessagesFromChat(sess, systemPrompt, 5)
	messages = append(messages, compactHistory...)

	return messages
}

// CompactMessagesFromChat converts session chat history to schema messages
// with turn-aware smart windowing. It keeps the last keepTurns number of turns
// full and compresses older turn groups into a system message summary.
// The original system prompt is always prepended first.
//
// Turn-aware boundary: ensures tool_call ↔ tool_result pairs are never split.
// Tool results are condensed by type (read_file→skeleton, bash→first line, etc.)
func CompactMessagesFromChat(sess *ChatSession, systemPrompt string, keepTurns int) []*schema.Message {
	var messages []*schema.Message
	messages = append(messages, schema.SystemMessage(systemPrompt))

	if len(sess.Messages) == 0 {
		return messages
	}

	// Use the session's TurnTracker (rebuilt after compression, or fresh from DB load)
	turns := sess.Turns
	if len(turns) == 0 {
		// Fallback: rebuild inline
		rebuildTurns(sess)
		turns = sess.Turns
	}
	if len(turns) == 0 {
		return messages
	}

	// Determine how many turns we keep in full from the tail.
	// Only keep turns that start wholly after any compressed turns.
	var keepStartIdx int
	if len(turns) <= keepTurns {
		// No compression needed — keep all messages
		// 两遍扫描确保 assistant(tool_calls) ↔ tool 消息正确配对:
		//   第一遍: 收集所有 assistant 声明的 tool_call_id (declaredIDs)
		//           和所有 tool 消息的 tool_call_id (existingToolIDs)
		//   第二遍: 按顺序构建 messages, 跳过孤立的 tool 消息和孤立的 assistant(tool_calls)
		// 这能正确处理 DB 中 tool 消息 created_time 早于 assistant 消息的乱序情况
		declaredIDs := make(map[string]bool)     // assistant 声明的 tool_call_id
		existingToolIDs := make(map[string]bool) // 实际存在的 tool 消息的 tool_call_id
		for _, msg := range sess.Messages {
			if msg.Role == "assistant" && msg.ToolCalls != "" {
				var tcs []schema.ToolCall
				if json.Unmarshal([]byte(msg.ToolCalls), &tcs) == nil {
					for _, tc := range tcs {
						declaredIDs[tc.ID] = true
					}
				}
			}
			if msg.Role == "tool" {
				existingToolIDs[msg.ToolCallID] = true
			}
		}
		for _, msg := range sess.Messages {
			if msg.Role == "tool" {
				// 孤立的 tool 消息（无对应 assistant tool_calls）会导致 400 Bad Request
				if !declaredIDs[msg.ToolCallID] {
					continue
				}
				messages = append(messages, schema.ToolMessage(condenseToolResult(msg), msg.ToolCallID))
			} else if msg.Role == "assistant" && msg.ToolCalls != "" {
				// assistant 带 tool_calls: 只保留有对应 tool 消息的 tool_call_id
				var tcs []schema.ToolCall
				if json.Unmarshal([]byte(msg.ToolCalls), &tcs) != nil {
					messages = append(messages, chatMessageToSchema(msg))
					continue
				}
				var kept []schema.ToolCall
				for _, tc := range tcs {
					if existingToolIDs[tc.ID] {
						kept = append(kept, tc)
					}
				}
				if len(kept) == 0 && len(tcs) > 0 {
					// 所有 tool_call 都没有对应 tool 消息 → 丢弃 tool_calls，只保留 content
					messages = append(messages, schema.AssistantMessage(msg.Content, nil))
				} else if len(kept) < len(tcs) {
					// 部分 tool_call 没有对应 tool 消息 → 只保留有配对的
					messages = append(messages, schema.AssistantMessage(msg.Content, kept))
				} else {
					messages = append(messages, schema.AssistantMessage(msg.Content, tcs))
				}
			} else {
				messages = append(messages, chatMessageToSchema(msg))
			}
		}
		return messages
	}

	keepStartTurn := len(turns) - keepTurns
	keepStartIdx = turns[keepStartTurn].StartIdx

	// ── Phase 1: Compress older turns (turn 0 … keepStartTurn-1) ──
	// Build a concise summary as a system message
	var summaryParts []string
	for ti := 0; ti < keepStartTurn; ti++ {
		turn := turns[ti]
		turnEnd := turn.StartIdx + turn.MsgCount
		if turnEnd > len(sess.Messages) {
			turnEnd = len(sess.Messages)
		}
		turnMsgs := sess.Messages[turn.StartIdx:turnEnd]
		summaryParts = append(summaryParts, condenseTurn(turnMsgs))
	}

	summary := "[历史对话摘要]\n" + strings.Join(summaryParts, "\n---\n")
	messages = append(messages, schema.SystemMessage(summary))

	// ── Phase 2: Keep the last keepTurns turns in full, with condensed tool results ──
	// 两遍扫描 (同 "no compression" 分支): 先收集 declaredIDs / existingToolIDs,
	// 再按顺序构建 messages, 确保每个 assistant(tool_calls) 都有对应的 tool 消息
	declaredIDs := make(map[string]bool)
	existingToolIDs := make(map[string]bool)
	for i := keepStartIdx; i < len(sess.Messages); i++ {
		msg := sess.Messages[i]
		if msg.Role == "assistant" && msg.ToolCalls != "" {
			var tcs []schema.ToolCall
			if json.Unmarshal([]byte(msg.ToolCalls), &tcs) == nil {
				for _, tc := range tcs {
					declaredIDs[tc.ID] = true
				}
			}
		}
		if msg.Role == "tool" {
			existingToolIDs[msg.ToolCallID] = true
		}
	}
	for i := keepStartIdx; i < len(sess.Messages); i++ {
		msg := sess.Messages[i]
		if msg.Role == "tool" {
			// 孤立的 tool 消息（无对应 assistant tool_calls）会导致 400 Bad Request
			if !declaredIDs[msg.ToolCallID] {
				continue
			}
			messages = append(messages, schema.ToolMessage(condenseToolResult(msg), msg.ToolCallID))
		} else if msg.Role == "assistant" && msg.ToolCalls != "" {
			var tcs []schema.ToolCall
			if json.Unmarshal([]byte(msg.ToolCalls), &tcs) != nil {
				messages = append(messages, chatMessageToSchema(msg))
				continue
			}
			var kept []schema.ToolCall
			for _, tc := range tcs {
				if existingToolIDs[tc.ID] {
					kept = append(kept, tc)
				}
			}
			if len(kept) == 0 && len(tcs) > 0 {
				messages = append(messages, schema.AssistantMessage(msg.Content, nil))
			} else if len(kept) < len(tcs) {
				messages = append(messages, schema.AssistantMessage(msg.Content, kept))
			} else {
				messages = append(messages, schema.AssistantMessage(msg.Content, tcs))
			}
		} else {
			messages = append(messages, chatMessageToSchema(msg))
		}
	}

	return messages
}

// chatMessageToSchema converts a ChatMessage to a schema.Message.
func chatMessageToSchema(msg ChatMessage) *schema.Message {
	if msg.Role == "user" {
		return schema.UserMessage(msg.Content)
	} else if msg.Role == "assistant" {
		if msg.ToolCalls != "" {
			var toolCalls []schema.ToolCall
			if err := json.Unmarshal([]byte(msg.ToolCalls), &toolCalls); err == nil {
				return schema.AssistantMessage(msg.Content, toolCalls)
			}
		}
		return schema.AssistantMessage(msg.Content, nil)
	} else if msg.Role == "tool" {
		return schema.ToolMessage(msg.Content, msg.ToolCallID)
	}
	// Fallback: treat as user message
	return schema.UserMessage(msg.Content)
}

// condenseTurn condenses an entire turn (user+assistant+tool messages) into
// a single-line summary for the compressed history block.
func condenseTurn(msgs []ChatMessage) string {
	if len(msgs) == 0 {
		return ""
	}
	// Extract user message
	userContent := ""
	for _, m := range msgs {
		if m.Role == "user" {
			userContent = truncate(m.Content, 150)
			break
		}
	}
	// Collect tool names used
	toolNames := make([]string, 0)
	for _, m := range msgs {
		if m.Role == "tool" && m.ToolName != "" {
			toolNames = append(toolNames, m.ToolName)
		}
	}
	// Find assistant final response
	assistantContent := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			assistantContent = truncate(msgs[i].Content, 200)
			break
		}
	}

	var parts []string
	if userContent != "" {
		parts = append(parts, "用户: "+userContent)
	}
	if len(toolNames) > 0 {
		parts = append(parts, "工具: "+strings.Join(uniqueStrings(toolNames), ", "))
	}
	if assistantContent != "" {
		parts = append(parts, "AI: "+assistantContent)
	}
	return strings.Join(parts, " | ")
}

// condenseToolResult condenses a tool result based on the tool type.
// Follows atomcode's condensed() pattern (message.rs:162-202):
// - read_file: compress to skeleton (extract signatures/imports)
// - bash: keep first 2 lines
// - Others: truncate to 200 chars, append "..."
func condenseToolResult(msg ChatMessage) string {
	content := msg.Content
	if len(content) <= 200 {
		return content
	}
	switch msg.ToolName {
	case "read_file":
		return CompressFileToSkeleton(content)
	case "bash", "shell":
		// Keep first 2 lines
		lines := strings.SplitN(content, "\n", 3)
		if len(lines) <= 2 {
			return content
		}
		return strings.Join(lines[:2], "\n") + "\n..."
	default:
		return truncate(content, 200)
	}
}

// truncate truncates a string to maxChars runes, appending "..." if truncated.
func truncate(s string, maxChars int) string {
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars]) + "..."
}

// uniqueStrings returns deduplicated strings preserving order.
func uniqueStrings(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// signatureKeywords lists line prefixes that indicate a structural declaration.
// Matches atomcode's compress_file_to_skeleton (message.rs:249-279).
var signatureKeywords = []string{
	"fn ", "pub fn ", "async fn ", "pub async fn ",
	"def ", "class ", "function ", "func ",
	"export ", "import ", "const ", "let ",
	"public ", "private ", "protected ",
	"interface ", "type ", "struct ", "enum ", "impl ",
	"package ", "use ", "from ", "#include",
}

// isSignatureLine checks whether a trimmed content line looks like a structural
// declaration at indent 0-2.
func isSignatureLine(content string) bool {
	if len(content) == 0 {
		return false
	}
	indent := 0
	for _, c := range content {
		if c == ' ' || c == '\t' {
			indent++
		} else {
			break
		}
	}
	if indent > 2 {
		return false
	}
	trimmed := strings.TrimSpace(content)
	for _, kw := range signatureKeywords {
		if strings.HasPrefix(trimmed, kw) {
			return true
		}
	}
	// Decorators / attributes
	if strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, "#[") {
		return true
	}
	// Vue/HTML template markers
	if trimmed == "<template>" || trimmed == "</template>" ||
		trimmed == "<script>" || trimmed == "</script>" ||
		trimmed == "<style>" || trimmed == "</style>" ||
		strings.HasPrefix(trimmed, "<template ") ||
		strings.HasPrefix(trimmed, "<script ") ||
		strings.HasPrefix(trimmed, "<style ") {
		return true
	}
	return false
}

// CompressFileToSkeleton extracts structural signatures from a read_file output
// (numbered lines). Returns ~10-20% of the original content but preserves
// function/class/import structure for the LLM. Falls back to first line + count
// if no signatures are found.
// Matches atomcode's compress_file_to_skeleton (message.rs:243-325).
func CompressFileToSkeleton(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return output
	}

	totalLines := 0
	var skeleton []string

	for _, line := range lines {
		// Parse "文件: path (N 行, M 字节)" header
		if strings.HasPrefix(line, "文件:") {
			skeleton = append(skeleton, line)
			continue
		}
		// Parse "  N| content" numbered lines
		content := line
		if len(line) > 7 {
			// Try to strip "%6d| " prefix (6 digit + "| ")
			rest := strings.TrimLeft(line, " ")
			if len(rest) >= 2 && rest[0] >= '0' && rest[0] <= '9' {
				// Find the "| " separator
				if idx := strings.Index(rest, "| "); idx >= 0 && idx <= 6 {
					totalLines++
					content = rest[idx+2:]
				}
			}
		}
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			continue
		}
		if isSignatureLine(content) {
			skeleton = append(skeleton, strings.TrimRight(line, " "))
		}
	}

	if len(skeleton) <= 1 { // Header only or nothing
		first := lines[0]
		if len(lines) > 1 {
			return first + fmt.Sprintf(" (%d 行)", len(lines))
		}
		return first
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[文件骨架 — %d 行，可使用 edit_file 编辑]\n", totalLines))
	for _, s := range skeleton {
		b.WriteString(s)
		b.WriteByte('\n')
	}
	return b.String()
}

// Helper functions

func formatTools(tools []string) string {
	result := ""
	for _, tool := range tools {
		result += tool + "\n"
	}
	return result
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// IsRateLimitError checks if the error indicates a rate limit hit.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	patterns := []string{
		"rate limit",
		"rate_limit",
		"too many requests",
		"throttle",
		"429",
	}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// IsContextOverflowError checks if the error is related to context window exceeding.
func IsContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	patterns := []string{
		"context length exceeded",
		"too many tokens",
		"max tokens exceeded",
		"prompt too long",
	}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// HasWriteOperation checks if any of the tool names is a write operation.
//
// 注意这是**工具名粒度**的粗判定（拿不到 arguments 的调用点才用它）。
// 名单不再在本文件硬编码：风险统一由意图层按
// 「声明的路由 → 底层端点方法 → 动词约定 → 意图声明」推导（见 pkg/mcp/intent/risk.go），
// 需要审批的集合是 {write, destructive, system}。
// 早先这里只认 spec.Risk==write，导致 RiskSystem 的 shell/文件写意图（shell_exec、
// fs_write、fs_edit、fs_replace）完全绕过审批门。
func HasWriteOperation(toolNames []string) bool {
	for _, name := range toolNames {
		if intent.NeedsApproval(intent.ActionRisk(strings.TrimSpace(name), nil)) {
			return true
		}
	}
	return false
}

// CallNeedsApproval 判断这一次**具体调用**要不要人工确认。
//
// 合并意图（如 content_article 的 list/get/save/delete）整体标 Risk=write，
// 按工具名判定会让纯查询也弹窗（用户反馈的问题 2）。这里下钻到 action 粒度：
// content_article action=list → 只读，不问；action=delete → 问。
func CallNeedsApproval(toolName, argsJSON string) bool {
	return intent.ActionNeedsApproval(toolName, argsJSON)
}

// CallRisk 返回一次调用的推导风险，供审批事件向用户展示"为什么问"。
func CallRisk(toolName, argsJSON string) string {
	return string(intent.ActionRiskFromArgsJSON(toolName, argsJSON))
}

// ContainsErrorKeywords checks if the string contains common error keywords.
func ContainsErrorKeywords(s string) bool {
	lower := strings.ToLower(s)
	patterns := []string{
		"error",
		"exception",
		"traceback",
		"panic",
	}
	chinesePatterns := []string{
		"错误",
		"异常",
		"报错",
		"失败",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	for _, p := range chinesePatterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// CompactMessages compresses older messages in a schema.Message slice into a
// [系统压缩] summary system message. It keeps the first (system) message and the
// last keepCount messages intact, summarizing everything in between.
// Uses safe boundary detection: never splits tool_call ↔ tool_result pairs.
// Anti-nesting: skips results with already-compressed summaries (does NOT
// re-summarize a [系统压缩] summary, instead replaces it in place).
// compactCut 计算压缩区间，返回 (可压缩区起始下标, 安全切点)：
// [compressedPrefix, safeCut) 之间的消息会被压缩，safeCut 之后原样保留。
//
// 两处要点：
//   - Anti-nesting：若已存在压缩摘要（messages[1] 带 [系统压缩] 标记），
//     起始下标后移到 2，避免摘要套摘要层层膨胀。
//   - Safe cut：切点不能落在 tool_call / tool_result 配对中间，
//     否则会产生「有 tool_call 却没有结果」的非法消息序列，模型 API 会直接报错。
func compactCut(messages []*schema.Message, keepCount int) (int, int) {
	if len(messages) <= keepCount+1 {
		return 1, 1
	}

	compressedPrefix := 1 // 默认：可压缩区从下标 1 开始（0 是 system prompt）
	if len(messages) > 2 && messages[1].Role == schema.System &&
		(strings.Contains(messages[1].Content, "[系统压缩]") ||
			strings.Contains(messages[1].Content, "[历史对话摘要]")) {
		compressedPrefix = 2 // 跳过旧摘要
	}

	targetCut := len(messages) - keepCount
	if targetCut < compressedPrefix {
		targetCut = compressedPrefix
	}
	safeCut := targetCut

	isSafe := func(m *schema.Message) bool {
		return m.Role == schema.User || (m.Role == schema.Assistant && len(m.ToolCalls) == 0)
	}

	// 从 targetCut 向前扫，找一个安全边界
	for i := targetCut; i >= compressedPrefix; i-- {
		if isSafe(messages[i]) {
			safeCut = i
			break
		}
	}
	// 向前没找到就向后扫
	if safeCut == targetCut {
		for i := targetCut + 1; i < len(messages); i++ {
			if isSafe(messages[i]) {
				safeCut = i
				break
			}
		}
	}

	// ── API 合规性校验 ──
	// 保留区的第一条不能是孤儿 Tool 结果，也不能是带 tool_calls 的 assistant
	// （它们的配对消息留在了压缩区里）。是的话就继续推进到下一个安全边界。
	for safeCut < len(messages) {
		msg := messages[safeCut]
		if msg.Role == schema.Tool {
			safeCut++
			for safeCut < len(messages) {
				if isSafe(messages[safeCut]) {
					break
				}
				safeCut++
			}
		} else if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
			safeCut++
			for safeCut < len(messages) {
				if messages[safeCut].Role != schema.Tool {
					break
				}
				safeCut++
			}
		} else {
			break
		}
	}

	return compressedPrefix, safeCut
}

// messagesContentLen 统计消息列表的总字符数，用于净损失守卫。
func messagesContentLen(msgs []*schema.Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Content)
	}
	return n
}

// Summarizer 语义压缩时用于生成摘要的 LLM 调用抽象。
// 由 controller 注入真实模型客户端；为 nil 或调用失败时回落到截断式压缩，
// 保证压缩能力不会因为模型不可用而整体失效。
type Summarizer interface {
	Summarize(ctx context.Context, prompt string) (string, error)
}

// SummarizerFunc 让普通函数直接实现 Summarizer（省去为一次性适配写结构体）。
type SummarizerFunc func(ctx context.Context, prompt string) (string, error)

func (f SummarizerFunc) Summarize(ctx context.Context, prompt string) (string, error) {
	return f(ctx, prompt)
}

// CompactMessagesWithSummary 语义压缩 (P1-5)。
//
// 原 CompactMessages 把中间每条消息暴力截断到 150 rune —— 严格说那是**丢信息**
// 而不是压缩：长任务跑到后段，模型会忘记自己改过哪些文件、为什么那么改，
// 于是重复劳动甚至改错地方。
//
// 这里改为让模型生成一段语义摘要，并加两道保护：
//   - 回落：Summarizer 为 nil / 调用出错 / 返回空 → 退回 CompactMessages 的截断行为
//   - 净损失守卫：摘要后若总长度没有明显变短（模型把原文照抄回来了），
//     判定本次压缩无效并退回截断方案 —— 仿 atomcode CompactReport.committed==false
//     的「拒绝提交」语义，避免"压了个寂寞"还白花一次调用。
func CompactMessagesWithSummary(ctx context.Context, messages []*schema.Message, keepCount int, sum Summarizer) []*schema.Message {
	fallback := func() []*schema.Message { return CompactMessages(messages, keepCount) }
	if sum == nil || len(messages) == 0 {
		return fallback()
	}

	compressedPrefix, safeCut := compactCut(messages, keepCount)
	if safeCut <= compressedPrefix {
		// 没有可压缩区间
		return fallback()
	}

	// 把待压缩区间渲染成待摘要文本（每条先做一次硬截断，防止单条超大）
	var sb strings.Builder
	for i := compressedPrefix; i < safeCut && i < len(messages); i++ {
		m := messages[i]
		content := m.Content
		if runes := []rune(content); len(runes) > 800 {
			content = string(runes[:800]) + "…"
		}
		sb.WriteString(string(m.Role) + ": " + content + "\n")
	}

	prompt := "请把下面这段 AI 与工具的对话历史压缩成一段简洁的中文摘要。" +
		"必须保留：用户最初的目标、已经完成的实际操作（尤其是创建/修改/删除了哪些文件或数据）、" +
		"关键结论与约束、尚未完成的事项、以及任何报错与处理方式。" +
		"不要保留逐字的工具输出原文，不要加入摘要之外的评论。控制在 400 字以内。\n\n" +
		sb.String()

	summaryText, err := sum.Summarize(ctx, prompt)
	if err != nil || strings.TrimSpace(summaryText) == "" {
		return fallback()
	}

	// 组装：system prompt + 语义摘要 + 保留区
	compacted := make([]*schema.Message, 0, 2+len(messages)-safeCut)
	compacted = append(compacted, messages[0])
	compacted = append(compacted, schema.SystemMessage(
		"[系统压缩] 以下为历史对话摘要（语义压缩）:\n"+strings.TrimSpace(summaryText)))
	if safeCut < len(messages) {
		compacted = append(compacted, messages[safeCut:]...)
	}

	// ── 净损失守卫 ──
	//
	// 在「原文 / 截断方案 / 语义方案」三者里选最短的那个，并且：
	//   - 两个压缩方案都没能比原文更短 → 干脆不压缩。
	//     消息很短时，"role: " 前缀与 "---" 分隔符会让截断结果**反而变长**，
	//     这时压缩只是往上下文里塞噪声（这是截断方案一直存在的老问题）。
	//   - 语义方案若没有比截断方案更短，说明模型没在摘要（把原文照抄回来了），
	//     采用截断方案 —— 仿 atomcode CompactReport.committed==false 的拒绝提交语义。
	//
	// 注意次序：必须先比出三者中最短的再决定，
	// 不能先拿截断方案和原文比 —— 否则真正更短的语义摘要会永远轮不到。
	truncated := CompactMessages(messages, keepCount)

	best := compacted
	bestLen := messagesContentLen(compacted)
	if tl := messagesContentLen(truncated); tl < bestLen {
		best, bestLen = truncated, tl
	}
	if messagesContentLen(messages) <= bestLen {
		return messages
	}
	return best
}

func CompactMessages(messages []*schema.Message, keepCount int) []*schema.Message {
	if len(messages) <= keepCount+1 {
		return messages
	}

	// First message is the system prompt — always keep it
	systemMsg := messages[0]

	// 压缩区间由 compactCut 统一计算（ truncation 与语义摘要两条路径共用）：
	//   - Anti-nesting：跳过已有的压缩摘要，避免摘要套摘要
	//   - Safe cut：切点不能落在 tool_call / tool_result 配对中间
	compressedPrefix, safeCut := compactCut(messages, keepCount)

	// ── Compress messages between compressedPrefix and safeCut ──
	var summaryParts []string
	hasOldSummaryKept := false
	for i := compressedPrefix; i < safeCut; i++ {
		msg := messages[i]
		// Already-compressed summary in the middle — keep its content directly
		// instead of re-truncating it (anti-nesting)
		if msg.Role == schema.System &&
			(strings.Contains(msg.Content, "[系统压缩]") ||
				strings.Contains(msg.Content, "[历史对话摘要]")) {
			summaryParts = append(summaryParts, msg.Content)
			hasOldSummaryKept = true
			continue
		}
		role := string(msg.Role)
		content := msg.Content
		runeContent := []rune(content)
		if len(runeContent) > 150 {
			content = string(runeContent[:150]) + "..."
		}
		summaryParts = append(summaryParts, role+": "+content)
	}

	// Determine whether to keep the old summary or create a new one.
	// If we kept an old summary directly and it already contains all the info,
	// prepend a brief note rather than nesting another [系统压缩] layer.
	var compacted []*schema.Message
	compacted = append(compacted, systemMsg)

	if hasOldSummaryKept {
		// Old summary already includes the relevant history — just add a brief
		// context note and proceed
		compacted = append(compacted, schema.SystemMessage(
			"[系统压缩] 以下为历史对话摘要，包含上方压缩的历史信息"))
	} else if len(summaryParts) > 0 {
		summary := "[系统压缩] 以下为历史对话摘要:\n" + strings.Join(summaryParts, "\n---\n")
		compacted = append(compacted, schema.SystemMessage(summary))
	}

	// Keep from safeCut to end (may be empty if safeCut >= len(messages))
	if safeCut < len(messages) {
		compacted = append(compacted, messages[safeCut:]...)
	}

	return compacted
}
