package provider

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocarina/gocsv"
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"math"
	"mime/multipart"
	"regexp"
	"strings"
)

type AnchorCSV struct {
	Title  string `csv:"title"`
	Link   string `csv:"link"`
	Weight int    `csv:"weight"`
}

func GetAnchorList(keyword string, currentPage, pageSize int) ([]*model.Anchor, int64, error) {
	var anchors []*model.Anchor
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := config.DB.Model(&model.Anchor{}).Order("id desc")
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

func GetAllAnchors() ([]*model.Anchor, error) {
	var anchors []*model.Anchor
	err := config.DB.Model(&model.Anchor{}).Order("weight desc").Find(&anchors).Error
	if err != nil {
		return nil, err
	}

	return anchors, nil
}

func GetAnchorById(id uint) (*model.Anchor, error) {
	var anchor model.Anchor

	err := config.DB.Where("`id` = ?", id).First(&anchor).Error
	if err != nil {
		return nil, err
	}

	return &anchor, nil
}

func GetAnchorByTitle(title string) (*model.Anchor, error) {
	var anchor model.Anchor

	err := config.DB.Where("`title` = ?", title).First(&anchor).Error
	if err != nil {
		return nil, err
	}

	return &anchor, nil
}

func ImportAnchors(file multipart.File, info *multipart.FileHeader) (string, error) {
	var anchors []*AnchorCSV

	if err := gocsv.Unmarshal(file, &anchors); err != nil {
		return "", err
	}

	total := 0
	for _, item := range anchors {
		item.Title = strings.TrimSpace(item.Title)
		if item.Title == "" {
			continue
		}
		anchor, err := GetAnchorByTitle(item.Title)
		if err != nil {
			//表示不存在
			anchor = &model.Anchor{
				Title: item.Title,
				Status:       1,
			}
			total ++
		}
		anchor.Link = item.Link
		anchor.Weight = item.Weight

		anchor.Save(config.DB)
	}

	return fmt.Sprintf("成功导入了%d个锚文本", total), nil
}

func DeleteAnchor(anchor *model.Anchor) error {
	err := config.DB.Delete(anchor).Error
	if err != nil {
		return err
	}

	//清理已经存在的anchor
	go CleanAnchor(anchor.Id)

	return nil
}

func CleanAnchor(anchorId uint) {
	var anchorData []*model.AnchorData
	err := config.DB.Where("`anchor_id` = ?", anchorId).Find(&anchorData).Error
	if err != nil {
		return
	}

	anchorIdStr := fmt.Sprintf("%d", anchorId)

	for _, data := range anchorData {
		if data.ItemType == model.ItemTypeArticle {
			//处理article
			articleData, err := GetArticleDataById(data.ItemId)
			if err != nil {
				continue
			}
			htmlR := strings.NewReader(articleData.Content)
			doc, err := goquery.NewDocumentFromReader(htmlR)
			if err == nil {
				clean := false
				doc.Find("a,strong").Each(func(i int, s *goquery.Selection) {
					existsId, exists := s.Attr("data-anchor")

					if exists && existsId == anchorIdStr {
						//清理它
						s.Contents().Unwrap()
						clean = true
					}
				})
				//清理完毕，更新
				if clean {
					//更新内容
					articleData.Content, _ = doc.Find("body").Html()
					config.DB.Save(articleData)
				}
			}
			//删除当前item
			config.DB.Unscoped().Delete(data)
		} else if data.ItemType == model.ItemTypeProduct {
			//处理产品
			productData, err := GetProductDataById(data.ItemId)
			if err != nil {
				continue
			}
			htmlR := strings.NewReader(productData.Content)
			doc, err := goquery.NewDocumentFromReader(htmlR)
			if err == nil {
				clean := false
				doc.Find("a,strong").Each(func(i int, s *goquery.Selection) {
					existsId, exists := s.Attr("data-anchor")

					if exists && existsId == anchorIdStr {
						//清理它
						s.Contents().Unwrap()
						clean = true
					}
				})
				//清理完毕，更新
				if clean {
					//更新内容
					productData.Content, _ = doc.Find("body").Html()
					config.DB.Save(productData)
				}
			}
			//删除当前item,永久删除
			config.DB.Unscoped().Delete(data)
		}
	}
}

func ChangeAnchor(anchor *model.Anchor, changeTitle bool) {
	//如果锚文本更改了名称，需要移除已经生成锚文本
	if changeTitle {
		//清理anchor
		CleanAnchor(anchor.Id)

		//更新替换数量
		anchor.ReplaceCount = 0
		config.DB.Save(anchor)
		return
	}
	//其他当做更改了连接
	//如果锚文本只更改了连接，则需要重新替换新的连接
	var anchorData []*model.AnchorData
	err := config.DB.Where("`anchor_id` = ?", anchor.Id).Find(&anchorData).Error
	if err != nil {
		return
	}

	anchorIdStr := fmt.Sprintf("%d", anchor.Id)

	for _, data := range anchorData {
		if data.ItemType == model.ItemTypeArticle {
			//处理article
			articleData, err := GetArticleDataById(data.ItemId)
			if err != nil {
				continue
			}
			htmlR := strings.NewReader(articleData.Content)
			doc, err := goquery.NewDocumentFromReader(htmlR)
			if err == nil {
				update := false
				doc.Find("a").Each(func(i int, s *goquery.Selection) {
					existsId, exists := s.Attr("data-anchor")

					if exists && existsId == anchorIdStr {
						//换成新的链接
						s.SetAttr("href", anchor.Link)
						update = true
					}
				})
				//更新完毕，更新
				if update {
					//更新内容
					articleData.Content, _ = doc.Find("body").Html()
					config.DB.Save(articleData)
				}
			}
		} else if data.ItemType == model.ItemTypeProduct {
			//处理产品
			productData, err := GetProductDataById(data.ItemId)
			if err != nil {
				continue
			}
			htmlR := strings.NewReader(productData.Content)
			doc, err := goquery.NewDocumentFromReader(htmlR)
			if err == nil {
				update := false
				doc.Find("a").Each(func(i int, s *goquery.Selection) {
					existsId, exists := s.Attr("data-anchor")

					if exists && existsId == anchorIdStr {
						//换成新的链接
						s.SetAttr("href", anchor.Link)
						update = true
					}
				})
				//更新完毕，更新
				if update {
					//更新内容
					productData.Content, _ = doc.Find("body").Html()
					config.DB.Save(productData)
				}
			}
		}
	}
}

//单个替换
func ReplaceAnchor(anchor *model.Anchor) {
	//交由下方执行
	ReplaceAnchors([]*model.Anchor{anchor})
}

//批量替换
func ReplaceAnchors(anchors []*model.Anchor) {
	if len(anchors) == 0 {
		anchors, _ = GetAllAnchors()
		if len(anchors) == 0 {
			//没有关键词，终止执行
			return
		}
	}

	//先遍历文章、产品，添加锚文本
	//每次取100个
	limit := 100
	offset := 0
	var articles []*model.Article
	for {
		config.DB.Order("id asc").Limit(limit).Offset(offset).Find(&articles)
		if len(articles) == 0 {
			break
		}
		//加下一轮
		offset += limit
		for _, v := range articles {
			//执行替换
			link := GetUrl("article", v, 0)
			ReplaceContent(anchors, "article", v.Id, link)
		}
	}
	offset = 0
	var products []*model.Product
	for {
		config.DB.Order("id asc").Limit(limit).Offset(offset).Find(&products)
		if len(products) == 0 {
			break
		}
		//加下一轮
		offset += limit
		for _, v := range products {
			//执行替换
			link := GetUrl("product", v, 0)
			ReplaceContent(anchors, "product", v.Id, link)
		}
	}
}

func ReplaceContent(anchors []*model.Anchor, itemType string, itemId uint, link string) string {
	if len(anchors) == 0 {
		anchors, _ = GetAllAnchors()
		if len(anchors) == 0 {
			//没有关键词，终止执行
			return ""
		}
	}

	content := ""

	var err error
	var articleData *model.ArticleData
	var productData *model.ProductData

	if itemType == "article" {
		articleData, err = GetArticleDataById(itemId)
		if err != nil {
			return ""
		}
		content = articleData.Content
	} else if itemType == "product" {
		productData, err = GetProductDataById(itemId)
		if err != nil {
			return ""
		}
		content = productData.Content
	} else {
		//暂不支持其他
		return ""
	}

	//获取纯文本字数
	stripedContent := library.StripTags(content)
	contentLen := len([]rune(stripedContent))
	if config.JsonData.PluginAnchor.AnchorDensity < 10 {
		//默认设置100
		config.JsonData.PluginAnchor.AnchorDensity = 100
	}

	//最大可以替换的数量
	maxAnchorNum := int(math.Ceil(float64(contentLen)/float64(config.JsonData.PluginAnchor.AnchorDensity)))

	type replaceType struct {
		Key string
		Value string
	}

	existsKeywords := map[string]bool{}
	existsLinks :=  map[string]bool{}

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
	//过滤所有属性
	reg, _ = regexp.Compile("(?i)<[a-z0-9]+(\\s+[^>]+)>")
	content = reg.ReplaceAllStringFunc(content, func(s string) string {
		//保留标签
		reg := regexp.MustCompile("(?i)<[a-z0-9]+(\\s+[^>]+)>")
		match := reg.FindStringSubmatch(s)

		key := fmt.Sprintf("{$%d}", numCount)
		newStr := strings.Replace(s, match[1], key, 1)
		replacedMatch = append(replacedMatch, &replaceType{
			Key:   key,
			Value: match[1],
		})
		numCount++

		return newStr
	})

	if len(existsLinks) < maxAnchorNum {
		//开始替换关键词
		for _, anchor := range anchors {
			if anchor.Title == "" {
				continue
			}
			if anchor.Link == link {
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
					replaceHtml = fmt.Sprintf("<a href=\"%s\" data-anchor=\"%d\">%s</a>", anchor.Link, anchor.Id, s)
					key = fmt.Sprintf("{$%d}", numCount)

					//加入计数
					existsLinks[anchor.Link] = true
					existsKeywords[anchor.Title] = true
				} else {
					//其他则加粗
					replaceHtml = fmt.Sprintf("<strong data-anchor=\"%d\">%s</strong>", anchor.Id, s)
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
				config.DB.Save(anchorData)
				//更新计数
				var count int64
				config.DB.Model(&model.AnchorData{}).Where("`anchor_id` = ?", anchor.Id).Count(&count)
				anchor.ReplaceCount = count
				config.DB.Save(anchor)
			}

			//判断数量是否达到了，达到了就跳出
			if len(existsLinks) >= maxAnchorNum {
				break
			}
		}
	}

	//关键词替换完毕，将原来替换的重新替换回去，需要倒序
	for i := len(replacedMatch)-1; i>= 0; i-- {
		content = strings.Replace(content, replacedMatch[i].Key, replacedMatch[i].Value, 1)
	}

	if itemType == "article" {
		if !strings.EqualFold(articleData.Content, content) {
			//内容有更新，执行更新
			articleData.Content = content
			config.DB.Save(articleData)
		}
	} else if itemType == "product" {
		if !strings.EqualFold(productData.Content, content) {
			//内容有更新，执行更新
			productData.Content = content
			config.DB.Save(productData)
		}
	}

	return content
}

func AutoInsertAnchor(keywords, link string) {
	keywords = strings.ReplaceAll(keywords, "，", ",")
	keywords = strings.ReplaceAll(keywords, " ", ",")
	keywords = strings.ReplaceAll(keywords, "_", ",")

	keywordArr := strings.Split(keywords, ",")
	for _, v := range keywordArr {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		_, err := GetAnchorByTitle(v)
		if err != nil {
			//插入新的
			anchor := &model.Anchor{
				Title:        v,
				Link:         link,
				Status:       1,
			}
			config.DB.Save(anchor)
		}
	}
}