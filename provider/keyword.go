package provider

import (
	"encoding/json"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"mime/multipart"
	"strconv"
	"strings"
)

type KeywordCSV struct {
	Title      string `csv:"title"`
	CategoryId uint   `csv:"category_id"`
}

func (w *Website) GetUserKeywordSetting() config.KeywordJson {
	var keywordJson config.KeywordJson
	value := w.GetSettingValue(KeywordSettingKey)
	if value == "" {
		return keywordJson
	}

	if err := json.Unmarshal([]byte(value), &keywordJson); err != nil {
		return keywordJson
	}

	return keywordJson
}

func (w *Website) SaveUserKeywordSetting(req config.KeywordJson, focus bool) error {
	keywordJson := w.GetUserKeywordSetting()
	if focus {
		keywordJson = req
	} else {
		if req.TitleExclude != nil {
			keywordJson.TitleExclude = req.TitleExclude
		}
		if req.TitleReplace != nil {
			keywordJson.TitleReplace = req.TitleReplace
		}
		if req.Language != "" {
			keywordJson.Language = req.Language
		}
		if req.MaxCount > 0 {
			keywordJson.MaxCount = req.MaxCount
		}
	}
	if req.MaxCount == 0 {
		keywordJson.MaxCount = 100000
	}

	_ = w.SaveSettingValue(KeywordSettingKey, keywordJson)
	//重新读取配置
	w.LoadKeywordSetting(w.GetSettingValue(KeywordSettingKey))

	return nil
}

// 始终保持只有一个keyword任务
var digKeywordRunning = false

var maxWordsNum = int64(100000)

func (w *Website) GetKeywordList(keyword string, currentPage, pageSize int) ([]*model.Keyword, int64, error) {
	var keywords []*model.Keyword
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := w.DB.Model(&model.Keyword{}).Order("id desc")
	if keyword != "" {
		//模糊搜索
		builder = builder.Where("(`title` like ?)", "%"+keyword+"%")
	}

	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&keywords).Error
	if err != nil {
		return nil, 0, err
	}

	return keywords, total, nil
}

func (w *Website) GetAllKeywords() ([]*model.Keyword, error) {
	var keywords []*model.Keyword
	err := w.DB.Model(&model.Keyword{}).Order("id desc").Find(&keywords).Error
	if err != nil {
		return nil, err
	}

	return keywords, nil
}

func (w *Website) GetKeywordById(id uint) (*model.Keyword, error) {
	var keyword model.Keyword

	err := w.DB.Where("`id` = ?", id).First(&keyword).Error
	if err != nil {
		return nil, err
	}

	return &keyword, nil
}

func (w *Website) GetKeywordByTitle(title string) (*model.Keyword, error) {
	var keyword model.Keyword

	err := w.DB.Where("`title` = ?", title).First(&keyword).Error
	if err != nil {
		return nil, err
	}

	return &keyword, nil
}

func (w *Website) ImportKeywords(file multipart.File, info *multipart.FileHeader) (string, error) {
	buff, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(buff), "\n")
	var total int
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// 格式：title, category_id
		if i == 0 {
			continue
		}
		values := strings.Split(line, ",")
		if len(values) < 2 {
			continue
		}
		title := strings.TrimSpace(values[0])
		if title == "" {
			continue
		}
		keyword, err := w.GetKeywordByTitle(title)
		if err != nil {
			//表示不存在
			keyword = &model.Keyword{
				Title:  title,
				Status: 1,
			}
			total++
		}
		categoryId, _ := strconv.Atoi(values[1])
		keyword.CategoryId = uint(categoryId)

		keyword.Save(w.DB)
	}

	return w.Tr("SuccessfullyImportedKeywords", total), nil
}

func (w *Website) DeleteKeyword(keyword *model.Keyword) error {
	err := w.DB.Delete(keyword).Error
	if err != nil {
		return err
	}

	return nil
}

// StartDigKeywords 开始挖掘关键词，通过核心词来拓展
// 最多只10万关键词，抓取前3级，如果超过3级，则每次只执行一级
func (w *Website) StartDigKeywords(focus bool) {
	if w.DB == nil {
		return
	}
	if w.KeywordConfig.AutoDig == false && !focus {
		return
	}
	if digKeywordRunning {
		return
	}
	digKeywordRunning = true
	defer func() {
		digKeywordRunning = false
	}()

	collector := NewKeywordCollect(w)
	collector.Start()
}

func ContainKeywords(title, keyword string) bool {
	if len(title) <= 2 {
		return false
	}
	title = strings.ToLower(title)
	words := WordSplit(strings.ToLower(keyword), false)
	maxLen := 0
	matchLen := 0
	for _, wd := range words {
		maxLen += len(wd)
		if strings.Contains(title, wd) {
			matchLen += len(wd)
		}
	}
	if float64(matchLen)/float64(maxLen) >= 0.3 {
		return true
	}

	return false
}
