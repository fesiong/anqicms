package provider

import (
	"encoding/json"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"log"
	"math/rand"
	"os"
	"time"
)

func (w *Website) GetAiGenerateSetting() config.AiGenerateConfig {
	var setting config.AiGenerateConfig
	value := w.GetSettingValue(AiGenerateSettingKey)
	if value == "" {
		return setting
	}

	if err := json.Unmarshal([]byte(value), &setting); err != nil {
		return setting
	}

	return setting
}

func (w *Website) SaveAiGenerateSetting(req config.AiGenerateConfig, focus bool) error {
	setting := w.GetAiGenerateSetting()
	if focus {
		setting = req
		if req.AiEngine == config.AiEngineDeepSeek && setting.OpenAiApi == "" {
			// DeepSeek 接口地址使用默认的接口地址
			setting.OpenAiApi = "https://api.deepseek.com/v1"
			setting.OpenAIModel = "deepseek-chat"
		}
	} else {
		if req.ContentReplace != nil {
			setting.ContentReplace = req.ContentReplace
		}
		if req.CategoryId > 0 {
			setting.CategoryId = req.CategoryId
		}
		if req.StartHour > 0 {
			setting.StartHour = req.StartHour
		}
		if req.EndHour > 0 {
			setting.EndHour = req.EndHour
		}
		if req.DailyLimit > 0 {
			setting.DailyLimit = req.DailyLimit
		}
		if len(req.OpenAIKeys) > 0 {
			setting.OpenAIKeys = req.OpenAIKeys
		}
	}
	if len(setting.OpenAIKeys) > 0 {
		// 检查是否可用
	}

	_ = w.SaveSettingValue(AiGenerateSettingKey, setting)
	//重新读取配置
	w.LoadAiGenerateSetting(w.GetSettingValue(AiGenerateSettingKey))
	go func() {
		w.CheckOpenAIAPIValid()
		if setting.Open {
			go w.AiGenerateArticles()
		}
	}()

	return nil
}

var runningAiGenerateArticles = false

func (w *Website) AiGenerateArticles() {
	if w.DB == nil {
		return
	}
	if !w.AiGenerateConfig.Open {
		return
	}
	if runningAiGenerateArticles {
		return
	}
	runningAiGenerateArticles = true
	defer func() {
		runningAiGenerateArticles = false
	}()

	if w.AiGenerateConfig.StartHour > 0 && time.Now().Hour() < w.AiGenerateConfig.StartHour {
		return
	}

	if w.AiGenerateConfig.EndHour > 0 && time.Now().Hour() >= w.AiGenerateConfig.EndHour {
		return
	}

	// 如果采集的文章数量达到了设置的限制，则当天停止采集
	if w.GetTodayArticleCount(config.ArchiveFromAi) > int64(w.AiGenerateConfig.DailyLimit) {
		return
	}

	lastId := uint(0)
	for {
		var keywords []*model.Keyword
		w.DB.Where("id > ? and last_time = 0", lastId).Order("id asc").Limit(10).Find(&keywords)
		if len(keywords) == 0 {
			break
		}
		lastId = keywords[len(keywords)-1].Id
		for i := 0; i < len(keywords); i++ {
			keyword := keywords[i]
			// 检查是否采集过
			if w.checkArticleExists(keyword.Title, "", "") {
				// 跳过这个关键词
				if keyword.ArticleCount == 0 {
					keyword.ArticleCount = 1
				}
				keyword.LastTime = time.Now().Unix()
				w.DB.Model(keyword).Select("article_count", "last_time").Updates(keyword)
				continue
			}
			total, err := w.AiGenerateArticlesByKeyword(*keyword, false)
			log.Printf("关键词：%s 生成了 %d 篇文章, %v", keyword.Title, total, err)
			// 达到数量了，退出
			if w.GetTodayArticleCount(config.ArchiveFromAi) > int64(w.AiGenerateConfig.DailyLimit) {
				return
			}
			// 每个关键词都需要间隔30秒以上
			time.Sleep(time.Duration(20+rand.Intn(20)) * time.Second)
			if err != nil {
				// 采集出错了，多半是出验证码了，跳过该任务，等下次开始
				// 延时 10分钟以上
				// time.Sleep(time.Duration(10+rand.Intn(20)) * time.Minute)
				break
			}
		}
	}
}

func (w *Website) AiGenerateArticlesByKeyword(keyword model.Keyword, focus bool) (total int, err error) {
	total, err = w.AnqiAiGenerateArticle(&keyword)

	if err != nil {
		return total, err
	}
	if total == 0 {
		return total, nil
	}

	keyword.ArticleCount = w.GetArticleTotalByKeywordId(keyword.Id)
	keyword.LastTime = time.Now().Unix()
	w.DB.Model(keyword).Select("article_count", "last_time").Updates(keyword)

	return total, nil
}

func (w *Website) CheckOpenAIAPIValid() bool {
	// check what if this server can visit chatgpt
	ops := &library.Options{Timeout: 10}
	proxy := os.Getenv("HTTP_PROXY")
	if len(proxy) > 0 {
		ops.Proxy = proxy
	}
	link := "https://api.openai.com/v1"
	if w.AiGenerateConfig.OpenAiApi != "" {
		link = w.AiGenerateConfig.OpenAiApi
	}
	_, err := library.Request(link, ops)
	if err == nil {
		w.AiGenerateConfig.ApiValid = true
	} else {
		w.AiGenerateConfig.ApiValid = false
	}
	return w.AiGenerateConfig.ApiValid
}

// GetOpenAIKey 尝试获取一个可用的key
func (w *Website) GetOpenAIKey() string {
	if len(w.AiGenerateConfig.OpenAIKeys) == 0 {
		return ""
	}
	// 先获取有效的key
	var tmpKey string
	var tmpIndex int
	w.AiGenerateConfig.KeyIndex = (w.AiGenerateConfig.KeyIndex + 1) % len(w.AiGenerateConfig.OpenAIKeys)
	for i, key := range w.AiGenerateConfig.OpenAIKeys {
		if !key.Invalid {
			if tmpKey == "" {
				tmpKey = key.Key
				tmpIndex = i
			}
			if w.AiGenerateConfig.KeyIndex >= i {
				tmpKey = key.Key
				tmpIndex = i
				break
			}
		}
	}
	w.AiGenerateConfig.KeyIndex = tmpIndex

	return tmpKey
}

func (w *Website) SetOpenAIKeyInvalid(invalidKey string) {
	if len(w.AiGenerateConfig.OpenAIKeys) == 0 {
		return
	}
	for i, key := range w.AiGenerateConfig.OpenAIKeys {
		if key.Key == invalidKey {
			w.AiGenerateConfig.OpenAIKeys[i].Invalid = true
			break
		}
	}
}
