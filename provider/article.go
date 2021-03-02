package provider

import (
	"github.com/PuerkitoBio/goquery"
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"irisweb/request"
	"net/url"
	"strings"
)

func SaveArticle(req *request.Article) (article *model.Article, err error) {
	var category *model.Category
	//检查分类
	if req.CategoryName != "" {
		category, err = GetCategoryByTitle(req.CategoryName)
		if err != nil {
			category = &model.Category{
				Title:  req.CategoryName,
				Status: 1,
			}
			err = category.Save(config.DB)
			if err != nil {
				return
			}
		}
		if category != nil {
			req.CategoryId = category.Id
		}
	}

	newPost := false
	if req.Id > 0 {
		article, err = GetArticleById(req.Id)
		if err != nil {
			return nil, err
		}

		if article.ArticleData == nil {
			article.ArticleData = &model.ArticleData{}
		}
		article.ArticleData.Content = req.Content
	} else {
		newPost = true
		newToken := library.GetPinyin(req.Title)
		_, err := CheckArticleByUrlToken(newToken)
		if err == nil {
			//增加随机
			newToken += library.GenerateRandString(3)
		}

		article = &model.Article{
			Status:   1,
			UrlToken: newToken,
			ArticleData: &model.ArticleData{
				Content: req.Content,
			},
		}
	}
	article.Title = req.Title
	article.Keywords = req.Keywords
	article.Description = req.Description
	article.CategoryId = req.CategoryId
	article.Images = req.Images

	//goquery
	htmlR := strings.NewReader(req.Content)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err == nil {
		baseHost := ""
		urls, err := url.Parse(config.JsonData.System.BaseUrl)
		if err == nil {
			baseHost = urls.Host
		}

		//提取描述
		if req.Description == "" {
			textRune := []rune(strings.TrimSpace(doc.Text()))
			if len(textRune) > 150 {
				article.Description = string(textRune[:150])
			} else {
				article.Description = string(textRune)
			}
		}
		//下载远程图片
		if config.JsonData.Content.RemoteDownload == 1 {
			doc.Find("img").Each(func(i int, s *goquery.Selection) {
				src, exists := s.Attr("src")
				if exists {
					alt := s.AttrOr("alt", "")
					imgUrl, err := url.Parse(src)
					if err == nil {
						if imgUrl.Host != "" && imgUrl.Host != baseHost {
							//外链
							attachment, err := DownloadRemoteImage(src, alt)
							if err == nil {
								s.SetAttr("src", attachment.Logo)
							}
						}
					}
				}
			})
		}
		//提取缩略图
		if len(article.Images) == 0 {
			imgSections := doc.Find("img")
			if imgSections.Length() > 0 {
				//获取第一条
				article.Images = append(article.Images, imgSections.Eq(0).AttrOr("src", ""))
			}
		}
		for i, v := range article.Images {
			article.Images[i] = strings.Replace(v, config.JsonData.System.BaseUrl, "", -1)
		}

		//过滤外链
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists {
				aUrl, err := url.Parse(href)
				if err == nil {
					if aUrl.Host != "" && aUrl.Host != baseHost {
						//外链
						if config.JsonData.Content.FilterOutlink == 1 {
							//过滤外链
							s.Contents().Unwrap()
						} else {
							//增加nofollow
							s.SetAttr("rel", "nofollow")
						}
					}
				}
			}
		})
		//返回最终可用的内容
		article.ArticleData.Content, _ = doc.Find("body").Html()
	}

	err = article.Save(config.DB)

	link := GetUrl("article", article, 0)

	//添加锚文本
	if config.JsonData.PluginAnchor.ReplaceWay == 1 {
		go ReplaceContent(nil, "article", article.Id, link)
	}
	//提取锚文本
	if config.JsonData.PluginAnchor.KeywordWay == 1 {

		go AutoInsertAnchor(article.Keywords, link)
	}

	//新发布的文章，执行推送
	if newPost {
		go PushArticle(link)
		if config.JsonData.PluginSitemap.AutoBuild == 1 {
			_ = AddonSitemap("article", link)
		}
	}
	return
}

func GetArticleById(id uint) (*model.Article, error) {
	var article model.Article
	db := config.DB
	err := db.Where("`id` = ?", id).First(&article).Error
	if err != nil {
		return nil, err
	}
	//加载内容
	article.ArticleData = &model.ArticleData{}
	db.Where("`id` = ?", article.Id).First(article.ArticleData)
	//加载分类
	var category model.Category
	err = db.Where("`id` = ?", article.CategoryId).First(&category).Error
	if err == nil {
		article.Category = &category
	}

	return &article, nil
}

func GetArticleDataById(id uint) (*model.ArticleData, error) {
	var data model.ArticleData
	err := config.DB.Where("`id` = ?", id).First(&data).Error
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func CheckArticleByUrlToken(urlToken string) (*model.Article, error) {
	var article model.Article
	db := config.DB
	err := db.Where("`url_token` = ?", urlToken).First(&article).Error
	if err != nil {
		return nil, err
	}

	return &article, nil
}

func GetArticleByUrlToken(urlToken string) (*model.Article, error) {
	var article model.Article
	db := config.DB
	err := db.Where("`url_token` = ?", urlToken).First(&article).Error
	if err != nil {
		return nil, err
	}
	//加载内容
	article.ArticleData = &model.ArticleData{}
	db.Where("`id` = ?", article.Id).First(article.ArticleData)
	//加载分类
	var category model.Category
	err = db.Where("`id` = ?", article.CategoryId).First(category).Error
	if err == nil {
		article.Category = &category
	}

	return &article, nil
}

func GetArticleList(categoryId uint, order string, currentPage int, pageSize int) ([]*model.Article, int64, error) {
	var articles []*model.Article
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := config.DB.Model(&model.Article{}).Where("`status` = 1")
	if categoryId > 0 {
		builder = builder.Where("`category_id` = ?", categoryId)
	}
	if order != "" {
		builder = builder.Order(order)
	}
	if err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&articles).Error; err != nil {
		return nil, 0, err
	}

	return articles, total, nil
}

func GetRelationArticleList(categoryId uint, id uint, limit int) ([]*model.Article, error) {
	var articles []*model.Article
	var articles2 []*model.Article
	db := config.DB
	if err := db.Model(&model.Article{}).Where("`status` = 1").Where("`id` > ?", id).Where("`category_id` = ?", categoryId).Order("id ASC").Limit(limit / 2).Find(&articles).Error; err != nil {
		//no
	}
	if err := db.Model(&model.Article{}).Where("`status` = 1").Where("`id` < ?", id).Where("`category_id` = ?", categoryId).Order("id DESC").Limit(limit / 2).Find(&articles2).Error; err != nil {
		//no
	}
	//列表不返回content
	if len(articles2) > 0 {
		for _, v := range articles2 {
			articles = append(articles, v)
		}
	}

	return articles, nil
}

func GetPrevArticleById(categoryId uint, id uint) (*model.Article, error) {
	var article model.Article
	db := config.DB
	if err := db.Model(&model.Article{}).Where("`category_id` = ?", categoryId).Where("`id` < ?", id).Where("`status` = 1").Last(&article).Error; err != nil {
		return nil, err
	}

	return &article, nil
}

func GetNextArticleById(categoryId uint, id uint) (*model.Article, error) {
	var article model.Article
	db := config.DB
	if err := db.Model(&model.Article{}).Where("`category_id` = ?", categoryId).Where("`id` > ?", id).Where("`status` = 1").First(&article).Error; err != nil {
		return nil, err
	}

	return &article, nil
}
