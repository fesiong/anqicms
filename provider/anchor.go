package provider

import (
	"fmt"
	"io"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"math"
	"mime/multipart"
	"regexp"
	"strconv"
	"strings"
)

type AnchorCSV struct {
	Title  string `csv:"title"`
	Link   string `csv:"link"`
	Weight int    `csv:"weight"`
}

func (w *Website) GetAnchorList(keyword string, currentPage, pageSize int) ([]*model.Anchor, int64, error) {
	var anchors []*model.Anchor
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := w.DB.Model(&model.Anchor{}).Order("id desc")
	if keyword != "" {
		//模糊搜索
		builder = builder.Where("(`title` like ? OR `link` like ?)", "%"+keyword+"%", "%"+keyword+"%")
	}

	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&anchors).Error
	if err != nil {
		return nil, 0, err
	}

	return anchors, total, nil
}

func (w *Website) GetAllAnchors() ([]*model.Anchor, error) {
	var anchors []*model.Anchor
	err := w.DB.Model(&model.Anchor{}).Order("weight desc").Find(&anchors).Error
	if err != nil {
		return nil, err
	}

	return anchors, nil
}

func (w *Website) GetAnchorById(id uint) (*model.Anchor, error) {
	var anchor model.Anchor

	err := w.DB.Where("`id` = ?", id).First(&anchor).Error
	if err != nil {
		return nil, err
	}

	return &anchor, nil
}

func (w *Website) GetAnchorByTitle(title string) (*model.Anchor, error) {
	var anchor model.Anchor

	err := w.DB.Where("`title` = ?", title).First(&anchor).Error
	if err != nil {
		return nil, err
	}

	return &anchor, nil
}

func (w *Website) ImportAnchors(file multipart.File, info *multipart.FileHeader) (string, error) {
	buff, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(buff), "\n")
	var total int
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// 格式：title, link, weight
		if i == 0 {
			continue
		}
		values := strings.Split(line, ",")
		if len(values) < 3 {
			continue
		}
		title := strings.TrimSpace(values[0])
		if title == "" {
			continue
		}
		anchor, err := w.GetAnchorByTitle(title)
		if err != nil {
			//表示不存在
			anchor = &model.Anchor{
				Title:  title,
				Status: 1,
			}
			total++
		}
		anchor.Link = strings.TrimPrefix(values[1], w.System.BaseUrl)
		anchor.Weight, _ = strconv.Atoi(values[2])

		anchor.Save(w.DB)
	}

	return w.Tr("SuccessfullyImportedAnchorTexts", total), nil
}

func (w *Website) DeleteAnchor(anchor *model.Anchor) error {
	err := w.DB.Delete(anchor).Error
	if err != nil {
		return err
	}

	//清理已经存在的anchor
	go w.CleanAnchor(anchor)

	return nil
}

func (w *Website) CleanAnchor(anchor *model.Anchor) {
	var anchorData []*model.AnchorData
	err := w.DB.Where("`anchor_id` = ?", anchor.Id).Find(&anchorData).Error
	if err != nil {
		return
	}

	anchorIdStr := fmt.Sprintf("%d", anchor.Id)

	for _, data := range anchorData {
		//处理archive
		archiveData, err := w.GetArchiveDataById(data.ItemId)
		if err != nil {
			continue
		}
		// 清理 anchor，由于不支持匹配模式 \1 ，因此，需要分开执行
		clean := false
		// a
		re, _ := regexp.Compile(`(?i)<a.*?data-anchor="(\d+)".*?>(.*?)</a>`)
		archiveData.Content = re.ReplaceAllStringFunc(archiveData.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 3 {
				return s
			}
			if match[1] == anchorIdStr {
				//清理它
				clean = true
				return match[2]
			}
			return s
		})
		// strong
		re, _ = regexp.Compile(`(?i)<strong.*?data-anchor="(\d+)".*?>(.*?)</strong>`)
		archiveData.Content = re.ReplaceAllStringFunc(archiveData.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 3 {
				return s
			}
			if match[1] == anchorIdStr {
				//清理它
				clean = true
				return match[2]
			}
			return s
		})
		// 清理Markdown
		// [keyword](url)
		re, _ = regexp.Compile(`(?i)(.?)\[(.*?)]\((.*?)\)`)
		archiveData.Content = re.ReplaceAllStringFunc(archiveData.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 4 {
				return s
			}
			if match[2] == anchor.Title && match[3] == anchor.Link {
				//清理它
				clean = true
				return match[2]
			}
			return s
		})
		// **Keyword**
		re, _ = regexp.Compile(`(?i)\*\*(.*?)\*\*`)
		archiveData.Content = re.ReplaceAllStringFunc(archiveData.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 2 {
				return s
			}
			if match[1] == anchor.Title {
				//清理它
				clean = true
				return match[1]
			}
			return s
		})
		//清理完毕，更新
		if clean {
			//更新内容
			w.DB.Save(archiveData)
		}
		//删除当前item
		w.DB.Unscoped().Delete(data)
	}
}

func (w *Website) ChangeAnchor(anchor *model.Anchor, changeTitle bool) {
	//如果锚文本更改了名称，需要移除已经生成锚文本
	if changeTitle {
		//清理anchor
		w.CleanAnchor(anchor)

		//更新替换数量
		anchor.ReplaceCount = 0
		w.DB.Save(anchor)
		return
	}
	//其他当做更改了连接
	//如果锚文本只更改了连接，则需要重新替换新的连接
	var anchorData []*model.AnchorData
	err := w.DB.Where("`anchor_id` = ?", anchor.Id).Find(&anchorData).Error
	if err != nil {
		return
	}

	anchorIdStr := fmt.Sprintf("%d", anchor.Id)

	for _, data := range anchorData {
		//处理archive
		archiveData, err := w.GetArchiveDataById(data.ItemId)
		if err != nil {
			continue
		}
		update := false
		re, _ := regexp.Compile(`(?i)<a.*?data-anchor="(\d+)".*?>(.*?)</a>`)
		archiveData.Content = re.ReplaceAllStringFunc(archiveData.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 3 {
				return s
			}
			if match[1] == anchorIdStr {
				// 更换链接
				re2, _ := regexp.Compile(`(?i)<a.*?href="(.+?)".*?>(.*?)</a>`)
				match = re2.FindStringSubmatch(s)
				if len(match) > 2 {
					update = true
					s = strings.Replace(s, match[1], anchor.Link, 1)
				}
			}
			return s
		})
		// [keyword](url)
		re, _ = regexp.Compile(`(?i)(.?)\[(.*?)]\((.*?)\)`)
		archiveData.Content = re.ReplaceAllStringFunc(archiveData.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 4 {
				return s
			}
			if match[2] == anchor.Title && match[1] != "!" {
				//更换链接
				s = strings.Replace(s, match[3], anchor.Link, 1)
			}
			return s
		})
		//更新完毕，更新
		if update {
			//更新内容
			w.DB.Save(archiveData)
		}
	}
}

// ReplaceAnchor 单个替换
func (w *Website) ReplaceAnchor(anchor *model.Anchor) {
	//交由下方执行
	if anchor == nil {
		w.ReplaceAnchors(nil)
	} else {
		w.ReplaceAnchors([]*model.Anchor{anchor})
	}
}

// ReplaceAnchors 批量替换
func (w *Website) ReplaceAnchors(anchors []*model.Anchor) {
	if len(anchors) == 0 {
		anchors, _ = w.GetAllAnchors()
		if len(anchors) == 0 {
			//没有关键词，终止执行
			return
		}
	}

	//先遍历文章、产品，添加锚文本
	//每次取100个
	limit := 100
	lastId := int64(0)
	var archives []*model.Archive

	for {
		w.DB.Where("`id` > ?", lastId).Order("id asc").Limit(limit).Find(&archives)
		if len(archives) == 0 {
			break
		}
		//加下一轮
		lastId = archives[len(archives)-1].Id
		for _, v := range archives {
			//执行替换
			link := w.GetUrl("archive", v, 0)
			w.ReplaceContent(anchors, "archive", v.Id, link)
		}
	}
}

func (w *Website) ReplaceContent(anchors []*model.Anchor, itemType string, itemId int64, link string) string {
	link = strings.TrimPrefix(link, w.System.BaseUrl)
	if len(anchors) == 0 {
		anchors, _ = w.GetAllAnchors()
		if len(anchors) == 0 {
			//没有关键词，终止执行
			return ""
		}
	}

	content := ""

	archiveData, err := w.GetArchiveDataById(itemId)
	if err != nil {
		return ""
	}
	content = archiveData.Content

	//获取纯文本字数
	stripedContent := library.StripTags(content)
	contentLen := len([]rune(stripedContent))
	if w.PluginAnchor.AnchorDensity < 20 {
		//默认设置200
		w.PluginAnchor.AnchorDensity = 200
	}

	// 判断是否是Markdown，如果开头是标签，则认为不是Markdown
	isMarkdown := false
	if !strings.HasPrefix(strings.TrimSpace(content), "<") {
		isMarkdown = true
	}
	//最大可以替换的数量
	maxAnchorNum := int(math.Ceil(float64(contentLen) / float64(w.PluginAnchor.AnchorDensity)))

	type replaceType struct {
		Key   string
		Value string
	}

	existsKeywords := map[string]bool{}
	existsLinks := map[string]bool{}

	var replacedMatch []*replaceType
	numCount := 0
	//所有的a标签计数，并替换掉
	reg, _ := regexp.Compile("(?i)<a[^>]*>(.*?)</a>")
	content = reg.ReplaceAllStringFunc(content, func(s string) string {

		reg := regexp.MustCompile("(?i)<a\\s*[^>]*href=[\"']?([^\"']*)[\"']?[^>]*>(.*?)</a>")
		match := reg.FindStringSubmatch(s)
		if len(match) > 2 {
			existsKeywords[strings.ToLower(match[2])] = true
			existsLinks[strings.ToLower(match[1])] = true
		}

		key := fmt.Sprintf("{$%d}", numCount)
		replacedMatch = append(replacedMatch, &replaceType{
			Key:   key,
			Value: s,
		})
		numCount++

		return key
	})
	//所有的strong标签替换掉
	reg, _ = regexp.Compile("(?i)<strong[^>]*>(.*?)</strong>")
	content = reg.ReplaceAllStringFunc(content, func(s string) string {
		key := fmt.Sprintf("{$%d}", numCount)
		replacedMatch = append(replacedMatch, &replaceType{
			Key:   key,
			Value: s,
		})
		numCount++

		return key
	})
	// [keyword](url)
	reg, _ = regexp.Compile(`(?i)(.?)\[(.*?)]\((.*?)\)`)
	content = reg.ReplaceAllStringFunc(content, func(s string) string {
		match := reg.FindStringSubmatch(s)
		if len(match) > 2 && match[1] != "!" {
			existsKeywords[strings.ToLower(match[2])] = true
			existsLinks[strings.ToLower(match[3])] = true
		}

		key := fmt.Sprintf("{$%d}", numCount)
		replacedMatch = append(replacedMatch, &replaceType{
			Key:   key,
			Value: s,
		})
		numCount++

		return key
	})
	// **Keyword**
	reg, _ = regexp.Compile(`(?i)\*\*(.*?)\*\*`)
	content = reg.ReplaceAllStringFunc(content, func(s string) string {
		key := fmt.Sprintf("{$%d}", numCount)
		replacedMatch = append(replacedMatch, &replaceType{
			Key:   key,
			Value: s,
		})
		numCount++

		return key
	})
	//过滤所有属性
	reg, _ = regexp.Compile("(?i)</?[a-z0-9]+(\\s+[^>]+)>")
	content = reg.ReplaceAllStringFunc(content, func(s string) string {
		key := fmt.Sprintf("{$%d}", numCount)
		replacedMatch = append(replacedMatch, &replaceType{
			Key:   key,
			Value: s,
		})
		numCount++

		return key
	})

	if len(existsLinks) < maxAnchorNum {
		//开始替换关键词
		for _, anchor := range anchors {
			if anchor.Title == "" {
				continue
			}
			if strings.HasSuffix(anchor.Link, link) {
				//当前url，跳过
				continue
			}
			//已经存在存在的关键词，或者链接，跳过
			if existsKeywords[strings.ToLower(anchor.Title)] || existsLinks[strings.ToLower(anchor.Link)] {
				continue
			}
			//开始替换
			replaceNum := 0
			replacer := strings.NewReplacer("\\", "\\\\", "/", "\\/", "{", "\\{", "}", "\\}", "^", "\\^", "$", "\\$", "*", "\\*", "+", "\\+", "?", "\\?", ".", "\\.", "|", "\\|", "-", "\\-", "[", "\\[", "]", "\\]", "(", "\\(", ")", "\\)")
			matchName := replacer.Replace(anchor.Title)

			reg, _ = regexp.Compile(fmt.Sprintf("(?i)%s", matchName))
			content = reg.ReplaceAllStringFunc(content, func(s string) string {
				replaceHtml := ""
				key := ""
				if replaceNum == 0 {
					//第一条替换为锚文本
					if isMarkdown {
						replaceHtml = fmt.Sprintf("[%s](%s)", s, anchor.Link)
					} else {
						replaceHtml = fmt.Sprintf("<a href=\"%s\" data-anchor=\"%d\">%s</a>", anchor.Link, anchor.Id, s)
					}
					key = fmt.Sprintf("{$%d}", numCount)

					//加入计数
					existsLinks[anchor.Link] = true
					existsKeywords[anchor.Title] = true
				} else {
					//其他则加粗
					if isMarkdown {
						replaceHtml = fmt.Sprintf("**%s**", s)
					} else {
						replaceHtml = fmt.Sprintf("<strong data-anchor=\"%d\">%s</strong>", anchor.Id, s)
					}
					key = fmt.Sprintf("{$%d}", numCount)
				}
				replaceNum++

				replacedMatch = append(replacedMatch, &replaceType{
					Key:   key,
					Value: replaceHtml,
				})
				numCount++

				return key
			})

			//如果有更新了，则记录
			if replaceNum > 0 {
				//插入记录
				anchorData := &model.AnchorData{
					AnchorId: anchor.Id,
					ItemType: itemType,
					ItemId:   itemId,
				}
				w.DB.Save(anchorData)
				//更新计数
				var count int64
				w.DB.Model(&model.AnchorData{}).Where("`anchor_id` = ?", anchor.Id).Count(&count)
				anchor.ReplaceCount = count
				w.DB.Save(anchor)
			}

			//判断数量是否达到了，达到了就跳出
			if len(existsLinks) >= maxAnchorNum {
				break
			}
		}
	}

	//关键词替换完毕，将原来替换的重新替换回去，需要倒序
	for i := len(replacedMatch) - 1; i >= 0; i-- {
		content = strings.Replace(content, replacedMatch[i].Key, replacedMatch[i].Value, 1)
	}

	if !strings.EqualFold(archiveData.Content, content) {
		//内容有更新，执行更新
		archiveData.Content = content
		w.DB.Save(archiveData)
	}

	return content
}

func (w *Website) AutoInsertAnchor(archiveId int64, keywords, link string) {
	link = strings.TrimPrefix(link, w.System.BaseUrl)
	keywords = strings.ReplaceAll(keywords, "，", ",")
	keywords = strings.ReplaceAll(keywords, "_", ",")

	keywordArr := strings.Split(keywords, ",")
	for _, v := range keywordArr {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		_, err := w.GetAnchorByTitle(v)
		if err != nil {
			//插入新的
			anchor := &model.Anchor{
				Title:     v,
				Link:      link,
				ArchiveId: archiveId,
				Status:    1,
			}
			w.DB.Save(anchor)
		}
	}
}

func (w *Website) InsertTitleToAnchor(req *request.PluginAnchorAddFromTitle) error {
	if req.Type == "category" {
		// from category
		var categories []*model.Category
		w.DB.Where("`id` IN (?)", req.Ids).Find(&categories)
		for _, category := range categories {
			category.Link = w.GetUrl("category", category, 0)
			w.AutoInsertAnchor(0, category.Title, category.Link)
		}
	} else if req.Type == "archive" {
		var archives []*model.Archive
		w.DB.Where("`id` IN (?)", req.Ids).Find(&archives)
		for _, archive := range archives {
			archive.Link = w.GetUrl("archive", archive, 0)
			w.AutoInsertAnchor(archive.Id, archive.Title, archive.Link)
		}
	}

	return nil
}
