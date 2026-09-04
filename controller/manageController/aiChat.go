package manageController

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/kataras/iris/v12"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/ai/eino"
	"kandaoni.com/anqicms/provider"
)

// ChatRequest represents an AI chat request
type ChatRequest struct {
	SessionID   string        `json:"session_id"`
	Message     string        `json:"message"`
	Model       string        `json:"model"`
	Files       []ChatFileRef `json:"files,omitempty"`
	IframeURL   string        `json:"iframe_url,omitempty"`   // 前端 AI 编辑器：当前 iframe 页面 URL
	SelectedDOM string        `json:"selected_dom,omitempty"` // 前端 AI 编辑器：管理员选中的 DOM 片段
}

// ChatFileRef represents a reference to an uploaded file
type ChatFileRef struct {
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	FileType string `json:"file_type"` // attachment|template
}

// ChatResponse represents an AI chat response
type ChatResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// Chat handles the chat request and returns SSE stream with AI response
func AiChat(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	if currentSite.AiSrv == nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "ai service not available",
		})
		return
	}
	var req ChatRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "invalid request",
		})
		return
	}

	// ── 兼容旧 AuthAiChat 接口的纯 prompt 请求 ──
	// 旧接口请求格式为 AnqiAiRequest{Prompt: "..."}，无 Message/SessionID 字段。
	// 当 ChatRequest.Message 为空时，回退尝试解析为 AnqiAiRequest，
	// 用 Prompt 填充 Message，并进入 purePromptMode:
	//   - 跳过工具绑定 (无 Tools/Handlers)
	//   - 跳过 session 管理 (不写入会话历史)
	//   - 跳过文件附件/编辑器上下文处理
	//   - 直接流式返回 AI 响应
	purePromptMode := false
	if req.Message == "" {
		// 重新读取原始请求体 (ctx.ReadJSON 已消费 body，但 iris 缓存了它)
		var oldReq provider.AnqiAiRequest
		if err2 := ctx.ReadJSON(&oldReq); err2 == nil && oldReq.Prompt != "" {
			req.Message = oldReq.Prompt
			purePromptMode = true
		}
	}

	if req.Message == "" {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "message cannot be empty",
		})
		return
	}

	// Set SSE headers
	ctx.ContentType("text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	writer := ctx.ResponseWriter()

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
	}

	slog.Debug("event: session", "data:", sessionID)
	// Send session ID first
	fmt.Fprintf(writer, "event: session\ndata: %s\n\n", sessionID)
	writer.Flush()

	defaultSite := provider.CurrentSite(nil)

	oldCfg := eino.GlobalConfig()
	if req.Model != "" {
		tplIdx, found := strings.CutPrefix(req.Model, "custom:")
		if found {
			aiSetting := defaultSite.LoadAiSetting("")
			idx, _ := strconv.Atoi(tplIdx)
			if idx >= 0 && idx < len(aiSetting.Configs) {
				cfg := aiSetting.Configs[idx]
				if oldCfg == nil || cfg.APIKey != oldCfg.APIKey {
					// 配置已更新，重新设置AI接口
					slog.Info("AI client initialized with updated config")
					if err := eino.SetGlobalConfig(cfg); err != nil {
						slog.Error("Failed to initialize AI client with provided config", "error", err)
						sendSSEWarning(writer, "AI配置无效: "+err.Error())
						return
					}
					// 配置成功
					oldCfg = cfg
					aiSetting.LastModel = req.Model
					defaultSite.SaveSettingValue(provider.AiSettingKey, aiSetting)
				}
			}
		} else {
			// 选择的是官方模型
			if config.AnqiUser.AuthId > 0 {
				if req.Model != "anqi-flash" && req.Model != "anqi-pro" {
					req.Model = "anqi-flash"
				}
				if req.Model != oldCfg.Model {
					// 配置已更新，重新设置AI接口
					if err := eino.SetOfficialConfig(req.Model); err != nil {
						slog.Error("Failed to initialize AI client", "error", err)
					} else {
						slog.Info("AI client initialized successfully")
					}
					aiSetting := defaultSite.LoadAiSetting("")
					aiSetting.LastModel = req.Model
					defaultSite.SaveSettingValue(provider.AiSettingKey, aiSetting)
				}
			} else {
				sendSSEWarning(writer, "请先绑定安企账号，后开始使用AI助手")
			}
		}
	}

	// 设置AI
	if oldCfg == nil {
		// 未检测到有效配置，返回 JSON 模板提示
		sendSSEWarning(writer, "AI接口尚未配置或配置错误。")
		fmt.Fprintf(writer, "event: config\ndata: %s\n\n", "{}")
		writer.Flush()
		return
	}

	// ── Step 1: 接收与诊断 ──
	// 负面反馈检测: 如果用户消息较短且包含负面关键词，标记为不满
	message := req.Message

	// ── purePromptMode: 旧 AuthAiChat 兼容路径 ──
	// 跳过工具绑定 / 文件附件 / 编辑器上下文 / 会话历史，
	// 直接流式返回 AI 对 prompt 的响应。
	if purePromptMode {
		// 解耦 context
		aiCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		_, pErr := generatePurePromptResponse(aiCtx, message, writer)
		if pErr != nil {
			slog.Error("Pure prompt response failed", "error", pErr)
		}

		slog.Debug("event: end", "data:", "[DONE]")
		if writer != nil {
			fmt.Fprintf(writer, "event: end\ndata: [DONE]\n\n")
			writer.Flush()
		}
		return
	}

	negativeKeywords := []string{"不对", "错了", "不行", "还是不行", "没用", "不是这样", "搞错",
		"又错", "白做", "越改越差", "恢复", "回滚", "撤销",
		"wrong", "not right", "still broken", "doesn't work", "undo", "revert", "go back"}
	isNegative := false
	if len([]rune(message)) < 80 {
		lowerMsg := strings.ToLower(message)
		for _, kw := range negativeKeywords {
			if strings.Contains(lowerMsg, kw) || strings.Contains(message, kw) {
				isNegative = true
				break
			}
		}
	}
	if isNegative {
		// 附加诊断提示，让模型回顾之前的操作
		message += "\n\n[系统诊断: 检测到你可能对之前的回答不满意。请回顾之前的操作，仔细检查是否有错误，然后重新处理。如果需要回滚或撤销，请明确说明。]"
	}

	// 自动诊断: 如果用户消息包含错误关键词，扫描日志附加错误信息
	if provider.ContainsErrorKeywords(message) {
		diagInfo := autoDiagnoseErrors()
		if diagInfo != "" {
			message += "\n\n[系统发现以下可能的错误信息]:\n" + diagInfo
		}
	}

	// 处理上传的文件附件：读取内容并追加到用户消息
	if len(req.Files) > 0 {
		var fileParts []string
		for _, f := range req.Files {
			fullPath := filepath.Join(currentSite.RootPath, f.FilePath)
			info, err := os.Stat(fullPath)
			if err != nil || info.IsDir() {
				continue
			}
			// 通过 MIME type 判断文件类型
			ext := strings.ToLower(filepath.Ext(f.FileName))
			mimeType := mime.TypeByExtension(ext)
			if mimeType == "" {
				// 扩展名未知时，读取文件开头内容推断是否为文本
				header := make([]byte, 2048)
				fh, err := os.Open(fullPath)
				if err == nil {
					n, _ := fh.Read(header)
					fh.Close()
					if n > 0 {
						header = header[:n]
						// 不含空字节且UTF-8可解码 → 视为文本
						if !bytes.Contains(header, []byte{0}) {
							mimeType = "text/plain"
						} else {
							mimeType = "application/octet-stream"
						}
					}
				}
			}

			if f.FileType == "template" || strings.HasPrefix(mimeType, "text/") ||
				strings.HasSuffix(mimeType, "+xml") ||
				mimeType == "application/json" ||
				mimeType == "application/javascript" ||
				mimeType == "application/xml" ||
				mimeType == "application/x-yaml" ||
				mimeType == "application/x-sh" ||
				mimeType == "application/sql" {
				// 文本文件：读取内容
				data, err := os.ReadFile(fullPath)
				if err != nil {
					continue
				}
				content := string(data)
				if len([]rune(content)) > 8000 {
					content = string([]rune(content)[:8000]) + "\n... [文件过长，仅显示前8000字符]"
				}
				if f.FileType == "template" {
					fileParts = append(fileParts, fmt.Sprintf("[模板文件: %s](本地路径: %s)\n---\n%s\n---", f.FileName, f.FilePath, content))
				} else {
					fileParts = append(fileParts, fmt.Sprintf("[文件: %s](本地路径: %s)\n---\n%s\n---", f.FileName, f.FilePath, content))
				}
			} else if strings.HasPrefix(mimeType, "image/") {
				fileParts = append(fileParts, fmt.Sprintf("[图片: %s] (%.1f KB, 本地路径: %s)", f.FileName, float64(info.Size())/1024, f.FilePath))
			} else {
				fileParts = append(fileParts, fmt.Sprintf("[附件: %s] (%.1f KB, 本地路径: %s, 类型: %s)", f.FileName, float64(info.Size())/1024, f.FilePath, mimeType))
			}
		}
		if len(fileParts) > 0 {
			message = strings.Join(fileParts, "\n\n") + "\n\n" + message
		}
	}

	// Add user message to history
	var chatFiles []provider.ChatFileRef
	if len(req.Files) > 0 {
		for _, f := range req.Files {
			chatFiles = append(chatFiles, provider.ChatFileRef{
				FileName: f.FileName,
				FilePath: f.FilePath,
			})
		}
	}

	// 前端 AI 编辑器上下文构建
	var editorContext string
	tplName, resolveErr := currentSite.ResolveTemplateFromURL(req.IframeURL, ctx.GetHeader("Admin"))
	if resolveErr != nil {
		slog.Warn("Failed to resolve template from URL", "error", resolveErr)
	}
	editorContext = fmt.Sprintf("[前端AI编辑器上下文]\n模板文件夹: %s", currentSite.GetTemplateDir())
	if req.IframeURL != "" {
		editorContext += fmt.Sprintf("\n当前页面URL: %s", req.IframeURL)
	}
	if tplName != "" {
		editorContext += fmt.Sprintf("\n当前模板: %s", tplName)
	}
	if req.SelectedDOM != "" {
		editorContext += fmt.Sprintf("\n用户选中的页面DOM片段:\n%s", req.SelectedDOM)
	}
	// // 读取主模板内容并注入，方便 AI 直接修改
	// if tplName != "" {
	// 	if tplContent, ok := currentSite.GetTemplate(tplName); ok {
	// 		if len([]rune(tplContent)) > 8000 {
	// 			tplContent = string([]rune(tplContent)[:8000]) + "\n... [模板过长，仅显示前8000字符]"
	// 		}
	// 		editorContext += fmt.Sprintf("\n主模板 %s 的内容:\n%s", tplName, tplContent)
	// 	}
	// }

	currentSite.AiSrv.AddMessage(sessionID, provider.ChatMessage{
		Role:    "user",
		Content: message,
		Files:   chatFiles,
	})

	// 解耦 context: 不绑定 HTTP 请求生命周期，避免 SSE 空闲超时导致 context canceled
	aiCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Generate AI response — try DeepSeek first, fall back to keyword matching
	response, err := generateAIResponse(aiCtx, ctx, sessionID, message, writer, editorContext)
	if err != nil {
		slog.Error("AI response generation failed", "error", err)
		if strings.Contains(err.Error(), "401") {
			sendSSEWarning(writer, "AI接口尚未配置或配置错误。")
			fmt.Fprintf(writer, "event: config\ndata: %s\n\n", "{}")
			writer.Flush()
			return
		}
		// Fallback: use keyword-based response
		var toolNames []string
		allTools := currentSite.AiSrv.GetAllTools()
		for _, tool := range allTools {
			toolNames = append(toolNames, fmt.Sprintf("- %s: %s", tool.Name, tool.Description))
		}
		response = currentSite.AiSrv.BuildAIResponse(message, toolNames)
	}

	// Add assistant message to history
	currentSite.AiSrv.AddMessage(sessionID, provider.ChatMessage{
		Role:    "assistant",
		Content: response,
	})

	slog.Debug("event: end", "data:", "[DONE]")
	// Send final end event
	if writer != nil {
		fmt.Fprintf(writer, "event: end\ndata: [DONE]\n\n")
		writer.Flush()
	}
}

// generatePurePromptResponse 处理旧 AuthAiChat 兼容的纯 prompt 单次对话。
// 不绑定工具、不写会话历史、不处理文件附件/编辑器上下文。
// 直接用 Eino client 流式生成响应，通过 SSE event:message 返回。
//
// 优化：丢弃思考内容 (OnReasoning 设为空操作)，避免 reasoning 延迟导致前端卡顿；
// 加系统提示要求不思考直接返回结果，缩短响应时间。
func generatePurePromptResponse(ctx context.Context, prompt string, writer io.Writer) (string, error) {
	client, err := eino.GetClient()
	if err != nil {
		return "", fmt.Errorf("AI client not available: %w", err)
	}

	// 纯 prompt 模式：系统提示要求直接返回结果 + 不绑定任何工具
	messages := []*schema.Message{
		schema.SystemMessage(
			"直接回答用户问题，不要进行思考推理。" +
				"如果用户要求返回 JSON，请只返回 JSON 内容，不要包裹在 markdown 代码块中。"),
		schema.UserMessage(prompt),
	}

	streamResult, streamErr := provider.StreamWithRetry(
		ctx,
		func(ctx context.Context, msgs []*schema.Message) (*schema.StreamReader[*schema.Message], error) {
			return client.Stream(ctx, msgs)
		},
		messages,
		&provider.StreamCallbacks{
			OnChunk: func(chunk string) {
				chunkData, _ := json.Marshal(iris.Map{
					"v":         chunk,
					"timestamp": time.Now().Unix(),
				})
				slog.Debug("event: message", "data:", string(chunkData))
				if writer != nil {
					fmt.Fprintf(writer, "event: message\ndata: %s\n\n", string(chunkData))
					if f, ok := writer.(interface{ Flush() error }); ok {
						f.Flush()
					}
				}
			},
			// 丢弃思考内容：不发送 reasoning SSE，避免前端误解析
			OnReasoning: func(content string) {},
			OnWarning: func(msg string) {
				sendSSEWarning(writer, msg)
			},
		},
	)

	if streamErr != nil {
		return "", streamErr
	}

	return streamResult.Response, nil
}

// generateAIResponse calls the DeepSeek API via Eino with tool support and streams the response back via SSE.
// Implements a 7-step verification workflow inspired by atomcode:
// Step 1: 接收与诊断 | Step 2: 上下文构建 | Step 3: 模型推理 | Step 4: 工具执行
// Step 5: 错误恢复 | Step 6: 验证 | Step 7: 压缩与闭环
func generateAIResponse(ctx context.Context, irisCtx iris.Context, sessionID string, userMessage string, writer io.Writer, editorContext string) (string, error) {
	// Try to get the Eino client
	client, err := eino.GetClient()
	if err != nil {
		return "", fmt.Errorf("AI client not available: %w", err)
	}

	// ── Step 1: 构建系统提示（每会话缓存一次，保持 prefix cache 稳定） ──
	// 参考 AtomCode 的做法：system prompt 只构建一次，后续复用
	currentSite := provider.CurrentSite(irisCtx)

	// 从 session 获取缓存的 system prompt，仅在首次构建
	sess := currentSite.AiSrv.GetOrCreateSession(sessionID)
	systemPrompt := sess.CachedSystemPrompt
	if systemPrompt == "" {
		systemPrompt = buildSystemPrompt()
		sess.CachedSystemPrompt = systemPrompt
	}

	// ── Step 2: 上下文构建（带智能窗口压缩） ──
	messages := currentSite.AiSrv.BuildToolMessages(sessionID, systemPrompt)

	// Add current user message
	userMsg := userMessage
	if currentSite != nil {
		// 将站点动态信息放在 user message 前，不污染 system prompt 缓存
		userMsg = fmt.Sprintf("[当前站点：%s]\n%s", currentSite.System.SiteName, userMessage)
	}
	// 注入前端 AI 编辑器上下文（iframe URL + 模板名 + 选中 DOM + 模板内容）
	if editorContext != "" {
		userMsg = editorContext + "\n\n" + userMsg
	}
	messages = append(messages, schema.UserMessage(userMsg))

	// ── 一次性绑定全部可用工具 ──
	// 直接将所有工具定义提供给模型，避免按需声明导致模型无法获得正确工具的问题
	allTools, allHandlers := currentSite.AiSrv.GetEinoTools()
	currentSite.AiSrv.Handlers = allHandlers
	if err := client.BindTools(allTools); err != nil {
		return "", fmt.Errorf("failed to bind all tools: %w", err)
	}
	slog.Info("Bound all available tools",
		"session", sessionID, "tools", len(allTools))

	// ── P0: 工具执行中间件链 ──
	// write_gate: 主会话写操作需审批；Agent 会话跳过审批
	// result_truncator: 执行后统一截断超大结果
	// redaction: 执行后统一脱敏 (密码/Token/API Key/私钥)
	allowSet := sess.AllowOnce
	if allowSet == nil {
		allowSet = provider.NewSessionAllowSet()
		sess.AllowOnce = allowSet
	}
	isAgentSession := false
	if irisCtx != nil {
		// AiAgentChat 路径: agent.SessionId 复用 generateAIResponse
		// 主会话 sessionID 形如 "session_<nano>"，Agent 会话由 GetAgentBySessionID 命中
		if ag := currentSite.AiSrv.GetAgentBySessionID(sessionID); ag != nil {
			isAgentSession = true
		}
	}
	// 构造 flush 闭包：iris ResponseWriter.Flush() 无返回值，
	// 不能用 interface{ Flush() error } 断言，直接用具体类型调用
	flushFn := func() {
		if irisCtx != nil {
			irisCtx.ResponseWriter().Flush()
		}
	}
	approvalFn := buildApprovalFn(currentSite.AiSrv, writer, flushFn, sessionID, allowSet, isAgentSession)
	// P1: circuit_breaker 放在最前，先于 write_gate
	// 这样重复调用在审批前就被熔断，避免无意义审批弹窗
	circuitBreaker := provider.NewCircuitBreakerMiddleware()
	mwChain := provider.NewMiddlewareChain(
		circuitBreaker,
		&provider.WriteGateMiddleware{},
		&provider.SensitivePathGateMiddleware{},
		&provider.ResultTruncatorMiddleware{},
		&provider.RedactionMiddleware{},
	)

	// ── 7步验证工作流主循环 ──
	maxRounds := 15
	// P2: 截断恢复 — 仿 atomcode MAX_TRUNCATION_CONTINUATIONS=4
	const maxTruncationContinuations = 4
	truncationContinuations := 0
	// P2: AI 响应被 max_tokens 截断 (finish_reason=length) 时的续接次数
	const maxResponseTruncationContinuations = 4
	responseTruncationContinuations := 0
	// P2: 空响应恢复 — 仿 atomcode EMPTY_RESPONSE_MAX_RETRIES=5
	const emptyResponseMaxRetries = 5
	emptyResponseRetries := 0
	var finalResponse string
	var round int
	var consecutiveReads int   // Step 4: 空转检测计数器
	var executedTools []string // Step 4: 本轮已执行工具列表
	var hadWriteOperation bool // Step 4: 是否执行过写操作
	var totalTokens int        // Token 统计
	var totalPromptTokens, totalCompletionTokens int
	contextCompactCalls := 0 // 上下文压缩计数

	for round = 0; round < maxRounds; round++ {
		roundStart := time.Now()
		// 设置当前模型名 (供 P8 遥测记录使用)
		if currentSite.AiSrv.ModelName == "" {
			currentSite.AiSrv.ModelName = "default"
		}
		// ── P3: 四层重试分层 (transport → open → stream → partial) ──
		// StreamWithRetry 封装了所有重试逻辑，包括:
		//   - open:      DEFAULT_MAX_PROVIDER_RETRIES=3, 3/6/9s 线性退避
		//   - stream:    MAX_STREAM_RETRIES=5, 流中断时保留已收内容, 整轮重发
		//   - partial:   MAX_PARTIAL_STREAM_RECOVERIES=1, 保留已收内容, 注入 NUDGE
		//   - rate limit 精细化: 首 429 静默 1s, sustained 按 Retry-After 头等待
		streamResult, streamErr := provider.StreamWithRetry(
			ctx,
			func(ctx context.Context, messages []*schema.Message) (*schema.StreamReader[*schema.Message], error) {
				return client.Stream(ctx, messages)
			},
			messages,
			&provider.StreamCallbacks{
				OnChunk: func(chunk string) {
					chunkData, _ := json.Marshal(iris.Map{
						"v":         chunk,
						"timestamp": time.Now().Unix(),
					})
					slog.Debug("event: message", "data:", string(chunkData))
					if writer != nil {
						fmt.Fprintf(writer, "event: message\ndata: %s\n\n", string(chunkData))
						if f, ok := writer.(interface{ Flush() error }); ok {
							f.Flush()
						}
					}
				},
				OnReasoning: func(content string) {
					reasoningData, _ := json.Marshal(iris.Map{
						"v":         content,
						"timestamp": time.Now().Unix(),
					})
					slog.Debug("event: reasoning", "data:", string(reasoningData))
					if writer != nil {
						fmt.Fprintf(writer, "event: reasoning\ndata: %s\n\n", string(reasoningData))
						if f, ok := writer.(interface{ Flush() error }); ok {
							f.Flush()
						}
					}
				},
				OnWarning: func(msg string) {
					sendSSEWarning(writer, msg)
				},
			},
		)

		if streamErr != nil {
			// 不可重试错误 (400 Bad Request / context overflow)
			if provider.IsContextOverflowError(streamErr) {
				sendSSEWarning(writer, "上下文过长，正在压缩...")
				slog.Warn("Context overflow, compacting messages")
				messages = provider.CompactMessages(messages, 5)
				contextCompactCalls++
				continue
			}
			return "", fmt.Errorf("AI stream generate failed: %w", streamErr)
		}

		// 从 StreamResult 提取数据
		fullResponse := streamResult.Response
		fullReasoning := streamResult.Reasoning
		roundToolCalls := streamResult.ToolCalls
		promptTokens := streamResult.PromptTokens
		completionTokens := streamResult.CompletionTokens

		totalTokens += promptTokens + completionTokens
		totalPromptTokens += promptTokens
		totalCompletionTokens += completionTokens

		// ── P8: 遥测与成本归因 — 每轮记录 token 用量与成本 ──
		if promptTokens > 0 || completionTokens > 0 {
			roundDuration := time.Since(roundStart).Milliseconds()
			currentSite.AiSrv.RecordUsage(
				ctx,
				sessionID,
				0, // agentID=0 表示主会话
				round,
				currentSite.AiSrv.ModelName,
				promptTokens, completionTokens,
				len(roundToolCalls),
				roundDuration,
			)
		}

		// ── P1: 整轮签名重复检测 ──
		if len(roundToolCalls) > 0 {
			sig := provider.BuildRoundSignature(roundToolCalls)
			circuitBreaker.RecordRound(sessionID, sig)
			action := circuitBreaker.CheckRepeat(sessionID, sig)
			switch action {
			case "nudge":
				// REPEAT_NUDGE_AT=3: 注入 nudge 提示，让 AI 换策略但继续执行
				nudgeMsg := schema.UserMessage(
					"[系统提示] 你已经用相同的工具和参数重复执行了多轮，结果不会有变化。" +
						"请换一种策略：使用不同的参数、换一个工具，或者基于已有结果直接给出结论。")
				messages = append(messages, nudgeMsg)
				sendSSEWarning(writer, "检测到重复工具调用，注入换策略提示...")
			case "terminate":
				// MAX_REPEAT_ROUNDS=6: 终止并返回最后响应
				sendSSEWarning(writer, "重复工具调用达到上限，终止执行")
				finalResponse = fullResponse
				if finalResponse == "" {
					finalResponse = "由于检测到重复的工具调用，执行已被终止。" +
						"请尝试换一种方式描述您的需求。"
				}
				goto streamDone
			}
		}

		// ── Step 6: 检查模型是否完成 ──
		if len(roundToolCalls) == 0 {
			// No tool calls — this is the final text response
			finalResponse = fullResponse

			// P2: AI 响应被 max_tokens 截断 (finish_reason=length)
			// 注入 TRUNCATION_RESUME_NUDGE 让 AI 从断点继续完成
			if streamResult.FinishReason == "length" && responseTruncationContinuations < maxResponseTruncationContinuations {
				responseTruncationContinuations++
				// 保留已收内容，注入续接提示
				messages = append(messages, schema.AssistantMessage(fullResponse, nil))
				messages = append(messages, schema.UserMessage(
					"[系统提示] 你上一次的回复被 max_tokens 截断了。请从中断处继续完成，不要重复已输出的内容。"))
				sendSSEWarning(writer, fmt.Sprintf("响应被截断，注入续接提示 (%d/%d)...",
					responseTruncationContinuations, maxResponseTruncationContinuations))
				continue
			}
			responseTruncationContinuations = 0

			// P2: 空响应恢复 — AI 返回空文本且无 tool_calls
			// 仿 atomcode EMPTY_RESPONSE_MAX_RETRIES=5，短退避 0.5s
			if finalResponse == "" {
				emptyResponseRetries++
				if emptyResponseRetries <= emptyResponseMaxRetries {
					// 0.5s 短退避后重发同请求
					time.Sleep(500 * time.Millisecond)
					// 注入 user 消息引导 AI 重新生成
					recoveryMsg := schema.UserMessage(
						"[系统提示] 你的上一次响应为空，没有任何内容输出。" +
							"请根据当前对话上下文和已执行的工具结果，重新生成回复。" +
							"如果需要更多信息，请调用工具；如果已有足够信息，请直接给出结论。")
					messages = append(messages, recoveryMsg)
					sendSSEWarning(writer, fmt.Sprintf("检测到空响应，引导 AI 重新生成 (%d/%d)...",
						emptyResponseRetries, emptyResponseMaxRetries))
					continue
				}
				// 超过最大重试次数，注入"请输出总结或调用工具"提示后退出
				sendSSEWarning(writer, "空响应重试次数已达上限，注入最终提示")
				finalResponse = "由于多次收到空响应，无法生成回复。请尝试换一种方式描述您的需求。"
				break
			}

			// 有内容输出，重置空响应计数器
			emptyResponseRetries = 0

			if hadWriteOperation && finalResponse == "" {
				// 有修改但没有总结，注入验证提示
				sendSSEWarning(writer, "正在验证修改...")
			}
			break
		}

		// ── Step 4: 工具执行 ──
		// Add the assistant's message (with tool calls and reasoning content) to the history
		assistantMsg := &schema.Message{
			Role:             schema.Assistant,
			Content:          fullResponse,
			ToolCalls:        roundToolCalls,
			ReasoningContent: fullReasoning,
		}
		messages = append(messages, assistantMsg)

		// Save intermediate assistant message (with tool calls) to session history
		toolCallsJSON, _ := json.Marshal(roundToolCalls)
		currentSite.AiSrv.AddMessage(sessionID, provider.ChatMessage{
			Role:      "assistant",
			Content:   fullResponse,
			ToolCalls: string(toolCallsJSON),
		})

		// 执行工具前的统计
		var currentRoundExecutedTools []string
		var currentRoundHadWrite bool

		// ── P5: 并行工具执行 ──
		// Phase ②: parallel_safe 读工具并行 (sync.WaitGroup + Semaphore cap 4)
		// 写工具独占串行
		// 结果按 tc.ID 顺序收集后追加到 messages，保持 tool_call ↔ tool_result 配对完整
		type toolExecResult struct {
			index  int // 在 roundToolCalls 中的位置
			tc     schema.ToolCall
			result *provider.ToolExecResult
			denied bool
			reason string
		}
		results := make([]toolExecResult, len(roundToolCalls))

		// 分类：读工具 (parallel_safe) vs 写工具
		type toolTask struct {
			index   int
			tc      schema.ToolCall
			isWrite bool
		}
		var readTasks, writeTasks []toolTask
		for i, tc := range roundToolCalls {
			toolName := tc.Function.Name
			isWrite := provider.HasWriteOperation([]string{toolName})
			if isWrite {
				writeTasks = append(writeTasks, toolTask{i, tc, true})
			} else {
				readTasks = append(readTasks, toolTask{i, tc, false})
			}
		}

		// Phase ②: 读工具并行执行
		const maxParallel = 4
		sem := make(chan struct{}, maxParallel)
		var wg sync.WaitGroup
		var writeMu sync.Mutex // 保护 writer (SSE) 和 messages

		for _, task := range readTasks {
			wg.Add(1)
			go func(task toolTask) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				tc := task.tc
				toolName := tc.Function.Name

				currentSite.AiSrv.Logger.Info("Executing tool (parallel)",
					"name", toolName, "args", tc.Function.Arguments, "round", round)

				// Send tool_call SSE (加锁保护 writer)
				writeMu.Lock()
				toolCallData, _ := json.Marshal(iris.Map{
					"name":         toolName,
					"arguments":    tc.Function.Arguments,
					"tool_call_id": tc.ID,
				})
				slog.Debug("event: tool_call", "data:", string(toolCallData))
				if writer != nil {
					fmt.Fprintf(writer, "event: tool_call\ndata: %s\n\n", string(toolCallData))
					if f, ok := writer.(interface{ Flush() error }); ok {
						f.Flush()
					}
				}
				writeMu.Unlock()

				// ── P4a: 检查技能 allowed-tools 约束 ──
				if !currentSite.AiSrv.IsToolAllowed(toolName) {
					currentSite.AiSrv.Logger.Info("Tool blocked by skill allowed-tools",
						"name", toolName)
					results[task.index] = toolExecResult{
						index:  task.index,
						tc:     tc,
						result: &provider.ToolExecResult{Content: fmt.Sprintf("错误：技能 allowed-tools 约束不允许调用 %s", toolName)},
						denied: true,
						reason: "blocked by skill allowed-tools",
					}
					return
				}

				handler := currentSite.AiSrv.Handlers[toolName]
				execCtx := &provider.ToolExecContext{
					SessionID:  sessionID,
					IsAgent:    isAgentSession,
					ToolName:   toolName,
					AllowOnce:  allowSet,
					ApprovalFn: approvalFn,
				}
				execResult, denied, reason := mwChain.ExecuteTool(ctx, &tc, handler, execCtx)

				results[task.index] = toolExecResult{
					index:  task.index,
					tc:     tc,
					result: execResult,
					denied: denied,
					reason: reason,
				}
			}(task)
		}

		// 等待所有读工具完成
		wg.Wait()

		// 写工具串行执行 (独占)
		for _, task := range writeTasks {
			tc := task.tc
			toolName := tc.Function.Name

			currentSite.AiSrv.Logger.Info("Executing tool (serial write)",
				"name", toolName, "args", tc.Function.Arguments, "round", round)

			// Send tool_call SSE
			toolCallData, _ := json.Marshal(iris.Map{
				"name":         toolName,
				"arguments":    tc.Function.Arguments,
				"tool_call_id": tc.ID,
			})
			slog.Debug("event: tool_call", "data:", string(toolCallData))
			if writer != nil {
				fmt.Fprintf(writer, "event: tool_call\ndata: %s\n\n", string(toolCallData))
				if f, ok := writer.(interface{ Flush() error }); ok {
					f.Flush()
				}
			}

			// ── P4a: 检查技能 allowed-tools 约束 ──
			if !currentSite.AiSrv.IsToolAllowed(toolName) {
				currentSite.AiSrv.Logger.Info("Tool blocked by skill allowed-tools",
					"name", toolName)
				results[task.index] = toolExecResult{
					index:  task.index,
					tc:     tc,
					result: &provider.ToolExecResult{Content: fmt.Sprintf("错误：技能 allowed-tools 约束不允许调用 %s", toolName)},
					denied: true,
					reason: "blocked by skill allowed-tools",
				}
				continue
			}

			handler := currentSite.AiSrv.Handlers[toolName]
			execCtx := &provider.ToolExecContext{
				SessionID:  sessionID,
				IsAgent:    isAgentSession,
				ToolName:   toolName,
				AllowOnce:  allowSet,
				ApprovalFn: approvalFn,
			}
			execResult, denied, reason := mwChain.ExecuteTool(ctx, &tc, handler, execCtx)

			results[task.index] = toolExecResult{
				index:  task.index,
				tc:     tc,
				result: execResult,
				denied: denied,
				reason: reason,
			}
		}

		// 按原始顺序处理结果，追加 tool_result 到 messages
		for _, res := range results {
			tc := res.tc
			toolName := tc.Function.Name
			result := res.result.Content

			if res.denied {
				currentSite.AiSrv.Logger.Info("Tool execution denied",
					"name", toolName, "reason", res.reason)
			}

			currentSite.AiSrv.Logger.Info("Tool result",
				"name", toolName, "result", result)

			// Send tool_result event to client
			toolResultData, _ := json.Marshal(iris.Map{
				"name":         toolName,
				"tool_call_id": tc.ID,
				"result":       result,
				"denied":       res.denied,
			})
			slog.Debug("event: tool_result", "data:", string(toolResultData))
			if writer != nil {
				fmt.Fprintf(writer, "event: tool_result\ndata: %s\n\n", string(toolResultData))
				if f, ok := writer.(interface{ Flush() error }); ok {
					f.Flush()
				}
			}

			// 前端 AI 编辑器：重载模板后，通知前端刷新 iframe
			if !res.denied && toolName == "template_reload" {
				if writer != nil {
					fmt.Fprintf(writer, "event: iframe-reload\ndata: {\"tool\":\"%s\"}\n\n", toolName)
					if f, ok := writer.(interface{ Flush() error }); ok {
						f.Flush()
					}
				}
			}

			// Add tool result message to the conversation
			toolMsg := schema.ToolMessage(result, tc.ID)
			messages = append(messages, toolMsg)

			// P2: 截断恢复 — tool_result 被截断时，注入提示让 AI 用 offset 续读
			if res.result.Truncated {
				truncationContinuations++
				if truncationContinuations <= maxTruncationContinuations {
					recoveryMsg := schema.UserMessage(
						fmt.Sprintf("[系统提示] 上一个工具 (%s) 的结果被截断了，你只看到了前半部分。"+
							"如果需要完整内容，请用 offset 参数从截断处继续读取，而不是重复调用。"+
							"如果前半部分已足够回答问题，请直接基于已有内容给出结论。", toolName))
					messages = append(messages, recoveryMsg)
					sendSSEWarning(writer, "结果已截断，提示 AI 续读剩余内容...")
				} else {
					forceSummaryMsg := schema.UserMessage(
						fmt.Sprintf("[系统提示] 工具结果已被截断 %d 次，不再续读。"+
							"请基于已获取的内容直接给出结论或换一种方式获取信息。", truncationContinuations))
					messages = append(messages, forceSummaryMsg)
					truncationContinuations = 0
					sendSSEWarning(writer, "截断次数已达上限，提示 AI 总结已有内容")
				}
			}

			// Save tool result to session history
			toolContent := condenseToolContent(result, toolName)
			currentSite.AiSrv.AddMessage(sessionID, provider.ChatMessage{
				Role:       "tool",
				Content:    toolContent,
				ToolCallID: tc.ID,
				ToolName:   toolName,
			})

			// 跟踪工具类型
			currentRoundExecutedTools = append(currentRoundExecutedTools, toolName)
			if provider.HasWriteOperation([]string{toolName}) {
				currentRoundHadWrite = true
				hadWriteOperation = true
			}
		}

		executedTools = append(executedTools, currentRoundExecutedTools...)

		// ── Step 4 (续): 空转检测 ──
		if currentRoundHadWrite {
			consecutiveReads = 0
		} else {
			// 检查是否全是读取操作
			allReadOnly := true
			for _, name := range currentRoundExecutedTools {
				if provider.HasWriteOperation([]string{name}) {
					allReadOnly = false
					break
				}
			}
			if allReadOnly {
				consecutiveReads++
			} else {
				consecutiveReads = 0
			}
		}

		// 如果连续多轮只有读取操作，注入空转警告
		if consecutiveReads >= 4 {
			sendSSEWarning(writer, "检测到连续读取操作，提示模型聚焦实际任务")
			slog.Warn("Stagnation detected", "consecutiveReads", consecutiveReads)
			warningMsg := schema.SystemMessage(
				"[系统提示] 你已经连续多次执行读取操作但没有执行任何修改。请评估当前进度，" +
					"如果已经获取了足够的信息，请开始执行实际的操作（创建/更新/删除/发布）。")
			messages = append(messages, warningMsg)
			consecutiveReads = 0
			continue
		}

		// ── Step 6: 验证注入 ──
		// 如果本轮有写操作且有可运行的验证手段，注入验证提示
		if currentRoundHadWrite {
			sendSSEWarning(writer, "修改已执行，正在等待模型验证...")
			slog.Info("Write operation detected, injecting verification prompt")

			verifyMsg := schema.SystemMessage(
				"[系统验证] 你已经成功执行了修改操作。请先验证你的修改是否正确，" +
					"必要时通过 bash 工具运行构建/检查/测试命令确认没有引入错误。" +
					"验证通过后再总结回答用户。")
			messages = append(messages, verifyMsg)
			continue
		}

		// ── Step 7: 上下文压缩（每 3 轮或消息过多时） ──
		if len(messages) > 12 && (round%3 == 2 || len(messages) > 20) {
			slog.Info("Compressing context", "messageCount", len(messages), "round", round)
			messages = provider.CompactMessages(messages, 5)
			contextCompactCalls++
		}
	}

	// Send token usage via SSE
	if writer != nil && totalTokens > 0 {
		usageData, _ := json.Marshal(iris.Map{
			"prompt_tokens":     totalPromptTokens,
			"completion_tokens": totalCompletionTokens,
			"total_tokens":      totalTokens,
		})
		slog.Debug("event: usage", "data:", string(usageData))
		fmt.Fprintf(writer, "event: usage\ndata: %s\n\n", string(usageData))
		if f, ok := writer.(interface{ Flush() error }); ok {
			f.Flush()
		}
	}

	// 统计日志
	slog.Info("AI response completed",
		"rounds", round+1,
		"totalTokens", totalTokens,
		"contextCompactCalls", contextCompactCalls,
		"toolsExecuted", len(executedTools))

streamDone:
	if round == maxRounds && finalResponse == "" {
		// 获取最后一次响应的文本
		if len(messages) >= 2 {
			lastMsg := messages[len(messages)-1]
			if lastMsg.Role == "assistant" {
				finalResponse = lastMsg.Content
			}
		}
		if finalResponse == "" {
			return "", fmt.Errorf("工具调用次数超过上限，未获取到最终回复")
		}
	}

	return finalResponse, nil
}

// sendSSEWarning sends a warning event via SSE
func sendSSEWarning(writer io.Writer, warning string) {
	if writer == nil {
		return
	}
	warningData, _ := json.Marshal(iris.Map{
		"v":         warning,
		"timestamp": time.Now().Unix(),
	})
	slog.Debug("event: warning", "data:", string(warningData))
	fmt.Fprintf(writer, "event: warning\ndata: %s\n\n", string(warningData))
	if f, ok := writer.(interface{ Flush() error }); ok {
		f.Flush()
	}
}

// condenseToolContent condenses a tool result for history storage based on the tool type.
// read_file: compress to skeleton (extract signatures/imports)
// bash: keep first 2 lines
// Others: keep first 500 chars
func condenseToolContent(result string, toolName string) string {
	if len(result) <= 500 {
		return result
	}
	switch toolName {
	case "read_file":
		return provider.CompressFileToSkeleton(result)
	case "bash", "shell":
		lines := strings.SplitN(result, "\n", 3)
		if len(lines) <= 2 {
			return result
		}
		return strings.Join(lines[:2], "\n") + "\n... [已截断]"
	default:
		runes := []rune(result)
		if len(runes) > 500 {
			return string(runes[:500]) + "\n... [已截断]"
		}
		return result
	}
}

// buildApprovalFn 构造主会话审批回调。
//
// 主会话 (isAgent=false): 通过 SSE 向前端发送 `tool_confirm` 事件，
// 然后阻塞等待前端 POST /ai/chat/confirm 唤醒 pendingApprovals channel。
// 超时 (5 分钟) 后视为拒绝，防止无限挂起。
//
// Agent 会话 (isAgent=true): 返回 nil，write_gate middleware 会跳过审批
// 直接放行 (Agent 自动执行不需要人工审批)。
func buildApprovalFn(aiSrv *provider.AiChatService, writer io.Writer, flushFn func(), sessionID string, allowSet *provider.SessionAllowSet, isAgent bool) func(ctx context.Context, call *schema.ToolCall, toolName string) (string, string) {
	if isAgent {
		// Agent 会话: 不需要审批
		return nil
	}
	return func(ctx context.Context, call *schema.ToolCall, toolName string) (string, string) {
		toolCallID := call.ID
		// 注册 pending approval，等待前端唤醒
		ch := aiSrv.RegisterPendingApproval(toolCallID)

		// 向前端发送 tool_confirm SSE 事件
		confirmData, _ := json.Marshal(iris.Map{
			"session_id":   sessionID,
			"tool_call_id": toolCallID,
			"name":         toolName,
			"arguments":    call.Function.Arguments,
		})
		slog.Debug("event: tool_confirm", "data:", string(confirmData))
		if writer != nil {
			fmt.Fprintf(writer, "event: tool_confirm\ndata: %s\n\n", string(confirmData))
			// iris ResponseWriter.Flush() 无返回值，不能用 interface{ Flush() error } 断言
			// 由调用方传入正确的 flushFn
			if flushFn != nil {
				flushFn()
				slog.Debug("event: tool_confirm flushed")
			}
		}
		// 阻塞等待审批决策，带 10 分钟超时（给用户充足决策时间）
		select {
		case decision := <-ch:
			return decision, ""
		case <-time.After(10 * time.Minute):
			// 超时: 取消 pending，返回拒绝
			aiSrv.CancelPendingApproval(toolCallID)
			return "deny", "审批超时，已拒绝"
		case <-ctx.Done():
			// 请求取消: 取消 pending
			aiSrv.CancelPendingApproval(toolCallID)
			return "deny", "请求已取消"
		}
	}
}

// AiToolConfirm 处理前端审批决策 (POST /ai/chat/confirm)。
// 请求体: { "tool_call_id": "xxx", "decision": "allow"|"deny"|"once_allow" }
// 唤醒主会话 generateAIResponse 中挂起的 buildApprovalFn。
func AiToolConfirm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	if currentSite.AiSrv == nil {
		ctx.JSON(iris.Map{"code": -1, "msg": "ai service not available"})
		return
	}

	var req struct {
		ToolCallID string `json:"tool_call_id"`
		Decision   string `json:"decision"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{"code": -1, "msg": "invalid request"})
		return
	}
	if req.ToolCallID == "" || req.Decision == "" {
		ctx.JSON(iris.Map{"code": -1, "msg": "tool_call_id and decision are required"})
		return
	}

	// 校验决策值
	switch req.Decision {
	case "allow", "deny", "once_allow":
		// 合法决策
	default:
		ctx.JSON(iris.Map{"code": -1, "msg": "decision must be allow/deny/once_allow"})
		return
	}

	ok := currentSite.AiSrv.ResolvePendingApproval(req.ToolCallID, req.Decision)
	if !ok {
		ctx.JSON(iris.Map{"code": -1, "msg": "no pending approval for this tool_call_id"})
		return
	}

	ctx.JSON(iris.Map{"code": 0, "msg": "success"})
}

// autoDiagnoseErrors scans recent log output and returns formatted error context.
func autoDiagnoseErrors() string {
	// 这里可以扩展为读取日志文件并提取最新错误
	// 目前返回空字符串，表示没有额外的诊断信息
	return ""
}

// GetHistory returns chat history
func GetAiHistory(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	if currentSite.AiSrv == nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "ai service not available",
		})
		return
	}
	sessionID := ctx.URLParamDefault("session_id", "")
	if sessionID == "" {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "session_id is required",
		})
		return
	}

	messages := currentSite.AiSrv.GetMessages(sessionID)
	ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "success",
		"data": messages,
	})
}

// GetAiSessions returns all chat sessions list
func GetAiSessions(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	if currentSite.AiSrv == nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "ai service not available",
		})
		return
	}

	sessions := currentSite.AiSrv.ListSessions()
	ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "success",
		"data": sessions,
	})
}

// Health returns health status
func AiHealth(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	if currentSite.AiSrv == nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "ai service not available",
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": 0,
		"msg":  "ok",
		"data": iris.Map{
			"service": "anqicms-ai-chat",
			"status":  "running",
		},
	})
}

// AiChatUpload 上传临时文件供AI对话使用
func AiChatUpload(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()

	sessionID := ctx.PostValueDefault("session_id", "common")
	// 生成唯一文件名: 时间戳_原始文件名
	ext := filepath.Ext(info.Filename)
	baseName := strings.TrimSuffix(info.Filename, ext)
	saveName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), baseName, ext)

	// 保存到临时目录: CachePath/ai/upload/{sessionID}/
	uploadDir := filepath.Join(currentSite.CachePath, "ai", "upload", sessionID)
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("DirectoryCreationFailed"),
		})
		return
	}

	savePath := filepath.Join(uploadDir, saveName)
	dst, err := os.Create(savePath)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("FileSaveFailed"),
		})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("FileSaveFailed"),
		})
		return
	}

	// 返回保存的相对路径，
	filePath := strings.TrimPrefix(savePath, currentSite.RootPath)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "success",
		"data": iris.Map{
			"file_name":  info.Filename,
			"file_size":  info.Size,
			"file_path":  filePath,
			"session_id": sessionID,
			"file_ext":   ext,
		},
	})
}

// GetAiSettings returns all custom AI provider configs
func GetAiSettings(ctx iris.Context) {
	// 使用默认站点处理
	defaultSite := provider.CurrentSite(nil)

	settings := defaultSite.LoadAiSetting("")

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "success",
		"data": settings,
	})
}

// SaveAiSettings saves a custom AI provider config (add or update or delete)
// need to provide full config
func SaveAiSettings(ctx iris.Context) {
	defaultSite := provider.CurrentSite(nil)
	var req []*eino.Config
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "invalid request",
		})
		return
	}

	settings := defaultSite.LoadAiSetting("")
	settings.Configs = req

	if err := defaultSite.SaveSettingValue(provider.AiSettingKey, settings); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "save failed",
		})
		return
	}

	// cache new setting
	defaultSite.Cache.Set("ai_setting", settings, 86400)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "success",
		"data": settings,
	})
}

// AiAgentList 返回所有 AI 智能体列表
func AiAgentList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var agents []model.AiAgent
	currentSite.DB.Order("id ASC").Find(&agents)
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "success",
		"data": agents,
	})
}

// AiAgentLog 返回指定 Agent 的执行日志
func AiAgentLog(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	agentId, err := ctx.Params().GetUint("id")
	if err != nil {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": "ID无效"})
		return
	}
	offset := (currentPage - 1) * pageSize
	var total int64

	var logs []model.AiAgentLog
	currentSite.DB.Model(&model.AiAgentLog{}).Where("agent_id = ?", agentId).Order("id DESC").Count(&total).Limit(pageSize).Offset(offset).Find(&logs)
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "success",
		"data": logs,
	})
}

// AiAgentChat 与 AI 智能体的专属会话对话（SSE 流式）
func AiAgentChat(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	if currentSite.AiSrv == nil {
		ctx.JSON(iris.Map{"code": -1, "msg": "ai service not available"})
		return
	}
	agentId, err := ctx.Params().GetUint("id")
	if err != nil {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": "ID无效"})
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := ctx.ReadJSON(&req); err != nil || req.Message == "" {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": "消息不能为空"})
		return
	}

	// 查找 Agent
	agent := currentSite.AiSrv.GetAgent(agentId)
	if agent == nil {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": "智能体不存在"})
		return
	}

	// SSE 流式输出
	ctx.ContentType("text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	writer := ctx.ResponseWriter()

	// 构建系统提示
	systemPrompt := "你是 AnQiCMS 的 AI 智能体。以下是你的策略和对话历史。用户正在与你对话。"
	if agent.Strategy != "" {
		systemPrompt += "\n\n## 你的策略\n" + agent.Strategy
	}
	if agent.LastSummary != "" {
		systemPrompt += "\n\n## 上次执行摘要\n" + agent.LastSummary
	}

	// 解耦 context: 不绑定 HTTP 请求生命周期
	aiCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 复用主对话的生成逻辑，使用 agent 的 session
	_, err = generateAIResponse(aiCtx, ctx, agent.SessionId, req.Message, writer, "")
	if err != nil {
		slog.Error("Agent chat failed", "agent_id", agentId, "error", err)
	}
}

// buildSystemPrompt returns the session-level system prompt (static text).
// The caller caches it on the ChatSession so messages[0] stays byte-identical
// across turns, preserving the upstream provider's prefix cache.
func buildSystemPrompt() string {
	return `你是一个专业的 AnQiCMS 网站内容管理 AI 助手，帮助用户管理文章、分类、标签和附件。

## 工作流
请遵循以下步骤完成每个任务：
1. **先规划**：了解用户需求后，确定需要使用的工具和步骤，不要盲目操作。
2. **再执行**：按计划调用工具完成任务。
3. **验证**：执行修改操作后，运行验证工具（如 bash 执行构建/检查命令）确认修改正确。
4. **总结**：验证通过后，用中文总结完成的操作和结果。

## 使用指南
- 在创建文章时，先查看可用分类（使用 category_list 工具），然后选择合适的分类ID。
- 在创建分类时，先查看可用模型（使用 module_list 工具），然后选择合适的模型ID。
- 请用中文回复，保持专业、友好的语气。
- 执行了修改操作（创建/更新/删除）后，先验证再继续下一步。

## 技能系统(Skills)
- skill_list: 列出所有可用技能。当用户任务需要专业指导时先调用此工具。
- skill_get: 加载指定技能的完整内容。先确认技能匹配再调用。
- skill_reload: 管理员编辑技能后使用。
处理专业任务时，先查看可用技能，再加载匹配的技能内容并遵循其指导。
	用户的操作都会在当前站点中执行，请根据实际情况使用工具。`
}
