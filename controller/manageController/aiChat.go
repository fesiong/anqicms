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
	SessionID string        `json:"session_id"`
	Message   string        `json:"message"`
	Model     string        `json:"model"`
	Files     []ChatFileRef `json:"files,omitempty"`
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
	currentSite.AiSrv.AddMessage(sessionID, provider.ChatMessage{
		Role:    "user",
		Content: message,
		Files:   chatFiles,
	})

	requestCtx := ctx.Request().Context()

	// Generate AI response — try DeepSeek first, fall back to keyword matching
	response, err := generateAIResponse(requestCtx, ctx, sessionID, message, writer)
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

	// Send final end event
	if writer != nil {
		fmt.Fprintf(writer, "event: end\ndata: [DONE]\n\n")
		writer.Flush()
	}
}

// generateAIResponse calls the DeepSeek API via Eino with tool support and streams the response back via SSE.
// Implements a 7-step verification workflow inspired by atomcode:
// Step 1: 接收与诊断 | Step 2: 上下文构建 | Step 3: 模型推理 | Step 4: 工具执行
// Step 5: 错误恢复 | Step 6: 验证 | Step 7: 压缩与闭环
func generateAIResponse(ctx context.Context, irisCtx iris.Context, sessionID string, userMessage string, writer io.Writer) (string, error) {
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
		// 附加能力包声明指引（仅首次，随 system prompt 一起缓存）
		declarationGuide := "\n\n## 能力包声明\n你必须首先调用 declare_capability_packages 工具，声明本次对话需要的能力包，然后才能使用其他工具。可用能力包：content（内容管理）、structure（结构管理）、design（模板设计）、seo（SEO优化）、admin（系统管理）、agent（智能体管理）、universal（通用工具）。根据用户请求选择合适的能力包组合。\n\n### 能力包说明\n- content: 文档创建/编辑/发布、分类管理、标签管理、附件管理、评论管理\n- structure: 页面管理、导航管理、友链管理、模块管理、重定向管理\n- design: 模板文件编辑、样式修改、锚文本管理\n- seo: 关键词管理、站点地图、搜索引擎统计\n- admin: 系统设置、插件管理、用户管理、订单管理、缓存管理\n- agent: 智能体管理\n- universal: 始终可用。文件读写、Shell、搜索、网站信息等"
		systemPrompt = strings.Replace(systemPrompt, "用户的操作都会在当前站点中执行", declarationGuide+"\n\n用户的操作都会在当前站点中执行", 1)
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
	messages = append(messages, schema.UserMessage(userMsg))

	// ── 能力包机制：按需绑定工具 ──
	// 未声明的会话先绑定摘要工具（精简参数），模型需先调用 declare_capability_packages
	// 已声明的会话直接绑定该能力包的完整工具定义
	if !sess.ToolsFinalized {
		summaryTools := currentSite.AiSrv.GetEinoToolsSummary()
		declareTool := provider.BuildDeclareTool()
		allBindTools := append(summaryTools, declareTool)

		// 注入声明工具 handler，捕获 client 和 sess 以完成工具切换
		currentSite.AiSrv.Handlers["declare_capability_packages"] = func(ctx context.Context, argsJSON string) (string, error) {
			var declareReq struct {
				Packages []string `json:"packages"`
			}
			if err := json.Unmarshal([]byte(argsJSON), &declareReq); err != nil {
				return "", fmt.Errorf("声明格式错误: %s", err.Error())
			}
			if len(declareReq.Packages) == 0 {
				return "", fmt.Errorf("必须至少声明一个能力包")
			}
			// 去重
			seen := map[string]bool{}
			var pkgs []string
			for _, p := range declareReq.Packages {
				if !seen[p] {
					seen[p] = true
					pkgs = append(pkgs, p)
				}
			}
			sess.DeclaredPackages = pkgs
			sess.ToolsFinalized = true
			slog.Info("Capability packages declared", "session", sessionID, "packages", pkgs)

			// 切换为完整的工具集
			filteredTools, filteredHandlers := currentSite.AiSrv.GetToolsByCapabilityPackages(pkgs)
			if err := client.BindTools(filteredTools); err != nil {
				return "", fmt.Errorf("工具绑定失败: %s", err.Error())
			}
			// 更新 handler 映射，保留 declare 工具以支持重新声明
			currentSite.AiSrv.Handlers = filteredHandlers
			currentSite.AiSrv.Handlers["declare_capability_packages"] = func(ctx context.Context, argsJSON string) (string, error) {
				return "能力包已更新", nil
			}
			slog.Info("Switched to full tools", "session", sessionID, "count", len(filteredTools))
			return fmt.Sprintf("能力包已确认：%s。现在可以使用这些能力包中的全部工具。", strings.Join(pkgs, ", ")), nil
		}

		if err := client.BindTools(allBindTools); err != nil {
			return "", fmt.Errorf("failed to bind summary tools: %w", err)
		}
		slog.Info("Using summary tools for capability declaration",
			"session", sessionID, "tools", len(allBindTools))
	} else {
		filteredTools, filteredHandlers := currentSite.AiSrv.GetToolsByCapabilityPackages(sess.DeclaredPackages)
		if err := client.BindTools(filteredTools); err != nil {
			return "", fmt.Errorf("failed to bind filtered tools: %w", err)
		}
		// 同步更新 handler 映射
		currentSite.AiSrv.Handlers = filteredHandlers
		slog.Info("Using filtered tools for execution",
			"session", sessionID, "packages", sess.DeclaredPackages, "tools", len(filteredTools))
	}

	// ── 7步验证工作流主循环 ──
	maxRounds := 15
	var finalResponse string
	var round int
	var retryCount int
	var consecutiveReads int   // Step 4: 空转检测计数器
	var executedTools []string // Step 4: 本轮已执行工具列表
	var hadWriteOperation bool // Step 4: 是否执行过写操作
	var totalTokens int        // Token 统计
	var totalPromptTokens, totalCompletionTokens int
	contextCompactCalls := 0 // 上下文压缩计数

	for round = 0; round < maxRounds; round++ {
		// ── Step 5: 错误恢复 — 重试循环 ──
		var stream *schema.StreamReader[*schema.Message]
		for attempt := 0; attempt <= 3; attempt++ {
			if attempt > 0 {
				retryCount++
			}
			stream, err = client.Stream(ctx, messages)
			if err == nil {
				break
			}
			if provider.IsRateLimitError(err) && attempt < 3 {
				wait := time.Duration((attempt+1)*3) * time.Second
				sendSSEWarning(writer, fmt.Sprintf("请求频率限制，%v 后重试(%d/3)...", wait, attempt+1))
				slog.Warn("Rate limited, retrying", "attempt", attempt+1, "wait", wait)
				time.Sleep(wait)
				continue
			}
			if provider.IsContextOverflowError(err) {
				sendSSEWarning(writer, "上下文过长，正在压缩...")
				slog.Warn("Context overflow, compacting messages")
				messages = provider.CompactMessages(messages, 5)
				contextCompactCalls++
				continue
			}
			return "", fmt.Errorf("AI stream generate failed: %w", err)
		}
		if stream == nil {
			return "", fmt.Errorf("AI stream generate failed after retries")
		}

		// ── Step 3: 模型推理（流式接收） ──
		var fullResponse strings.Builder
		var fullReasoning strings.Builder
		var roundToolCalls []schema.ToolCall
		var toolCallChunks []*schema.Message
		var promptTokens, completionTokens int

		for {
			msg, err := stream.Recv()
			if err != nil {
				break
			}

			// 跟踪 token 使用
			if msg.ResponseMeta != nil {
				if msg.ResponseMeta.Usage != nil {
					promptTokens = msg.ResponseMeta.Usage.PromptTokens
					completionTokens = msg.ResponseMeta.Usage.CompletionTokens
				}
			}

			chunk := msg.Content
			if chunk != "" {
				fullResponse.WriteString(chunk)
				// Send each chunk as SSE
				chunkData, _ := json.Marshal(iris.Map{
					"v":         chunk,
					"timestamp": time.Now().Unix(),
				})
				if writer != nil {
					fmt.Fprintf(writer, "event: message\ndata: %s\n\n", string(chunkData))
					if f, ok := writer.(interface{ Flush() error }); ok {
						f.Flush()
					}
				}
			}

			// Handle reasoning content
			if msg.ReasoningContent != "" {
				fullReasoning.WriteString(msg.ReasoningContent)
				reasoningData, _ := json.Marshal(iris.Map{
					"v":         msg.ReasoningContent,
					"timestamp": time.Now().Unix(),
				})
				if writer != nil {
					fmt.Fprintf(writer, "event: reasoning\ndata: %s\n\n", string(reasoningData))
					if f, ok := writer.(interface{ Flush() error }); ok {
						f.Flush()
					}
				}
			}

			// Collect all tool call chunks for later merging
			if len(msg.ToolCalls) > 0 {
				toolCallChunks = append(toolCallChunks, msg)
			}
		}
		stream.Close()

		totalTokens += promptTokens + completionTokens
		totalPromptTokens += promptTokens
		totalCompletionTokens += completionTokens

		// Merge all tool call chunks accumulated during streaming.
		// Individual chunks contain partial data (e.g. empty ID, split Arguments),
		// so we must use schema.ConcatMessages to reconstruct the full tool calls.
		if len(toolCallChunks) > 0 {
			merged, err := schema.ConcatMessages(toolCallChunks)
			if err == nil && merged != nil && len(merged.ToolCalls) > 0 {
				roundToolCalls = merged.ToolCalls
			}
		}

		// ── Step 6: 检查模型是否完成 ──
		if len(roundToolCalls) == 0 {
			// No tool calls — this is the final text response
			finalResponse = fullResponse.String()
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
			Content:          fullResponse.String(),
			ToolCalls:        roundToolCalls,
			ReasoningContent: fullReasoning.String(),
		}
		messages = append(messages, assistantMsg)

		// Save intermediate assistant message (with tool calls) to session history
		toolCallsJSON, _ := json.Marshal(roundToolCalls)
		currentSite.AiSrv.AddMessage(sessionID, provider.ChatMessage{
			Role:      "assistant",
			Content:   fullResponse.String(),
			ToolCalls: string(toolCallsJSON),
		})

		// 执行工具前的统计
		var currentRoundExecutedTools []string
		var currentRoundHadWrite bool

		// Execute each tool
		for _, tc := range roundToolCalls {
			toolName := tc.Function.Name
			argsJSON := tc.Function.Arguments

			currentSite.AiSrv.Logger.Info("Executing tool",
				"name", toolName,
				"args", argsJSON,
				"round", round)

			// Send tool_call event to client
			toolCallData, _ := json.Marshal(iris.Map{
				"name":         toolName,
				"arguments":    argsJSON,
				"tool_call_id": tc.ID,
			})
			if writer != nil {
				fmt.Fprintf(writer, "event: tool_call\ndata: %s\n\n", string(toolCallData))
				if f, ok := writer.(interface{ Flush() error }); ok {
					f.Flush()
				}
			}

			// Execute the handler
			handler, exists := currentSite.AiSrv.Handlers[toolName]
			var result string
			if !exists {
				result = fmt.Sprintf("错误：未知工具 %s", toolName)
			} else {
				result, err = handler(ctx, argsJSON)
				if err != nil {
					result = fmt.Sprintf("工具执行错误: %s", err.Error())
				}
			}

			currentSite.AiSrv.Logger.Info("Tool result",
				"name", toolName,
				"result", result)

			// Send tool_result event to client
			toolResultData, _ := json.Marshal(iris.Map{
				"name":         toolName,
				"tool_call_id": tc.ID,
				"result":       result,
			})
			if writer != nil {
				fmt.Fprintf(writer, "event: tool_result\ndata: %s\n\n", string(toolResultData))
				if f, ok := writer.(interface{ Flush() error }); ok {
					f.Flush()
				}
			}

			// Add tool result message to the conversation
			toolMsg := schema.ToolMessage(result, tc.ID)
			messages = append(messages, toolMsg)

			// Save tool result to session history (truncated if too large, condensed with tool name)
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

			// 截断过大的工具结果
			if len(result) > 10000 {
				result = result[:10000] + "\n... [结果已截断]"
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
	currentSite.DB.Where("agent_id = ?", agentId).Order("id DESC").Count(&total).Limit(pageSize).Offset(offset).Find(&logs)
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

	// 复用主对话的生成逻辑，使用 agent 的 session
	_, err = generateAIResponse(ctx, ctx, agent.SessionId, req.Message, writer)
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
