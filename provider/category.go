package provider

import (
	"errors"
	"github.com/PuerkitoBio/goquery"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"net/url"
	"strings"
	"time"
)

func GetCategories(moduleId uint, title string, parentId uint) ([]*model.Category, error) {
	var categories []*model.Category
	db := dao.DB
	builder := db.Where("`type` = ? and `status` = ?", config.CategoryTypeArchive, 1)
	if moduleId > 0 {
		builder = builder.Where("`module_id` = ?", moduleId)
	}

	err := builder.Order("module_id asc,sort asc").Find(&categories).Error
	if err != nil {
		return nil, err
	}

	categoryTree := NewCategoryTree(categories)
	categories = categoryTree.GetTree(parentId, "")

	return categories, nil
}

func GetPages(title string) ([]*model.Category, error) {
	var categories []*model.Category
	db := dao.DB
	builder := db.Where("`type` = ? and `status` = ?", config.CategoryTypePage, 1)
	if title != "" {
		builder = builder.Where("`title` like ?", "%"+title+"%")
	}

	err := builder.Order("sort asc").Find(&categories).Error
	if err != nil {
		return nil, err
	}

	return categories, nil
}

func GetCategoryByTitle(title string) (*model.Category, error) {
	var category model.Category
	db := dao.DB
	err := db.Where("`title` = ?", title).First(&category).Error
	if err != nil {
		return nil, err
	}

	return &category, nil
}

func GetCategoryById(id uint) (*model.Category, error) {
	var category model.Category
	db := dao.DB
	err := db.Where("`id` = ?", id).First(&category).Error
	if err != nil {
		return nil, err
	}

	return &category, nil
}

func GetCategoryByUrlToken(urlToken string) (*model.Category, error) {
	var category model.Category
	db := dao.DB
	err := db.Where("`url_token` = ?", urlToken).First(&category).Error
	if err != nil {
		return nil, err
	}

	return &category, nil
}

func SaveCategory(req *request.Category) (category *model.Category, err error) {
	newPost := false
	if req.Id > 0 {
		category, err = GetCategoryById(req.Id)
		if err != nil {
			return nil, err
		}
	} else {
		category = &model.Category{
			Status: 1,
		}
		newPost = true
	}
	category.Title = req.Title
	category.SeoTitle = req.SeoTitle
	category.Keywords = req.Keywords
	category.Description = req.Description
	category.Type = req.Type
	category.ModuleId = req.ModuleId
	category.ParentId = req.ParentId
	category.Sort = req.Sort
	category.Status = 1
	category.Template = req.Template
	category.DetailTemplate = req.DetailTemplate
	category.IsInherit = req.IsInherit
	category.Images = req.Images
	category.Logo = req.Logo
	for i, v := range category.Images {
		category.Images[i] = strings.TrimPrefix(v, config.JsonData.System.BaseUrl)
	}
	if category.Logo != "" {
		category.Logo = strings.TrimPrefix(category.Logo, config.JsonData.System.BaseUrl)
	}
	// 判断重复
	if req.UrlToken != "" {
		req.UrlToken = library.ParseUrlToken(req.UrlToken)
		exists, err := GetCategoryByUrlToken(req.UrlToken)
		if err == nil {
			if category.Id == 0 || (category.Id > 0 && exists.Id != category.Id) {
				return nil, errors.New(config.Lang("自定义URL重复"))
			}
		}
		category.UrlToken = req.UrlToken
	}
	//增加判断上级，强制类型与上级同步
	if category.ParentId > 0 {
		parent, err := GetCategoryById(category.ParentId)
		if err == nil {
			category.Type = parent.Type
			category.ModuleId = parent.ModuleId
		}
	}
	if category.UrlToken == "" {
		newToken := library.GetPinyin(req.Title)
		_, err := GetCategoryByUrlToken(newToken)
		if err == nil {
			//增加随机
			newToken += library.GenerateRandString(3)
		}
		category.UrlToken = newToken
	}
	// 将单个&nbsp;替换为空格
	req.Content = library.ReplaceSingleSpace(req.Content)
	req.Content = strings.ReplaceAll(req.Content, config.JsonData.System.BaseUrl, "")
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
		if category.Description == "" {
			textRune := []rune(strings.ReplaceAll(CleanTagsAndSpaces(doc.Text()), "\n", " "))
			if len(textRune) > 150 {
				category.Description = string(textRune[:150])
			} else {
				category.Description = string(textRune)
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
		if len(category.Logo) == 0 {
			imgSections := doc.Find("img")
			if imgSections.Length() > 0 {
				//获取第一条
				category.Logo = imgSections.Eq(0).AttrOr("src", "")
			}
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
		req.Content, _ = doc.Find("body").Html()
	}
	category.Content = req.Content

	err = category.Save(dao.DB)
	if err != nil {
		return
	}
	if newPost && category.Status == config.ContentStatusOK {
		link := GetUrl("category", category, 0)
		go PushArchive(link)
		if config.JsonData.PluginSitemap.AutoBuild == 1 {
			_ = AddonSitemap("category", link, time.Unix(category.UpdatedTime, 0).Format("2006-01-02"))
		}
	}

	DeleteCacheCategories()
	DeleteCacheIndex()

	return
}

// GetCategoryTemplate 获取分类模板，如果检测到不继承，则停止获取
func GetCategoryTemplate(category *model.Category) *response.CategoryTemplate {
	if category == nil {
		return nil
	}

	if category.Template != "" {
		return &response.CategoryTemplate{
			Template:       category.Template,
			DetailTemplate: category.DetailTemplate,
		}
	}

	//查找上级
	if category.ParentId > 0 {
		parent := GetCategoryFromCache(category.ParentId)
		if parent != nil {
			// 如果上级存在模板，并且选择不继承，从这里阻止
			if parent.Template != "" && parent.IsInherit == 0 {
				return nil
			}
		}
		return GetCategoryTemplate(parent)
	}

	//不存在，则返回空
	return nil
}

func DeleteCacheCategories() {
	library.MemCache.Delete("categories")
}

func GetCacheCategories() []model.Category {
	if dao.DB == nil {
		return nil
	}
	var categories []model.Category

	result := library.MemCache.Get("categories")
	if result != nil {
		var ok bool
		categories, ok = result.([]model.Category)
		if ok {
			return categories
		}
	}

	dao.DB.Where(model.Category{}).Order("sort asc").Find(&categories)

	library.MemCache.Set("categories", categories, 0)

	return categories
}

// GetSubCategoryIds 获取分类的子分类
func GetSubCategoryIds(categoryId uint, categories []model.Category) []uint {
	var subIds []uint
	if categories == nil {
		categories = GetCacheCategories()
	}

	for i := range categories {
		if categories[i].ParentId == categoryId {
			subIds = append(subIds, categories[i].Id)
			subIds = append(subIds, GetSubCategoryIds(categories[i].Id, categories)...)
		}
	}

	return subIds
}

func GetCategoryFromCache(categoryId uint) *model.Category {
	categories := GetCacheCategories()
	for i := range categories {
		if categories[i].Id == categoryId {
			return &categories[i]
		}
	}

	return nil
}

func GetCategoryFromCacheByToken(urlToken string) *model.Category {
	categories := GetCacheCategories()
	for i := range categories {
		if categories[i].UrlToken == urlToken {
			return &categories[i]
		}
	}

	return nil
}

func GetCategoriesFromCache(moduleId, parentId uint, pageType int) []*model.Category {
	var tmpCategories []*model.Category
	categories := GetCacheCategories()
	for i := range categories {
		if pageType == config.CategoryTypePage {
			if categories[i].Type != config.CategoryTypePage {
				continue
			}
		} else if parentId == 0 {
			if categories[i].ModuleId != moduleId {
				continue
			}
		}
		if categories[i].ParentId == parentId {
			tmpCategories = append(tmpCategories, &categories[i])
		}
	}

	return tmpCategories
}
