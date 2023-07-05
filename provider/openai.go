package provider

import (
	"context"
	"errors"
	"github.com/sashabaranov/go-openai"
	"kandaoni.com/anqicms/config"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type OpenAIResult struct {
	Content string `json:"content"`
	Usage   int    `json:"usage"`
	Code    int    `json:"code"`
}

func (w *Website) SelfAiPseudoResult(req *AnqiAiRequest) (*AnqiAiRequest, error) {
	if !w.AiGenerateConfig.ApiValid {
		return nil, errors.New("接口不可用")
	}
	key := w.GetOpenAIKey()
	if key == "" {
		return nil, errors.New("无可用Key")
	}
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
		prompt := "请根据提供的内容完成内容重构，并保持段落结构：\n" + contentTexts[i]
		if req.Language == config.LanguageEn {
			prompt = "Please complete the content reconstruction according to the provided content, and keep the paragraph structure:\n" + contentTexts[i]
		}
		result, err := GetOpenAIResponse(key, prompt)
		if err != nil {
			if result.Code == 401 || result.Code == 429 {
				w.SetOpenAIKeyInvalid(key)
			}
			return nil, err
		}
		if len(result.Content) == 0 {
			return nil, errors.New("文本内容不足")
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
		} else {
			results[i] = "<p>" + results[i] + "</p>"
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

	return req, nil
}

func (w *Website) SelfAiGenerateResult(req *AnqiAiRequest) (*AnqiAiRequest, error) {
	if !w.AiGenerateConfig.ApiValid {
		return nil, errors.New("接口不可用")
	}
	key := w.GetOpenAIKey()
	if key == "" {
		return nil, errors.New("无可用Key")
	}

	prompt := "请根据关键词生成一篇中文文章，将文章标题放在第一行。关键词：" + req.Keyword
	if req.Language == config.LanguageEn {
		prompt = "Please generate an English article based on the keywords, and put the article title on the first line. Keywords: " + req.Keyword
	}
	if len(req.Demand) > 0 {
		prompt += "\n" + req.Demand
	}
	result, err := GetOpenAIResponse(key, prompt)
	if err != nil {
		if result.Code == 401 || result.Code == 429 {
			w.SetOpenAIKeyInvalid(key)
		}
		return nil, err
	}
	if len(result.Content) < 2 {
		return nil, errors.New("生成内容不足")
	}
	// 解析内容
	// 获取标题
	results := strings.Split(result.Content, "\n")
	title := results[0]
	if req.Language == config.LanguageEn && strings.Count(title, " ") > 20 && !strings.Contains(results[0], "Title:") {
		title = req.Keyword
	} else if req.Language == config.LanguageZh && utf8.RuneCountInString(title) > 50 && !strings.Contains(results[0], "标题：") {
		title = req.Keyword
	} else {
		results = results[1:]
	}
	// 标题替换：
	title = strings.TrimPrefix(title, "Title:")
	title = strings.TrimPrefix(title, "标题：")
	title = strings.TrimPrefix(title, "文章标题：")
	title = strings.Replace(title, "：", "，", 1)

	// 需要移除的关键词：
	var removeWords = []string{
		"首先，", "其次，", "再次，", "再者，", "最后，", "接下来，", "前言：", "另外，", "同时，", "因此，", "与此同时，", "事实上，", "除此之外，", "然而，", "此外，",
		"近年来，", "目前，", "不过，", "众所周知，", "那么，",
		"总之，", "结语，", "结语：", "结论，", "结论：", "总结，", "总结：", "综上所述，", "简单来说，", "总的来说，", "总结起来，", "总而言之，", "总体来讲，",
	}
	for i := 0; i < len(results); i++ {
		results[i] = strings.TrimSpace(results[i])
		for _, w := range removeWords {
			if strings.HasPrefix(results[i], w) {
				results[i] = strings.TrimPrefix(results[i], w)
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
		results[i] = "<p>" + results[i] + "</p>"
	}
	req.Title = title
	req.Content = strings.Join(results, "\n")

	return req, nil
}

func GetOpenAIResponse(apiKey, prompt string) (*OpenAIResult, error) {
	client := openai.NewClient(apiKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
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

func GetOpenAIStreamResponse(apiKey, prompt string) (*openai.ChatCompletionStream, error) {
	c := openai.NewClient(apiKey)
	ctx := context.Background()

	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Stream: true,
	}
	return c.CreateChatCompletionStream(ctx, req)
}
