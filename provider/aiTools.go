package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/ai/eino"
)

type ArgId struct {
	Id int64 `json:"id"`
}

// toolHandler executes a tool given its JSON arguments and returns a text result.
type toolHandler func(ctx context.Context, argsJSON string) (string, error)

// getEinoTools returns tool definitions (schema.ToolInfo) and a name→handler map.
// The handlers use the site stored in the service.
func (svc *AiChatService) getEinoTools() ([]*schema.ToolInfo, map[string]toolHandler) {
	tools := make([]*schema.ToolInfo, 0)
	handlers := make(map[string]toolHandler)

	add := func(ti *schema.ToolInfo, fn toolHandler) {
		tools = append(tools, ti)
		handlers[ti.Name] = fn
	}
	add(&schema.ToolInfo{
		Name:        "template_reload",
		Desc:        "重新加载模板。在修改了模板文件内容或切换模板后，需要调用此工具使更改生效。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil {
			return "错误：站点未初始化", nil
		}
		config.RestartChan <- config.RestartConfig{Code: 0, SiteId: w.Id}
		data := map[string]any{
			"message":   "模板重载信号已发送",
			"template":  w.System.TemplateName,
			"reload_in": "1秒",
		}
		return makeJSONEnvelope(0, data, "模板重载信号已发送，模板将在1秒内重新加载", true, 200)
	})
	add(&schema.ToolInfo{
		Name: "attachment_upload",
		Desc: "上传附件（图片）到站点，支持三种方式（三选一）：1. base64参数，传入图片的base64编码内容或data URI（推荐，客户端本地文件或AI生成的图片先编码为base64再上传）；2. url参数传远程URL；3. url参数传服务器本地文件路径（仅当文件已在CMS服务器上时有效，通常是AI聊天上传按钮上传的临时文件路径file_path）。注意：MCP客户端本地路径在服务器上不存在，请使用base64方式。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"base64":    {Type: schema.String, Desc: "图片的base64编码内容，支持裸base64字符串或data URI格式（data:image/png;base64,xxx），与url二选一"},
			"url":       {Type: schema.String, Desc: "图片的远程URL地址，或本地文件路径（绝对路径或基于站点项目目录的相对路径），与base64二选一"},
			"file_name": {Type: schema.String, Desc: "保存的文件名（不含扩展名），可选"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Base64   string `json:"base64"`
			URL      string `json:"url"`
			FileName string `json:"file_name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Base64 == "" && args.URL == "" {
			return "错误：base64 和 url 至少提供一个", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		// base64 内容优先
		if args.Base64 != "" {
			attachment, err := svc.uploadAttachmentFromBase64(args.Base64, args.FileName)
			if err != nil {
				return "", fmt.Errorf("上传附件失败: %w", err)
			}
			attachment.GetThumb(w.PluginStorage.StorageUrl)
			data := map[string]any{
				"id":        attachment.Id,
				"file_name": attachment.FileName,
				"url":       attachment.Logo,
			}
			return makeJSONEnvelope(0, data, "附件上传成功", true, 200)
		}
		var attachment *model.Attachment
		// 判断是否为本地文件路径
		if strings.HasPrefix(args.URL, "http://") || strings.HasPrefix(args.URL, "https://") {
			// 远程URL下载
			var err error
			attachment, err = w.DownloadRemoteImage(args.URL, args.FileName, 0)
			if err != nil {
				return "", fmt.Errorf("上传附件失败: %w", err)
			}
		} else {
			// 本地文件路径
			localPath := args.URL
			if !filepath.IsAbs(localPath) {
				localPath = filepath.Join(svc.projectRoot, localPath)
			}
			// 安全检查：防止路径遍历
			localPath, err := filepath.Abs(localPath)
			if err != nil {
				return "", fmt.Errorf("无法解析路径: %w", err)
			}
			if svc.projectRoot != "" && !strings.HasPrefix(localPath, svc.projectRoot) {
				return "", fmt.Errorf("路径超出项目目录范围: %s", localPath)
			}
			// 打开文件
			file, err := os.Open(localPath)
			if err != nil {
				return "", fmt.Errorf("无法打开文件: %w", err)
			}
			defer file.Close()
			stat, err := file.Stat()
			if err != nil {
				return "", fmt.Errorf("无法获取文件信息: %w", err)
			}
			fileName := args.FileName
			if fileName == "" {
				fileName = strings.TrimSuffix(stat.Name(), filepath.Ext(stat.Name()))
			}
			fileHeader := &multipart.FileHeader{
				Filename: stat.Name(),
				Size:     stat.Size(),
			}
			attachment, err = w.AttachmentUpload(file, fileHeader, 0, 0, 0)
			if err != nil {
				return "", fmt.Errorf("上传附件失败: %w", err)
			}
		}
		attachment.GetThumb(w.PluginStorage.StorageUrl)
		data := map[string]any{
			"id":        attachment.Id,
			"file_name": attachment.FileName,
			"url":       attachment.Logo,
		}
		return makeJSONEnvelope(0, data, "附件上传成功", true, 200)
	})
	add(&schema.ToolInfo{
		Name: "agent_create",
		Desc: "创建一个 AI 智能体（Agent），拥有独立的会话和记忆，可定时执行任务。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name":       {Type: schema.String, Desc: "智能体名称，如 '每日热词写作'", Required: true},
			"strategy":   {Type: schema.String, Desc: "执行策略。描述每次执行时需要做什么，包括步骤、标准、输出要求。如 '每天搜索互联网热词，据此写3篇文章并发布'", Required: true},
			"cron":       {Type: schema.String, Desc: "Cron 表达式，如 '0 8 * * *' 表示每天8点执行。留空表示仅手动触发"},
			"max_runs":   {Type: schema.Integer, Desc: "最大执行次数，0=不限（默认0）"},
			"max_rounds": {Type: schema.Integer, Desc: "单次执行最大轮数，0=用默认20（默认0）"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Name      string `json:"name"`
			Strategy  string `json:"strategy"`
			Cron      string `json:"cron"`
			MaxRuns   int    `json:"max_runs"`
			MaxRounds int    `json:"max_rounds"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Name == "" || args.Strategy == "" {
			return "错误：名称和策略不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}

		agent := &model.AiAgent{
			Name:      args.Name,
			Strategy:  args.Strategy,
			CronExpr:  args.Cron,
			MaxRuns:   args.MaxRuns,
			MaxRounds: args.MaxRounds,
			Enabled:   1,
			SessionId: "", // 创建后分配
		}
		if err := w.DB.Create(agent).Error; err != nil {
			return "", fmt.Errorf("创建智能体失败: %w", err)
		}
		// 分配专属会话 ID
		agent.SessionId = fmt.Sprintf("agent_%d", agent.Id)
		w.DB.Model(agent).Update("session_id", agent.SessionId)

		// 如果有 cron 表达式，计算首次执行时间
		if agent.CronExpr != "" {
			scheduler, err := cronParser.Parse(agent.CronExpr)
			if err == nil {
				agent.NextRunAt = scheduler.Next(time.Now()).Unix()
				w.DB.Model(agent).Update("next_run_at", agent.NextRunAt)
			}
		}

		// 加入内存缓存
		svc.agentsMu.Lock()
		svc.agents[agent.Id] = agent
		svc.agentsMu.Unlock()

		data := map[string]any{
			"id":         agent.Id,
			"name":       agent.Name,
			"strategy":   agent.Strategy,
			"cron":       agent.CronExpr,
			"max_rounds": args.MaxRounds,
			"session_id": agent.SessionId,
		}
		return makeJSONEnvelope(0, data, fmt.Sprintf("智能体创建成功！ID: %d", agent.Id), true, 200)
	})
	add(&schema.ToolInfo{
		Name: "agent_edit",
		Desc: "修改一个已存在的 AI 智能体（Agent）的配置：名称、策略、Cron、最大执行次数、单次最大轮数、启用状态。只需传要修改的字段，其余保持不变。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":         {Type: schema.Integer, Desc: "智能体ID", Required: true},
			"name":       {Type: schema.String, Desc: "智能体名称"},
			"strategy":   {Type: schema.String, Desc: "执行策略描述"},
			"cron":       {Type: schema.String, Desc: "Cron 表达式，传空字符串可清空（仅手动触发）"},
			"max_runs":   {Type: schema.Integer, Desc: "最大执行次数，0=不限"},
			"max_rounds": {Type: schema.Integer, Desc: "单次执行最大轮数，0=用默认20"},
			"enabled":    {Type: schema.Integer, Desc: "1=启用 0=暂停"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		// 用指针字段区分「未传」与「显式传 0」，否则无法把 max_runs=0 / enabled=0 这种合法值写入。
		var args struct {
			Id        uint    `json:"id"`
			Name      *string `json:"name"`
			Strategy  *string `json:"strategy"`
			Cron      *string `json:"cron"`
			MaxRuns   *int    `json:"max_runs"`
			MaxRounds *int    `json:"max_rounds"`
			Enabled   *int    `json:"enabled"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id == 0 {
			return "错误：ID必须大于0", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		agent := &model.AiAgent{}
		if err := w.DB.Where("id = ?", args.Id).First(agent).Error; err != nil {
			return "错误：智能体不存在", nil
		}

		updates := map[string]interface{}{}
		if args.Name != nil {
			updates["name"] = *args.Name
			agent.Name = *args.Name
		}
		if args.Strategy != nil {
			updates["strategy"] = *args.Strategy
			agent.Strategy = *args.Strategy
		}
		if args.Cron != nil {
			updates["cron_expr"] = *args.Cron
			agent.CronExpr = *args.Cron
			if *args.Cron == "" {
				agent.NextRunAt = 0
			} else if sched, perr := cronParser.Parse(*args.Cron); perr == nil {
				agent.NextRunAt = sched.Next(time.Now()).Unix()
			}
			updates["next_run_at"] = agent.NextRunAt
		}
		if args.MaxRuns != nil {
			updates["max_runs"] = *args.MaxRuns
			agent.MaxRuns = *args.MaxRuns
		}
		if args.MaxRounds != nil {
			updates["max_rounds"] = *args.MaxRounds
			agent.MaxRounds = *args.MaxRounds
		}
		if args.Enabled != nil {
			updates["enabled"] = *args.Enabled
			agent.Enabled = *args.Enabled
		}
		if len(updates) == 0 {
			return "未提供任何要修改的字段", nil
		}
		if err := w.DB.Model(agent).Updates(updates).Error; err != nil {
			return "", fmt.Errorf("更新智能体失败: %w", err)
		}
		// 同步内存缓存，使调度器立即看到新配置（含 Cron / Enabled / NextRunAt）
		svc.agentsMu.Lock()
		svc.agents[agent.Id] = agent
		svc.agentsMu.Unlock()

		enabledStr := "✅ 运行中"
		if agent.Enabled == 0 {
			enabledStr = "⏸ 已暂停"
		}
		roundsStr := "默认(20)"
		if agent.MaxRounds > 0 {
			roundsStr = fmt.Sprintf("%d", agent.MaxRounds)
		}
		cronStr := agent.CronExpr
		if cronStr == "" {
			cronStr = "仅手动"
		}
		return fmt.Sprintf("智能体 #%d 已更新\n名称: %s\n策略: %s\nCron: %s\n最大执行次数: %d\n单次最大轮数: %s\n状态: %s",
			agent.Id, agent.Name, agent.Strategy, cronStr, agent.MaxRuns, roundsStr, enabledStr), nil
	})
	add(&schema.ToolInfo{
		Name:        "agent_list",
		Desc:        "查看所有 AI 智能体列表，包含状态、上次运行时间、运行次数。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		var agents []model.AiAgent
		w.DB.Order("id ASC").Find(&agents)
		if len(agents) == 0 {
			data := map[string]any{"count": 0}
			return makeJSONEnvelope(0, data, "暂无智能体。使用 agent_create 创建一个。", true, 200)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "共 %d 个智能体：\n\n", len(agents))
		for _, a := range agents {
			enabledStr := "✅ 运行中"
			if a.Enabled == 0 {
				enabledStr = "⏸ 已暂停"
			}
			lastRun := "从未"
			if a.LastRunAt > 0 {
				lastRun = time.Unix(a.LastRunAt, 0).Format("2006-01-02 15:04")
			}
			cronStr := a.CronExpr
			if cronStr == "" {
				cronStr = "仅手动"
			}
			fmt.Fprintf(&b, "#%d | %s | %s\n", a.Id, a.Name, enabledStr)
			fmt.Fprintf(&b, "  策略: %s\n", truncate(a.Strategy, 100))
			fmt.Fprintf(&b, "  Cron: %s | 上次: %s | 运行: %d次\n", cronStr, lastRun, a.RunCount)
		}
		return b.String(), nil
	})
	add(&schema.ToolInfo{
		Name: "agent_delete",
		Desc: "删除指定 AI 智能体及其所有执行日志。不可恢复。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "智能体ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id <= 0 {
			return "错误：ID必须大于0", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		var agent model.AiAgent
		if err := w.DB.Where("id = ?", args.Id).First(&agent).Error; err != nil {
			return "错误：智能体不存在", nil
		}
		// 删除执行日志
		w.DB.Where("agent_id = ?", args.Id).Delete(&model.AiAgentLog{})
		// 删除 Agent
		w.DB.Delete(&agent)
		// 从内存缓存移除
		svc.agentsMu.Lock()
		delete(svc.agents, uint(args.Id))
		svc.agentsMu.Unlock()
		data := map[string]any{"id": args.Id}
		return makeJSONEnvelope(0, data, fmt.Sprintf("智能体 #%d 已删除", args.Id), true, 200)
	})
	add(&schema.ToolInfo{
		Name: "agent_toggle",
		Desc: "暂停或恢复一个 AI 智能体。暂停后不会定时执行。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":      {Type: schema.Integer, Desc: "智能体ID", Required: true},
			"enabled": {Type: schema.Integer, Desc: "1=启用(运行中)，0=暂停", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Id      int `json:"id"`
			Enabled int `json:"enabled"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id <= 0 {
			return "错误：ID必须大于0", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		var agent model.AiAgent
		if err := w.DB.Where("id = ?", args.Id).First(&agent).Error; err != nil {
			return "错误：智能体不存在", nil
		}
		enabled := 0
		if args.Enabled == 1 {
			enabled = 1
		}
		// 重新启用时，重新计算 NextRunAt，防止 NextRunAt=0 导致永不执行
		if enabled == 1 && agent.CronExpr != "" {
			scheduler, err := cronParser.Parse(agent.CronExpr)
			if err == nil {
				agent.NextRunAt = scheduler.Next(time.Now()).Unix()
				w.DB.Model(&agent).Update("next_run_at", agent.NextRunAt)
			}
		}
		w.DB.Model(&agent).Update("enabled", enabled)
		svc.agentsMu.Lock()
		if existing, ok := svc.agents[uint(args.Id)]; ok {
			existing.Enabled = enabled
			if enabled == 1 {
				existing.NextRunAt = agent.NextRunAt
			}
		} else if enabled == 1 {
			// 如果内存中没有该 agent，加入内存
			agent.Enabled = enabled
			svc.agents[uint(args.Id)] = &agent
		}
		svc.agentsMu.Unlock()
		status := "已暂停"
		if enabled == 1 {
			status = "运行中"
		}
		data := map[string]any{
			"id":      args.Id,
			"enabled": enabled == 1,
		}
		return makeJSONEnvelope(0, data, fmt.Sprintf("智能体 #%d 状态已更改为: %s", args.Id, status), true, 200)
	})
	add(&schema.ToolInfo{
		Name: "agent_run",
		Desc: "手动触发一个 AI 智能体立即执行一次任务。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {Type: schema.Integer, Desc: "智能体ID", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args ArgId
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id <= 0 {
			return "错误：ID必须大于0", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		svc.agentsMu.RLock()
		agent, exists := svc.agents[uint(args.Id)]
		svc.agentsMu.RUnlock()
		if !exists {
			return "错误：智能体不存在或未加载", nil
		}
		result, err := svc.ExecuteAgent(agent)
		if err != nil {
			data := map[string]any{"agent_id": args.Id, "error": err.Error()}
			return makeJSONEnvelope(1, data, fmt.Sprintf("智能体 #%d 执行失败", args.Id), false, 500)
		}
		data := map[string]any{
			"agent_id": args.Id,
			"result":   result,
		}
		return makeJSONEnvelope(0, data, fmt.Sprintf("智能体 #%d 执行完成", args.Id), true, 200)
	})
	add(&schema.ToolInfo{
		Name: "agent_chat",
		Desc: "与指定 AI 智能体的专属会话对话。可以查看它的执行历史、调整策略、询问进度等。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id":      {Type: schema.Integer, Desc: "智能体ID", Required: true},
			"message": {Type: schema.String, Desc: "发送给智能体的消息", Required: true},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Id      int    `json:"id"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if args.Id <= 0 || args.Message == "" {
			return "错误：ID和消息不能为空", nil
		}
		w := svc.site
		if w == nil || w.DB == nil {
			return "错误：站点未初始化", nil
		}
		svc.agentsMu.RLock()
		agent, exists := svc.agents[uint(args.Id)]
		svc.agentsMu.RUnlock()
		if !exists {
			return "错误：智能体不存在或未加载", nil
		}

		// 获取 Eino client
		client, err := eino.GetClient()
		if err != nil {
			return "", fmt.Errorf("AI client not available: %w", err)
		}
		if len(svc.Tools) > 0 {
			if err := client.BindTools(svc.Tools); err != nil {
				return "", fmt.Errorf("failed to bind tools: %w", err)
			}
		}

		// 构建消息：Agent 的历史上下文 + 用户消息
		systemPrompt := `你是 AnQiCMS 的 AI 智能体。以下是你的策略和对话历史。
用户正在与你对话，请回答问题并可以执行工具。`

		if agent.Strategy != "" {
			systemPrompt += "\n\n## 你的策略\n" + agent.Strategy
		}
		if agent.LastSummary != "" {
			systemPrompt += "\n\n## 上次执行摘要\n" + agent.LastSummary
		}

		messages := svc.BuildToolMessages(agent.SessionId, systemPrompt)
		messages = append(messages, schema.UserMessage(args.Message))

		// 非流式调用，单轮回复
		msg, err := client.Generate(ctx, messages)
		if err != nil {
			return "", fmt.Errorf("AI generate failed: %w", err)
		}

		// 保存到会话历史
		svc.AddMessage(agent.SessionId, ChatMessage{
			Role:    "user",
			Content: args.Message,
		})
		svc.AddMessage(agent.SessionId, ChatMessage{
			Role:    "assistant",
			Content: msg.Content,
		})

		return msg.Content, nil
	})
	add(skillListTool())
	add(skillGetTool())
	add(skillReloadTool())
	add(skillSaveTool())

	// ── SkillHub 技能仓库工具 ──
	add(&schema.ToolInfo{
		Name: "skill_search",
		Desc: "在 SkillHub 技能仓库 (https://skillhub.cn) 搜索技能。" +
			"当本地没有合适的技能时，使用此工具在线搜索。" +
			"找到后用 skill_install 安装。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {Type: schema.String, Required: true, Desc: "搜索关键词 (如 pdf, code review)"},
			"limit": {Type: schema.Integer, Desc: "返回结果上限 (1-50, 默认 10)"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		resp, err := SearchSkillHub(ctx, args.Query, args.Limit)
		if err != nil {
			return fmt.Sprintf("搜索失败: %s", err.Error()), nil
		}
		return FormatSkillHubSearchResults(resp, args.Query), nil
	})

	add(&schema.ToolInfo{
		Name: "skill_install",
		Desc: "从 SkillHub 技能仓库下载并安装技能。" +
			"下载 SKILL.md zip 包并解压到全局技能目录，安装后可用 skill_get 加载。" +
			"force=true 覆盖已安装的同名技能。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"slug":  {Type: schema.String, Required: true, Desc: "SkillHub 技能 slug (如 find-skills)"},
			"force": {Type: schema.Boolean, Desc: "覆盖已安装技能 (默认 false)"},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Slug  string `json:"slug"`
			Force bool   `json:"force"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		info, err := InstallSkillFromSkillHub(ctx, args.Slug, args.Force)
		return FormatSkillHubInstallResult(info, err), nil
	})

	// ── P7: Subagent/Team 工具 ──
	add(&schema.ToolInfo{
		Name: "task",
		Desc: "派发并行子任务 (仿 atomcode `task` 工具)。" +
			"每个子任务在独立上下文中执行，结果汇总到主对话。" +
			"explore 子任务只读 (调查/搜索)，worker 子任务可写但需声明 scope (允许写的文件范围)，scope 非重叠。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"tasks": {
				Type: schema.Array, Required: true,
				Desc: "子任务列表",
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Object,
					SubParams: map[string]*schema.ParameterInfo{
						"description": {Type: schema.String, Required: true, Desc: "3-5 词任务标签"},
						"prompt":      {Type: schema.String, Required: true, Desc: "子任务完整指令"},
						"type":        {Type: schema.String, Required: true, Desc: "explore (只读) 或 worker (可写)"},
						"scope":       {Type: schema.Array, Desc: "worker 允许写的文件 scope (globs)"},
					},
				},
			},
		}),
	}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Tasks []struct {
				Description string   `json:"description"`
				Prompt      string   `json:"prompt"`
				Type        string   `json:"type"`
				Scope       []string `json:"scope"`
			} `json:"tasks"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("无法解析参数: %w", err)
		}
		if len(args.Tasks) == 0 {
			return "错误：至少需要一个子任务", nil
		}
		if len(args.Tasks) > 10 {
			return "错误：一次最多派发 10 个子任务", nil
		}

		// 构建 SubagentTask 列表
		tasks := make([]*SubagentTask, 0, len(args.Tasks))
		for i, ts := range args.Tasks {
			taskID := fmt.Sprintf("task_%d_%d", time.Now().UnixNano(), i+1)
			subType := SubagentExplore
			if ts.Type == "worker" {
				subType = SubagentWorker
			} else if ts.Type != "explore" {
				return fmt.Sprintf("错误：子任务 %d 的 type 必须是 explore 或 worker", i+1), nil
			}

			// worker 必须声明 scope
			if subType == SubagentWorker && len(ts.Scope) == 0 {
				return fmt.Sprintf("错误：worker 子任务 %d 必须声明 scope", i+1), nil
			}

			tasks = append(tasks, &SubagentTask{
				ID:          taskID,
				Description: ts.Description,
				Prompt:      ts.Prompt,
				Type:        subType,
				Scope:       ts.Scope,
				MaxRounds:   10,
				Timeout:     5 * time.Minute,
			})
		}

		// 并行执行所有子任务
		taskCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		results := svc.DispatchTasks(taskCtx, tasks)

		// 格式化汇总结果（按子代理蒸馏上限回传，避免污染主上下文）
		return FormatSubagentResults(results, DefaultSubagentDistillMaxRunes), nil
	})

	return tools, handlers
}

// makeJSONEnvelope 构造 api_invoke 格式的 JSON 字符串
func makeJSONEnvelope(code int, data any, msg string, ok bool, status int) (string, error) {
	envelope := map[string]any{
		"code":   code,
		"data":   data,
		"msg":    msg,
		"ok":     ok,
		"status": status,
	}
	jsonBytes, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// uploadAttachmentFromBase64 将 base64 编码的图片内容（裸 base64 或 data URI）保存为站点附件。
// 写入临时文件后复用 AttachmentUpload，从而继承水印、压缩、缩略图、md5 去重等处理逻辑。
func (svc *AiChatService) uploadAttachmentFromBase64(base64Str string, fileName string) (*model.Attachment, error) {
	w := svc.site
	if w == nil || w.DB == nil {
		return nil, fmt.Errorf("站点未初始化")
	}
	base64Str = strings.TrimSpace(base64Str)
	// 解析 data URI：data:image/png;base64,xxxx
	ext := ""
	if strings.HasPrefix(base64Str, "data:") {
		idx := strings.Index(base64Str, ",")
		if idx <= 0 {
			return nil, fmt.Errorf("无效的 data URI 格式")
		}
		meta := base64Str[5:idx] // 如 image/png;base64
		base64Str = strings.TrimSpace(base64Str[idx+1:])
		if parts := strings.SplitN(meta, "/", 2); len(parts) == 2 {
			ext = strings.ToLower(strings.Split(parts[1], ";")[0])
			ext = strings.ReplaceAll(ext, "+xml", "")
		}
	}
	// 只保留字母数字，避免 svg+xml 之类的异常扩展名
	var extBuf strings.Builder
	for _, c := range ext {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			extBuf.WriteRune(c)
		}
	}
	ext = extBuf.String()

	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		// 兼容无填充与 URL-safe 变体
		if d, err2 := base64.RawStdEncoding.DecodeString(base64Str); err2 == nil {
			data, err = d, nil
		} else if d, err2 := base64.URLEncoding.DecodeString(base64Str); err2 == nil {
			data, err = d, nil
		} else if d, err2 := base64.RawURLEncoding.DecodeString(base64Str); err2 == nil {
			data, err = d, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("base64解码失败: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("解码后的文件内容为空")
	}
	if len(data) > 30*1024*1024 {
		return nil, fmt.Errorf("文件过大，base64 上传最大支持 30MB")
	}

	if fileName == "" {
		fileName = "base64-image"
	}
	tmpFile, err := os.CreateTemp("", "anqi-upload-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("写入临时文件失败: %w", err)
	}
	tmpFile.Close()

	f, err := os.Open(tmpName)
	if err != nil {
		return nil, fmt.Errorf("打开临时文件失败: %w", err)
	}
	defer f.Close()
	fileHeader := &multipart.FileHeader{
		Filename: fileName + ext,
		Size:     int64(len(data)),
	}
	return w.AttachmentUpload(f, fileHeader, 0, 0, 0)
}
