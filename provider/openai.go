package provider

import (
	"context"
	"errors"
	"github.com/sashabaranov/go-openai"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type OpenAIResult struct {
	Content string `json:"content"`
	Usage   int    `json:"usage"`
	Code    int    `json:"code"`
}

func (w *Website) SelfAiTranslate(content string, toLanguage string) (string, error) {
	prompt := "请将下列文字翻译成" + toLanguage + "：\n" + content
	if w.AiGenerateConfig.AiEngine == config.AiEngineOpenAI {
		if !w.AiGenerateConfig.ApiValid {
			return "", errors.New(w.Tr("InterfaceUnavailable"))
		}
		key := w.GetOpenAIKey()
		if key == "" {
			return "", errors.New(w.Tr("NoAvailableKey"))
		}

		result, err := w.GetOpenAIResponse(key, prompt)
		if err != nil {
			if result != nil && (result.Code == 401 || result.Code == 429) {
				w.SetOpenAIKeyInvalid(key)
			}
			return "", err
		}
		if len(result.Content) == 0 {
			return "", errors.New(w.Tr("InsufficientTextContent"))
		}

		return result.Content, nil
	} else if w.AiGenerateConfig.AiEngine == config.AiEngineSpark {
		tmpContent, err := GetSparkResponse(w.AiGenerateConfig.Spark, prompt)
		if err != nil {
			return "", err
		}
		if len(tmpContent) == 0 {
			return "", errors.New(w.Tr("InsufficientTextContent"))
		}

		return tmpContent, nil
	}

	return "", errors.New(w.Tr("NoAiGenerationSourceSelected"))
}

func (w *Website) SelfAiPseudoResult(req *AnqiAiRequest) (*AnqiAiRequest, error) {
	var result *OpenAIResult
	var err error
	// 标题则采用另一种方式
	prompt := "请重写这个标题：\n" + req.Title
	if req.Language == config.LanguageEn {
		prompt = "Please rewrite this title:\n" + req.Title
	}
	if w.AiGenerateConfig.AiEngine == config.AiEngineOpenAI {
		if !w.AiGenerateConfig.ApiValid {
			return nil, errors.New(w.Tr("InterfaceUnavailable"))
		}
		key := w.GetOpenAIKey()
		if key == "" {
			return nil, errors.New(w.Tr("NoAvailableKey"))
		}

		result, err = w.GetOpenAIResponse(key, prompt)
		if err != nil {
			if result.Code == 401 || result.Code == 429 {
				w.SetOpenAIKeyInvalid(key)
			}
			return nil, err
		}
	} else if w.AiGenerateConfig.AiEngine == config.AiEngineSpark {
		content, err := GetSparkResponse(w.AiGenerateConfig.Spark, prompt)
		if err != nil {
			return nil, err
		}
		result = &OpenAIResult{
			Content: content,
			Usage:   0,
			Code:    200,
		}
	} else {
		// 错误
		return nil, errors.New(w.Tr("NoAiGenerationSourceSelected"))
	}

	if len(result.Content) == 0 {
		return nil, errors.New(w.Tr("InsufficientTextContent"))
	}
	req.Title = result.Content

	// 先获取文章img，如果有的话
	re, _ := regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
	images := re.FindAllString(req.Content, -1)

	contentText := ParsePlanText(req.Content, "")
	texts := strings.Split(contentText, "\n")
	start := 0
	var contentTexts []string
	if utf8.RuneCountInString(contentText) > 1000 {
		for i := 1; i <= len(texts); i++ {
			if utf8.RuneCountInString(strings.Join(texts[start:i], "\n")) > 1000 {
				tmpText := strings.Join(texts[start:i-1], "\n")
				contentTexts = append(contentTexts, tmpText)
				start = i - 1
			}
		}
		tmpText := strings.Join(texts[start:], "\n")
		contentTexts = append(contentTexts, tmpText)
	} else {
		contentTexts = append(contentTexts, contentText)
	}
	for i := range contentTexts {
		// before replace
		prompt = "请根据提供的内容完成内容重构，并保持段落结构：\n" + contentTexts[i]
		if req.Language == config.LanguageEn {
			prompt = "Please complete the content reconstruction according to the provided content, and keep the paragraph structure:\n" + contentTexts[i]
		}
		if w.AiGenerateConfig.AiEngine == config.AiEngineOpenAI {
			if !w.AiGenerateConfig.ApiValid {
				return nil, errors.New(w.Tr("InterfaceUnavailable"))
			}
			key := w.GetOpenAIKey()
			if key == "" {
				return nil, errors.New(w.Tr("NoAvailableKey"))
			}

			result, err = w.GetOpenAIResponse(key, prompt)
			if err != nil {
				if result.Code == 401 || result.Code == 429 {
					w.SetOpenAIKeyInvalid(key)
				}
				return nil, err
			}
		} else if w.AiGenerateConfig.AiEngine == config.AiEngineSpark {
			content, err := GetSparkResponse(w.AiGenerateConfig.Spark, prompt)
			if err != nil {
				return nil, err
			}
			result = &OpenAIResult{
				Content: content,
				Usage:   0,
				Code:    200,
			}
		}

		if len(result.Content) == 0 {
			return nil, errors.New(w.Tr("InsufficientTextContent"))
		}

		contentTexts[i] = result.Content
	}
	translated := strings.Join(contentTexts, "\n")

	results := strings.Split(translated, "\n")
	for i := 0; i < len(results); i++ {
		results[i] = strings.TrimSpace(results[i])
		if len(results[i]) == 0 {
			results = append(results[:i], results[i+1:]...)
			i--
		}
	}
	// 如果有图片，则需要重新插入图片
	if len(images) > 0 {
		for i := range images {
			insertIndex := i*2 + 1
			if len(results) >= insertIndex {
				results = append(results[:insertIndex], results[insertIndex-1:]...)
				results[insertIndex] = images[i]
			}
		}
	}

	req.Content = strings.Join(results, "\n")
	if w.Content.Editor != "markdown" {
		req.Content = library.MarkdownToHTML(req.Content, w.System.BaseUrl, w.Content.FilterOutlink)
	}

	return req, nil
}

func (w *Website) SelfAiGenerateResult(req *AnqiAiRequest) (*AnqiAiRequest, error) {
	var result *OpenAIResult
	var err error
	prompt := "请根据关键词生成一篇中文文章，将文章标题放在第一行。关键词：" + req.Keyword
	if w.AiGenerateConfig.DoubleTitle {
		prompt = "请您基于关键词'" + req.Keyword + "'生成一篇双标题文章，输出格式'主标题：（在此处输入主标题）\n副标题：（在此处输入副标题）正文：（在此处输入正文内容）'，要求表意清晰，主题鲜明，分段表述"
	}
	if strings.HasPrefix(req.Language, config.LanguageEn) || strings.HasPrefix(w.AiGenerateConfig.Language, config.LanguageEn) {
		prompt = "Please generate an English article based on the keywords, and put the article title on the first line. Keywords: " + req.Keyword
	}
	if len(req.Demand) > 0 {
		prompt += "\n" + req.Demand
	}
	if w.AiGenerateConfig.AiEngine == config.AiEngineOpenAI || w.AiGenerateConfig.AiEngine == config.AiEngineDeepSeek {
		if w.AiGenerateConfig.AiEngine == config.AiEngineOpenAI {
			if !w.AiGenerateConfig.ApiValid {
				return nil, errors.New(w.Tr("InterfaceUnavailable"))
			}
		}
		key := w.GetOpenAIKey()
		if key == "" {
			return nil, errors.New(w.Tr("NoAvailableKey"))
		}
		// DeepSeek和openai共用一个处理方法
		result, err = w.GetOpenAIResponse(key, prompt)
		if err != nil {
			if result.Code == 401 || result.Code == 429 {
				w.SetOpenAIKeyInvalid(key)
			}
			return nil, err
		}
	} else if w.AiGenerateConfig.AiEngine == config.AiEngineSpark {
		content, err := GetSparkResponse(w.AiGenerateConfig.Spark, prompt)
		if err != nil {
			return nil, err
		}
		result = &OpenAIResult{
			Content: content,
			Usage:   0,
			Code:    200,
		}
	} else {
		// 错误
		return nil, errors.New(w.Tr("NoAiGenerationSourceSelected"))
	}

	if len(result.Content) < 2 {
		return nil, errors.New(w.Tr("InsufficientGeneratedContent"))
	}
	// 解析内容
	if strings.Count(result.Content, "\n") < 3 {
		replaces := []map[string]string{
			{"key": "副标题：", "value": "\n副标题："},
			{"key": "副标题:", "value": "\n副标题："},
			{"key": "正文：", "value": "\n正文："},
			{"key": "正文:", "value": "\n正文："},
			{"key": "内容：", "value": "\n正文："},
			{"key": "内容:", "value": "\n正文："},
		}
		for _, item := range replaces {
			result.Content = strings.Replace(result.Content, item["key"], item["value"], 1)
		}
		var tmpContent []string
		runes := []rune(result.Content)
		start := false
		tmpIndex := 0
		for i, v := range runes {
			if v == '\n' {
				tmpContent = append(tmpContent, string(runes[tmpIndex:i+1]))
				tmpIndex = i + 1
			} else if v == '。' || v == '！' || v == '？' || v == '?' || v == '!' {
				if !start {
					tmpIndex = i + 1
					start = true
				} else if i-tmpIndex >= 200 {
					tmpContent = append(tmpContent, string(runes[tmpIndex:i+1]))
					tmpIndex = i + 1
				}
			}
		}
		if len(runes)-tmpIndex > 1 {
			tmpContent = append(tmpContent, string(runes[tmpIndex:]))
		}
		result.Content = strings.Join(tmpContent, "\n")
	}
	replaces := []map[string]string{
		{"key": "文章标题:", "value": "标题："},
		{"key": "文章标题：", "value": "标题："},
		{"key": "[文章标题]", "value": "标题："},
		{"key": "【文章标题】", "value": "标题："},
		{"key": "标题:", "value": "标题："},
		{"key": "[标题]", "value": "标题："},
		{"key": "【标题】", "value": "标题："},
		{"key": "主标题:", "value": "主标题："},
		{"key": "副标题:", "value": "副标题："},
		{"key": "正文:", "value": "正文："},
		{"key": "[正文]", "value": "正文："},
		{"key": "【正文】", "value": "正文："},
		{"key": "内容:", "value": "正文："},
		{"key": "内容：", "value": "正文："},
		{"key": "[内容]", "value": "正文："},
		{"key": "【内容】", "value": "正文："},
		{"key": "：：", "value": "："},
		{"key": ":：", "value": "："},
	}
	for _, item := range replaces {
		result.Content = strings.Replace(result.Content, item["key"], item["value"], 1)
	}
	// 获取标题
	results := strings.Split(result.Content, "\n")
	title := strings.Trim(results[0], "# *")
	if w.AiGenerateConfig.DoubleTitle {
		if strings.Contains(title, "主标题：") {
			title = strings.TrimPrefix(title, "主标题：")
		}
		results = results[1:]
		if len(results) > 0 && strings.HasPrefix(results[0], "副标题：") {
			splitType := w.AiGenerateConfig.DoubleSplit
			if splitType == 5 {
				// 随机 0-4
				splitType = rand.New(rand.NewSource(time.Now().UnixNano())).Intn(5)
			}
			switch splitType {
			case 0:
				// 主标题(副标题)
				title += "(" + strings.TrimPrefix(results[0], "副标题：") + ")"
			case 1:
				// 主标题-副标题
				title += "-" + strings.TrimPrefix(results[0], "副标题：")
			case 2:
				// 主标题？副标题
				title += "，" + strings.TrimPrefix(results[0], "副标题：")
			case 3:
				// 主标题，副标题
				title += "，" + strings.TrimPrefix(results[0], "副标题：")
			case 4:
				// 主标题：副标题
				title += "：" + strings.TrimPrefix(results[0], "副标题：")

			}
		}
	}
	if req.Language == config.LanguageEn && strings.Count(title, " ") > 20 && !strings.Contains(results[0], "Title:") {
		title = req.Keyword
	} else if req.Language == config.LanguageZh && w.AiGenerateConfig.DoubleTitle == false && utf8.RuneCountInString(title) > 50 && !strings.Contains(results[0], "标题：") {
		title = req.Keyword
	} else {
		results = results[1:]
	}
	// 标题替换：
	title = strings.TrimPrefix(title, "Title:")
	title = strings.TrimPrefix(title, "标题：")
	title = strings.TrimPrefix(title, "文章标题：")
	title = strings.TrimPrefix(title, "主标题：")
	title = strings.TrimPrefix(title, "副标题：")
	title = strings.Replace(title, "副标题", "", 1)
	title = strings.Replace(title, "：", "，", 1)
	title = strings.Trim(title, "# *")
	if utf8.RuneCountInString(title) > 150 {
		title = string([]rune(title)[:150])
	}
	// 需要移除的关键词：
	var removeWords = []string{
		"作为语言AI，我不能提供对此事件的道德判断和态度", "首先，", "其次，", "最后，", "总之，", "总而言之，", "这不是什么关键词吧，这就是一堆原始材料，给我点时间，我来给你写一篇让你满意的文章！", "【强语气】", "注意：以下内容由AI生成，仅供参考。", "作为AI语言生成器，", "作为语言AI，", "以下是AI生成的文章，仅供参考：", "注意，该文章仅为AI生成，可能存在不当之处，仅供参考。",
		"AI生成", "作为AI语言模型", "本篇文章是人工智能生成", "作为 AI 语言模型", "不过总的来说，", "最重要的是，", "对于个人而言，", "值得注意的是，", "值得一提的是，", "需要注意的是，", "需要指出的是，", "需要明确的是，", "通过这个活动，", "除了以上几点，", "以上就是关于", "综合以上几点，", "在这个过程中，", "一段时间以来，", "综上所述，", "除此之外，",
		"众所周知，", "尽管如此，", "一般来说，", "不仅如此，", "总的来说，", "总的来看，", "总体来说，", "通常来说，", "无论如何，", "总体而言，", "总结起来，", "总结一下，", "在游戏中，", "就在这时，", "他们认为，", "有人认为，", "提醒大家，", "只有这样，", "近年来，", "在未来，", "首先是", "其次是", "最后是", "再比如，", "如果说，", "接下来，", "事实上，",
		"在中国，", "请记住，", "实际上，", "通过它，", "现如今，", "这时候，", "而近日，", "而现在，", "比如说，", "据说，", "据悉，", "这样，", "比如，", "近日，", "未来，", "如果，", "这时，", "然后，", "今天，", "还有，", "最终，", "下面，", "而且，", "然而，", "再次，", "但是，", "再者，", "此外，", "另外，", "现在，", "目前，", "同时，", "最近，",
		"那么，", "并且，", "因此，", "为此，", "当然，", "其中，", "不过，", "因为，", "所以，", "如今，", "例如，", "接着，", "总结，", "总结：", "结论，", "结语，", "结语：", "好了，", "原来，", "记住，", "九锤", "很抱歉，作为AI助手，", "作为AI助手，", "前言：", "与此同时，", "结论：", "简单来说，", "总体来讲，", "内容：", "标题：", "正文：",
	}
	for i := 0; i < len(results); i++ {
		results[i] = strings.TrimSpace(results[i])
		for _, w2 := range removeWords {
			if strings.HasPrefix(results[i], w2) {
				results[i] = strings.TrimPrefix(results[i], w2)
				break
			}
		}
		if i == len(results)-1 {
			// 内容最后一段，删除第一个逗号前的内容
			var seps []string
			seps = strings.SplitN(results[i], "，", 2)
			if len(seps) == 2 && utf8.RuneCountInString(seps[0]) < 6 {
				results[i] = seps[1]
			}
		}
	}
	req.Title = title
	req.Content = strings.Join(results, "\n")
	if w.Content.Editor != "markdown" {
		req.Content = library.MarkdownToHTML(req.Content, w.System.BaseUrl, w.Content.FilterOutlink)
	}

	return req, nil
}

func (w *Website) GetOpenAIResponse(apiKey, prompt string) (*OpenAIResult, error) {
	cfg := openai.DefaultConfig(apiKey)
	if w.AiGenerateConfig.OpenAiApi != "" {
		cfg.BaseURL = w.AiGenerateConfig.OpenAiApi
	}
	transport := &http.Transport{}
	proxy := os.Getenv("HTTP_PROXY")
	if len(proxy) > 0 {
		proxyUrl, err := url.Parse(proxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyUrl)
		}
	}
	cfg.HTTPClient = &http.Client{
		Transport: transport,
	}
	client := openai.NewClientWithConfig(cfg)
	model := openai.GPT3Dot5Turbo
	if w.AiGenerateConfig.OpenAIModel != "" {
		model = w.AiGenerateConfig.OpenAIModel
	}
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	result := &OpenAIResult{}

	if err != nil {
		msg := err.Error()
		re, _ := regexp.Compile(`code: (\d+),`)
		match := re.FindStringSubmatch(msg)
		if len(match) > 1 {
			result.Code, _ = strconv.Atoi(match[1])
		}
		return result, err
	}
	result.Content = resp.Choices[0].Message.Content
	result.Usage = resp.Usage.TotalTokens

	return result, nil
}

func (w *Website) GetOpenAIStreamResponse(apiKey, prompt string) (*openai.ChatCompletionStream, error) {
	cfg := openai.DefaultConfig(apiKey)
	if w.AiGenerateConfig.OpenAiApi != "" {
		cfg.BaseURL = w.AiGenerateConfig.OpenAiApi
	}
	transport := &http.Transport{}
	proxy := os.Getenv("HTTP_PROXY")
	if len(proxy) > 0 {
		proxyUrl, err := url.Parse(proxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyUrl)
		}
	}
	cfg.HTTPClient = &http.Client{
		Transport: transport,
	}
	client := openai.NewClientWithConfig(cfg)
	ctx := context.Background()
	model := openai.GPT3Dot5Turbo
	if w.AiGenerateConfig.OpenAIModel != "" {
		model = w.AiGenerateConfig.OpenAIModel
	}
	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Stream: true,
	}
	return client.CreateChatCompletionStream(ctx, req)
}
