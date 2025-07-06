package provider

import (
	"bytes"
	"fmt"
	"golang.org/x/net/html"
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

const AnchorCacheKey = "anchor_list"

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
	// 先从缓存中读取
	err := w.Cache.Get(AnchorCacheKey, &anchors)
	if err == nil {
		return anchors, nil
	}
	err = w.DB.Model(&model.Anchor{}).Order("weight desc").Find(&anchors).Error
	if err != nil {
		return nil, err
	}
	_ = w.Cache.Set(AnchorCacheKey, anchors, 3600)

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
	w.Cache.Delete(AnchorCacheKey)

	return w.Tr("SuccessfullyImportedAnchorTexts", total), nil
}

func (w *Website) DeleteAnchor(anchor *model.Anchor) error {
	err := w.DB.Delete(anchor).Error
	if err != nil {
		return err
	}
	w.Cache.Delete(AnchorCacheKey)
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
	w.Cache.Delete(AnchorCacheKey)
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
	w.Cache.Delete(AnchorCacheKey)
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

func (w *Website) ReplaceContentText(anchors []*model.Anchor, content string, link string) (string, bool) {
	//content = html.UnescapeString(content)
	link = strings.TrimPrefix(link, w.System.BaseUrl)
	if len(anchors) == 0 {
		anchors, _ = w.GetAllAnchors()
		if len(anchors) == 0 {
			//没有关键词，终止执行
			return content, false
		}
	}

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

	isModified := false

	existsKeywords := make(map[string]bool, maxAnchorNum*2)
	if isMarkdown {
		// Markdown的处理方式
		var replaceMdText = func(text string, addStrongTag bool) string {
			for _, anchor := range anchors {
				// 创建匹配关键词的正则表达式（考虑单词边界）
				pattern := regexp.QuoteMeta(anchor.Title)
				if CheckContentIsEnglish(anchor.Title) {
					pattern = `(?i)\b` + regexp.QuoteMeta(anchor.Title) + `\b`
				}
				re, err := regexp.Compile(pattern)
				if err != nil {
					continue
				}
				// 查找所有匹配项
				matchIdx := re.FindAllStringIndex(text, -1)
				if matchIdx != nil {
					// 如果是不需要匹配的，则不处理
					if _, ok2 := existsKeywords[anchor.Title]; !ok2 {
						isModified = true
						existsKeywords[anchor.Title] = true
						var subText bytes.Buffer
						lastIndex := 0
						// 只处理1次
						for k, match := range matchIdx {
							// 添加匹配前的文本
							subText.WriteString(text[lastIndex:match[0]])
							// 添加锚文本链接, strong
							if k == 0 {
								subText.WriteString(fmt.Sprintf("[%s](%s)", text[match[0]:match[1]], anchor.Link))
							} else if addStrongTag {
								subText.WriteString(fmt.Sprintf("**%s**", text[match[0]:match[1]]))
							} else {
								subText.WriteString(text[match[0]:match[1]])
							}
							lastIndex = match[1]
						}
						// 添加剩余文本
						subText.WriteString(text[lastIndex:])
						text = subText.String()
					} else if addStrongTag {
						isModified = true
						// 需要添加 strong
						var subText bytes.Buffer
						lastIndex := 0
						for _, match := range matchIdx {
							// 添加匹配前的文本
							subText.WriteString(text[lastIndex:match[0]])
							// 添加** **
							subText.WriteString(fmt.Sprintf("**%s**", text[match[0]:match[1]]))
							lastIndex = match[1]
						}
						// 添加剩余文本
						subText.WriteString(text[lastIndex:])
						text = subText.String()
					}
				}
			}

			return text
		}

		reg, _ := regexp.Compile("(?i)<a[^>]*>(.*?)</a>")
		matches := reg.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 2 {
				existsKeywords[strings.ToLower(match[2])] = true
			}
		}
		// [keyword](url)
		reg, _ = regexp.Compile(`(?i)(.?)\[(.*?)]\((.*?)\)`)
		matches = reg.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 2 && match[1] != "!" {
				existsKeywords[strings.ToLower(match[2])] = true
			}
		}
		var newText bytes.Buffer
		// 逐行处理
		skipLineStart := -1
		contents := strings.Split(strings.TrimSpace(content), "\n")
		for i, line := range contents {
			// 跳过 # 开头的行，连续跳过 ``` 的行
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				newText.WriteString(line + "\n")
				continue
			} else if strings.HasPrefix(strings.TrimSpace(line), "```") {
				if skipLineStart == -1 {
					skipLineStart = i
				} else {
					skipLineStart = -1
				}
				newText.WriteString(line + "\n")
				continue
			} else if skipLineStart != -1 {
				newText.WriteString(line + "\n")
				continue
			}
			// 跳过html标签
			if strings.Contains(line, "<") && strings.Contains(line, ">") {
				newText.WriteString(line + "\n")
				continue
			}
			if len(existsKeywords) >= maxAnchorNum {
				// 已达到上限，不再继续
				newText.WriteString(line + "\n")
				continue
			}
			// 跳过 ` `
			re, _ := regexp.Compile("`.*`")
			matchIdx := re.FindAllStringIndex(line, -1)
			if len(matchIdx) > 0 {
				var subText bytes.Buffer
				lastIndex := 0
				for _, match := range matchIdx {
					// 添加匹配前的文本
					subText.WriteString(replaceMdText(line[lastIndex:match[0]], w.PluginAnchor.NoStrongTag == 0))
					subText.WriteString(line[match[0]:match[1]])
					lastIndex = match[1]
				}
				// 添加剩余文本
				subText.WriteString(replaceMdText(line[lastIndex:], w.PluginAnchor.NoStrongTag == 0))
				line = subText.String()
			} else {
				// 整段处理
				line = replaceMdText(line, w.PluginAnchor.NoStrongTag == 0)
			}
			newText.WriteString(line)
			newText.WriteString("\n")
		}
		if isModified {
			content = newText.String()
		}
	} else {
		// 处理html
		doc, err := html.Parse(strings.NewReader(content))
		if err != nil {
			return content, false
		}
		// 创建已处理节点的映射，避免重复替换
		processedNodes := make(map[*html.Node]bool)
		skipTags := map[string]bool{"a": true, "code": true, "pre": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true}
		// 遍历并处理文本节点
		var traverse func(*html.Node)
		traverse = func(n *html.Node) {
			if n.Type == html.TextNode && !isInsideTag(n, skipTags) && !processedNodes[n] {
				if len(existsKeywords) >= maxAnchorNum {
					// 已达到上限，不再继续
					return
				}
				text := n.Data
				modified := false

				for _, anchor := range anchors {
					// 创建匹配关键词的正则表达式（考虑单词边界）
					pattern := regexp.QuoteMeta(anchor.Title)
					if CheckContentIsEnglish(anchor.Title) {
						pattern = `(?i)\b` + regexp.QuoteMeta(anchor.Title) + `\b`
					}
					re, err := regexp.Compile(pattern)
					if err != nil {
						continue
					}

					// 查找所有匹配项
					matches := re.FindAllStringIndex(text, -1)

					if matches != nil {
						// 如果是不需要匹配的，则不处理
						if _, ok2 := existsKeywords[anchor.Title]; !ok2 {
							existsKeywords[anchor.Title] = true
							modified = true
							var newText bytes.Buffer
							lastIndex := 0
							// 只处理1次
							for k, match := range matches {
								// 添加匹配前的文本
								newText.WriteString(text[lastIndex:match[0]])
								// 添加锚文本链接
								if k == 0 {
									newText.WriteString(fmt.Sprintf("<a href=\"%s\" data-anchor=\"%d\">%s</a>", anchor.Link, anchor.Id, text[match[0]:match[1]]))
								} else if w.PluginAnchor.NoStrongTag == 0 {
									// 加粗
									newText.WriteString(fmt.Sprintf("<strong data-anchor=\"%d\">%s</strong>", anchor.Id, text[match[0]:match[1]]))
								} else {
									newText.WriteString(text[match[0]:match[1]])
								}
								lastIndex = match[1]
							}
							// 添加剩余文本
							newText.WriteString(text[lastIndex:])
							text = newText.String()
						} else if w.PluginAnchor.NoStrongTag == 0 {
							modified = true
							var newText bytes.Buffer
							lastIndex := 0
							// 只处理1次
							for _, match := range matches {
								// 添加匹配前的文本
								newText.WriteString(text[lastIndex:match[0]])
								// 加粗
								newText.WriteString(fmt.Sprintf("<strong data-anchor=\"%d\">%s</strong>", anchor.Id, text[match[0]:match[1]]))
								lastIndex = match[1]
							}
							// 添加剩余文本
							newText.WriteString(text[lastIndex:])
							text = newText.String()
						}
					}
				}

				if modified {
					isModified = true
					// 创建新的文本节点
					parent := n.Parent
					newNodes, err := html.ParseFragment(strings.NewReader(text), parent)
					if err != nil {
						return
					}

					// 替换原节点
					for _, newNode := range newNodes {
						parent.InsertBefore(newNode, n)
						processedNodes[newNode] = true
					}
					parent.RemoveChild(n)
					return
				}
			}

			// 递归处理子节点
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				traverse(c)
			}
		}

		traverse(doc)
		if isModified {
			// 将处理后的文档转换回字符串
			var buf bytes.Buffer
			err = html.Render(&buf, doc)
			if err != nil {
				return content, false
			}
			content = buf.String()
			// 只要body部分
			re2, _ := regexp.Compile(`(?is)<body>(.*)</body>`)
			content2 := re2.FindStringSubmatch(content)
			if len(content2) > 0 {
				content = content2[1]
			}
		}
	}

	return content, isModified
}

// isInsideTag 检查节点是否在指定节点内
func isInsideTag(n *html.Node, tagNames map[string]bool) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && tagNames[p.Data] {
			return true
		}
	}
	return false
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

	archiveData, err := w.GetArchiveDataById(itemId)
	if err != nil {
		return ""
	}
	content, isModified := w.ReplaceContentText(anchors, archiveData.Content, link)

	if isModified {
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
	w.Cache.Delete(AnchorCacheKey)
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
	w.Cache.Delete(AnchorCacheKey)

	return nil
}
