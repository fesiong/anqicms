package provider

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func (w *Website) SelfAiTranslateResult(req *AnqiAiRequest) (*AnqiAiRequest, error) {
	// 翻译标题
	if len(req.Title) > 0 {
		// 翻译标题
		// 使用AI翻译
		if w.PluginTranslate.Engine == config.TranslateEngineAi {
			tmpContent, err := w.SelfAiTranslate(req.Title, req.ToLanguage)
			if err != nil {
				return nil, err
			}
			req.Title = tmpContent
		} else if w.PluginTranslate.Engine == config.TranslateEngineBaidu {
			// 使用百度翻译
			baiduTranslate := NewBaiduTranslate(w.PluginTranslate.BaiduAppId, w.PluginTranslate.BaiduAppSecret)
			content, err := baiduTranslate.Translate(req.Title, req.Language, req.ToLanguage)
			if err != nil {
				return nil, err
			}
			req.Title = content
		} else if w.PluginTranslate.Engine == config.TranslateEngineYoudao {
			// 有道翻译
			youdaoTranslate := NewYoudaoTranslate(w.PluginTranslate.YoudaoAppKey, w.PluginTranslate.YoudaoAppSecret)
			content, err := youdaoTranslate.Translate(req.Title, req.Language, req.ToLanguage)
			if err != nil {
				return nil, err
			}
			req.Title = content
		} else if w.PluginTranslate.Engine == config.TranslateEngineDeepl {
			// Deepl翻译
			client := NewDeepl(w.PluginTranslate.DeeplAuthKey)
			glossariesId, err := client.GetGlossaries()
			if err != nil {
				return nil, err
			}
			translation, _, err := client.Translate(
				req.Title,
				req.Language,
				req.ToLanguage,
				glossariesId,
			)
			if err != nil {
				return nil, err
			}

			req.Title = translation
		} else {
			return nil, errors.New(w.Tr("NoAiGenerationSourceSelected"))
		}
	}
	// 翻译内容
	if len(req.Content) > 0 {
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
			// 使用AI翻译
			if w.PluginTranslate.Engine == config.TranslateEngineAi {
				tmpContent, err := w.SelfAiTranslate(contentTexts[i], req.ToLanguage)
				if err != nil {
					return nil, err
				}
				contentTexts[i] = tmpContent
			} else if w.PluginTranslate.Engine == config.TranslateEngineBaidu {
				// 百度的默认qps=1
				time.Sleep(time.Second)
				// 使用百度翻译
				baiduTranslate := NewBaiduTranslate(w.PluginTranslate.BaiduAppId, w.PluginTranslate.BaiduAppSecret)
				content, err := baiduTranslate.Translate(contentTexts[i], req.Language, req.ToLanguage)
				if err != nil {
					return nil, err
				}
				contentTexts[i] = content
			} else if w.PluginTranslate.Engine == config.TranslateEngineYoudao {
				// 有道翻译
				youdaoTranslate := NewYoudaoTranslate(w.PluginTranslate.YoudaoAppKey, w.PluginTranslate.YoudaoAppSecret)
				content, err := youdaoTranslate.Translate(contentTexts[i], req.Language, req.ToLanguage)
				if err != nil {
					return nil, err
				}
				contentTexts[i] = content
			} else if w.PluginTranslate.Engine == config.TranslateEngineDeepl {
				// Deepl翻译
				client := NewDeepl(w.PluginTranslate.DeeplAuthKey)
				glossariesId, err := client.GetGlossaries()
				if err != nil {
					return nil, err
				}
				translation, _, err := client.Translate(
					contentTexts[i],
					req.Language,
					req.ToLanguage,
					glossariesId,
				)
				if err != nil {
					return nil, err
				}

				contentTexts[i] = translation
			}
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
	}

	return req, nil
}

type BaiduTranslate struct {
	AppId     string
	AppSecret string
}

type BaiduTransResult struct {
	ErrorCode   string `json:"error_code"`
	ErrorMsg    string `json:"error_msg"`
	From        string `json:"from"`
	To          string `json:"to"`
	TransResult []struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	} `json:"trans_result"`
}

func NewBaiduTranslate(appId, appSecret string) *BaiduTranslate {
	return &BaiduTranslate{
		AppId:     appId,
		AppSecret: appSecret,
	}
}

func (b *BaiduTranslate) Translate(content string, fromLanguage string, toLanguage string) (string, error) {
	// 将请求参数中的 APPID(appid)， 翻译 query(q，注意为UTF-8编码)，随机数(salt)，以及平台分配的密钥(可在管理控制台查看) 按照 appid+q+salt+密钥的顺序拼接得到字符串 1。
	salt := library.GenerateRandString(5)
	query := url.Values{}
	query.Add("appid", b.AppId)
	query.Add("q", content)
	query.Add("salt", salt)
	query.Add("from", fromLanguage)
	query.Add("to", toLanguage)
	signStr := b.AppId + content + salt + b.AppSecret
	sign := library.Md5(signStr)
	query.Add("sign", sign)
	urlString := query.Encode()
	resp, err := http.Post("https://fanyi-api.baidu.com/api/trans/vip/translate", "application/x-www-form-urlencoded", strings.NewReader(urlString))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result BaiduTransResult
	_ = json.Unmarshal(body, &result)

	if result.ErrorCode != "" {
		return "", errors.New(result.ErrorMsg)
	}

	var translated string
	for _, item := range result.TransResult {
		translated += item.Dst
	}
	if len(translated) == 0 {
		return "", errors.New("empty result")
	}

	return translated, nil
}

type YoudaoTranslate struct {
	AppKey    string
	AppSecret string
}

type YoudaoTranslateResult struct {
	ErrorCode   string   `json:"errorCode"`
	Query       string   `json:"query"`
	Translation []string `json:"translation"`
	L           string   `json:"l"`
}

func NewYoudaoTranslate(appKey, appSecret string) *YoudaoTranslate {
	return &YoudaoTranslate{
		AppKey:    appKey,
		AppSecret: appSecret,
	}
}

func (yt *YoudaoTranslate) Translate(content string, fromLanguage string, toLanguage string) (string, error) {
	// 将请求参数中的 APPID(appid)， 翻译 query(q，注意为UTF-8编码)，随机数(salt)，以及平台分配的密钥(可在管理控制台查看) 按照 appid+q+salt+密钥的顺序拼接得到字符串 1。
	salt := library.GenerateRandString(5)
	if strings.ToLower(fromLanguage) == "zh-cn" {
		fromLanguage = "zh-CHS"
	} else if strings.ToLower(fromLanguage) == "zh-tw" {
		fromLanguage = "zh-CHT"
	}
	if strings.ToLower(toLanguage) == "zh-cn" {
		toLanguage = "zh-CHS"
	} else if strings.ToLower(toLanguage) == "zh-tw" {
		toLanguage = "zh-CHT"
	}
	curtime := strconv.FormatInt(time.Now().Unix(), 10)
	query := url.Values{}
	query.Add("appKey", yt.AppKey)
	query.Add("q", content)
	query.Add("salt", salt)
	query.Add("from", fromLanguage)
	query.Add("to", toLanguage)
	query.Add("curtime", curtime)
	query.Add("signType", "v3")
	// appKey + truncate(query) + salt + curtime + key;
	signStr := yt.AppKey + yt.truncate(content) + salt + curtime + yt.AppSecret
	sign := yt.encrypt(signStr)
	query.Add("sign", sign)
	urlString := query.Encode()
	resp, err := http.Post("https://openapi.youdao.com/api", "application/x-www-form-urlencoded", strings.NewReader(urlString))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result YoudaoTranslateResult
	_ = json.Unmarshal(body, &result)
	translated := strings.Join(result.Translation, "")
	if len(translated) == 0 {
		return "", errors.New("empty result")
	}

	return translated, nil
}

func (yt *YoudaoTranslate) truncate(q string) string {
	count := utf8.RuneCountInString(q)
	if count <= 20 {
		return q
	}
	runeText := []rune(q)
	return fmt.Sprintf("%s%d%s", string(runeText[0:10]), count, string(runeText[count-10:]))
}

func (yt *YoudaoTranslate) encrypt(strSrc string) string {
	bt := []byte(strSrc)
	bts := sha256.Sum256(bt)
	return hex.EncodeToString(bts[:])
}

func (yt *YoudaoTranslate) getInput(q string) string {
	str := []rune(q)
	strLen := len(str)
	if strLen <= 20 {
		return q
	} else {
		return string(str[:10]) + strconv.Itoa(strLen) + string(str[strLen-10:])
	}
}

func (yt *YoudaoTranslate) getUuid() string {
	b := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type Deepl struct {
	baseURL string
	apiKey  string
}

type TranslationPayload struct {
	Text       []string `json:"text"`
	TargetLang string   `json:"target_lang"`
	SourceLang string   `json:"source_lang"`
	GlossaryID string   `json:"glossary_id"`
}

type TranslationResponse struct {
	Translations []Translation `json:"translations"`
}

type Translation struct {
	DetectedSourceLanguage string `json:"detected_source_language"`
	Text                   string `json:"text"`
}

func NewDeepl(apiKey string) *Deepl {
	baseURL := "https://api.deepl.com/v2"
	if strings.HasSuffix(apiKey, ":fx") {
		baseURL = "https://api-free.deepl.com/v2"
	}

	return &Deepl{apiKey: apiKey, baseURL: baseURL}
}

func (d *Deepl) Translate(text string, sourceLang, targetLang string, glossaryID string) (string, string, error) {
	sourceLang = strings.ToUpper(sourceLang)
	targetLang = strings.ToUpper(targetLang)

	payload := TranslationPayload{
		Text:       []string{text},
		TargetLang: targetLang,
		SourceLang: sourceLang,
		GlossaryID: glossaryID,
	}

	j, err := json.Marshal(payload)

	if err != nil {
		return "", "", err
	}

	apiUrl := fmt.Sprintf("%s/translate", d.baseURL)

	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(j))

	if err != nil {
		return "", "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("DeepL-Auth-Key %s", d.apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	responseData := TranslationResponse{}

	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &responseData)

	if err != nil {
		return "", "", err
	}

	var result []string
	for _, item := range responseData.Translations {
		sourceLang = item.DetectedSourceLanguage
		result = append(result, item.Text)
	}

	return strings.Join(result, "\n"), sourceLang, nil
}

func (d *Deepl) GetGlossaries() (string, error) {

	url := fmt.Sprintf("%s/glossaries", d.baseURL)

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("DeepL-Auth-Key %s", d.apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	return buf.String(), nil
}

type CreateGlossaryPayload struct {
	Name          string `json:"name"`
	SourceLang    string `json:"source_lang"`
	TargetLang    string `json:"target_lang"`
	EntriesFormat string `json:"entries_format"`
	Entries       string `json:"entries"`
}

type Glossary struct {
	ID           string `json:"glossary_id"`
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	SourceLang   string `json:"source_lang"`
	TargetLang   string `json:"target_lang"`
	CreationTime string `json:"creation_time"`
	EntryCount   int    `json:"entry_count"`
}

func (d *Deepl) CreateGlossary(name, sourceLang, targetLang string, entriesTSV io.Reader) (*Glossary, error) {

	buf := new(strings.Builder)
	_, err := io.Copy(buf, entriesTSV)

	if err != nil {
		return nil, err
	}

	entries := strings.ReplaceAll(buf.String(), "\r\n", "\n")

	payload := CreateGlossaryPayload{
		Name:          name,
		SourceLang:    sourceLang,
		TargetLang:    targetLang,
		EntriesFormat: "tsv",
		Entries:       entries,
	}

	j, err := json.Marshal(payload)

	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/glossaries", d.baseURL)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("DeepL-Auth-Key %s", d.apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	createdGlossary := Glossary{}

	err = json.NewDecoder(resp.Body).Decode(&createdGlossary)

	if err != nil {
		return nil, err
	}

	return &createdGlossary, nil
}
