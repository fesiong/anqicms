package provider

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm"
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"irisweb/request"
	"net/url"
	"strconv"
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

	//extra
	extraFields := map[string]interface{}{}
	if len(config.JsonData.ArticleExtraFields) > 0 {
		for _, v := range config.JsonData.ArticleExtraFields {
			//先检查是否有必填而没有填写的
			if v.Required && req.Extra[v.FieldName] == nil {
				return nil, fmt.Errorf("%s必填", v.Name)
			}
			if req.Extra[v.FieldName] != nil {
				if v.Type == config.CustomFieldTypeCheckbox {
					//只有这个类型的数据是数组,数组转成,分隔字符串
					if val, ok := req.Extra[v.FieldName].([]interface{}); ok {
						var val2 []string
						for _, v2 := range val {
							val2 = append(val2, v2.(string))
						}
						extraFields[v.FieldName] = strings.Join(val2, ",")
					}

				} else if v.Type == config.CustomFieldTypeNumber {
					//只有这个类型的数据是数字，转成数字
					extraFields[v.FieldName], _ = strconv.Atoi(req.Extra[v.FieldName].(string))
				} else {
					extraFields[v.FieldName] = req.Extra[v.FieldName]
				}
			} else {
				if v.Type == config.CustomFieldTypeNumber {
					//只有这个类型的数据是数字，转成数字
					extraFields[v.FieldName] = 0
				} else {
					extraFields[v.FieldName] = ""
				}
			}
		}
	}

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
		//检查有多少个material
		var materialIds []uint
		doc.Find("div[data-material]").Each(func(i int, s *goquery.Selection) {
			tmpId, exists := s.Attr("data-material")
			if exists {
				//记录material
				materialId, err := strconv.Atoi(tmpId)
				if err == nil {
					materialIds = append(materialIds, uint(materialId))
				}
			}
		})
		go LogMaterialData(materialIds, "article", article.Id)

		//返回最终可用的内容
		article.ArticleData.Content, _ = doc.Find("body").Html()
	}

	err = article.Save(config.DB)

	//extra
	if len(extraFields) > 0 {
		//入库
		config.DB.Model(article).Updates(extraFields)
	}

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
	article.Extra = GetArticleExtra(article.Id)

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
	err = db.Where("`id` = ?", article.CategoryId).First(&category).Error
	if err == nil {
		article.Category = &category
	}
	article.Extra = GetArticleExtra(article.Id)

	return &article, nil
}

func GetArticleList(categoryId uint, q string, order string, currentPage int, pageSize int) ([]*model.Article, int64, error) {
	var articles []*model.Article
	offset := (currentPage - 1) * pageSize
	var total int64

	extraFields := map[uint]map[string]*model.CustomField{}
	var results []map[string]interface{}
	var fields []string
	fields = append(fields, "id")
	if len(config.JsonData.ArticleExtraFields) > 0 {
		for _, v := range config.JsonData.ArticleExtraFields {
			fields = append(fields, v.FieldName)
		}
	}

	builder := config.DB.Model(&model.Article{}).Where("`status` = 1")
	if categoryId > 0 {
		builder = builder.Where("`category_id` = ?", categoryId)
	}
	if q != "" {
		builder = builder.Where("`title` like ?", "%"+q+"%")
	}
	if order != "" {
		builder = builder.Order(order)
	}
	builder = builder.Count(&total).Limit(pageSize).Offset(offset)
	if err := builder.Find(&articles).Error; err != nil {
		return nil, 0, err
	}
	if len(fields) > 0 {
		builder.Select(strings.Join(fields, ",")).Scan(&results)
		for _, field := range results {
			item := map[string]*model.CustomField{}
			for _, v := range config.JsonData.ArticleExtraFields {
				item[v.FieldName] = &model.CustomField{
					Name:      v.Name,
					Value:     field[v.FieldName],
				}
			}
			if id, ok := field["id"].(uint32); ok {
				extraFields[uint(id)] = item
			}
		}
		for i := range articles {
			if extraFields[articles[i].Id] != nil {
				articles[i].Extra = extraFields[articles[i].Id]
			}
		}
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

func SaveArticleExtraFields(reqFields []*config.CustomField) error {
	var diffFields []*config.CustomField
	for _, v := range config.JsonData.ArticleExtraFields {
		exists := false
		for _, f := range reqFields {
			if v == f {
				exists = true
			}
		}
		if !exists {
			diffFields = append(diffFields, v)
		}
	}

	//对于需要去除的fields，进行删除操作
	if len(diffFields) > 0 {
		for _, v := range diffFields {
			if config.DB.Migrator().HasColumn(&model.Article{}, v.FieldName) {
				config.DB.Migrator().DropColumn(&model.Article{}, v.FieldName)
			}
		}
	}
	//然后再追加
	stmt := &gorm.Statement{DB: config.DB}
	stmt.Parse(&model.Article{})
	for _, v := range reqFields {
		column := v.GetFieldColumn()
		if !config.DB.Migrator().HasColumn(&model.Article{}, v.FieldName) {
			//创建语句
			config.DB.Exec("ALTER TABLE ? ADD COLUMN ?", gorm.Expr(stmt.Table), gorm.Expr(column))
		} else {
			//更新语句
			config.DB.Exec("ALTER TABLE ? MODIFY COLUMN ?", gorm.Expr(stmt.Table), gorm.Expr(column))
		}
	}

	//记录到内容
	config.JsonData.ArticleExtraFields = reqFields

	err := config.WriteConfig()

	return err
}

func GetArticleExtra(id uint) map[string]*model.CustomField {
	//读取extra
	result := map[string]interface{}{}
	extraFields := map[string]*model.CustomField{}
	if len(config.JsonData.ArticleExtraFields) > 0 {
		var fields []string
		for _, v := range config.JsonData.ArticleExtraFields {
			fields = append(fields, v.FieldName)
		}
		//从数据库中取出来
		config.DB.Model(&model.Article{}).Where("`id` = ?", id).Select(strings.Join(fields, ",")).Scan(&result)
		//extra的CheckBox的值
		for _, v := range config.JsonData.ArticleExtraFields {
			extraFields[v.FieldName] = &model.CustomField{
				Name:      v.Name,
				Value:     result[v.FieldName],
			}
		}
	}

	return extraFields
}
