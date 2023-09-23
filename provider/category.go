package provider

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"net/url"
	"strings"
	"time"
)

func (w *Website) GetCategories(ops func(tx *gorm.DB) *gorm.DB, parentId uint) ([]*model.Category, error) {
	var categories []*model.Category
	err := ops(w.DB).Find(&categories).Error
	if err != nil {
		return nil, err
	}
	for i := range categories {
		categories[i].GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
		categories[i].Link = w.GetUrl("category", categories[i], 0)
	}
	categoryTree := NewCategoryTree(categories)
	categories = categoryTree.GetTree(parentId, "")

	return categories, nil
}

func (w *Website) GetCategoryByTitle(title string) (*model.Category, error) {
	return w.GetCategoryByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`title` = ?", title)
	})
}

func (w *Website) GetCategoryById(id uint) (*model.Category, error) {
	return w.GetCategoryByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`id` = ?", id)
	})
}

func (w *Website) GetCategoryByUrlToken(urlToken string) (*model.Category, error) {
	if urlToken == "" {
		return nil, errors.New("empty token")
	}
	return w.GetCategoryByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`url_token` = ?", urlToken)
	})
}

func (w *Website) GetCategoryByFunc(ops func(tx *gorm.DB) *gorm.DB) (*model.Category, error) {
	var category model.Category
	err := ops(w.DB).Take(&category).Error
	if err != nil {
		return nil, err
	}
	category.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
	category.Link = w.GetUrl("category", &category, 0)

	return &category, nil
}

func (w *Website) SaveCategory(req *request.Category) (category *model.Category, err error) {
	newPost := false
	if req.Id > 0 {
		category, err = w.GetCategoryById(req.Id)
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
		category.Images[i] = strings.TrimPrefix(v, w.PluginStorage.StorageUrl)
	}
	if category.Logo != "" {
		category.Logo = strings.TrimPrefix(category.Logo, w.PluginStorage.StorageUrl)
	}
	//增加判断上级，强制类型与上级同步
	if category.ParentId > 0 {
		parent, err := w.GetCategoryById(category.ParentId)
		if err == nil {
			category.Type = parent.Type
			category.ModuleId = parent.ModuleId
		}
	}
	// 判断重复
	req.UrlToken = library.ParseUrlToken(req.UrlToken)
	if req.UrlToken == "" {
		req.UrlToken = library.GetPinyin(req.Title, w.Content.UrlTokenType == config.UrlTokenTypeSort)
	}
	category.UrlToken = w.VerifyCategoryUrlToken(req.UrlToken, category.Id)
	if category.ModuleId == 0 {
		modules := w.GetCacheModules()
		if len(modules) > 0 {
			category.ModuleId = modules[0].Id
		}
	}
	// 将单个&nbsp;替换为空格
	req.Content = library.ReplaceSingleSpace(req.Content)
	req.Content = strings.ReplaceAll(req.Content, w.System.BaseUrl, "")
	//goquery
	htmlR := strings.NewReader(req.Content)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err == nil {
		baseHost := ""
		urls, err := url.Parse(w.System.BaseUrl)
		if err == nil {
			baseHost = urls.Host
		}

		//提取描述
		if category.Description == "" {
			category.Description = library.ParseDescription(strings.ReplaceAll(CleanTagsAndSpaces(doc.Text()), "\n", " "))
		}
		//下载远程图片
		if w.Content.RemoteDownload == 1 {
			doc.Find("img").Each(func(i int, s *goquery.Selection) {
				src, exists := s.Attr("src")
				if exists {
					alt := s.AttrOr("alt", "")
					imgUrl, err := url.Parse(src)
					if err == nil {
						if imgUrl.Host != "" && imgUrl.Host != baseHost && !strings.HasPrefix(src, w.PluginStorage.StorageUrl) {
							//外链
							attachment, err := w.DownloadRemoteImage(src, alt)
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
						if w.Content.FilterOutlink == 1 {
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

	err = category.Save(w.DB)
	if err != nil {
		return
	}
	if newPost && category.Status == config.ContentStatusOK {
		link := w.GetUrl("category", category, 0)
		go w.PushArchive(link)
		if w.PluginSitemap.AutoBuild == 1 {
			_ = w.AddonSitemap("category", link, time.Unix(category.UpdatedTime, 0).Format("2006-01-02"))
		}
	}
	category.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
	w.DeleteCacheCategories()
	w.DeleteCacheIndex()

	return
}

// GetCategoryTemplate 获取分类模板，如果检测到不继承，则停止获取
func (w *Website) GetCategoryTemplate(category *model.Category) *response.CategoryTemplate {
	if category == nil {
		return nil
	}

	if category.Template != "" || category.DetailTemplate != "" {
		return &response.CategoryTemplate{
			Template:       category.Template,
			DetailTemplate: category.DetailTemplate,
		}
	}

	//查找上级
	if category.ParentId > 0 {
		parent := w.GetCategoryFromCache(category.ParentId)
		if parent != nil {
			// 如果上级存在模板，并且选择不继承，从这里阻止
			if parent.Template != "" && parent.IsInherit == 0 {
				return nil
			}
		}
		return w.GetCategoryTemplate(parent)
	}

	//不存在，则返回空
	return nil
}

func (w *Website) DeleteCacheCategories() {
	w.MemCache.Delete("categories")
}

func (w *Website) GetCacheCategories() []*model.Category {
	if w.DB == nil {
		return nil
	}
	var categories []*model.Category

	result := w.MemCache.Get("categories")
	if result != nil {
		var ok bool
		categories, ok = result.([]*model.Category)
		if ok {
			return categories
		}
	}

	w.DB.Model(model.Category{}).Order("sort asc").Find(&categories)
	for i := range categories {
		categories[i].GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
		categories[i].Link = w.GetUrl("category", categories[i], 0)
	}
	categoryTree := NewCategoryTree(categories)
	categories = categoryTree.GetTree(0, "")

	w.MemCache.Set("categories", categories, 0)

	return categories
}

// GetSubCategoryIds 获取分类的子分类
func (w *Website) GetSubCategoryIds(categoryId uint, categories []*model.Category) []uint {
	var subIds []uint
	if categories == nil {
		categories = w.GetCacheCategories()
	}

	for i := range categories {
		if categories[i].ParentId == categoryId {
			subIds = append(subIds, categories[i].Id)
			subIds = append(subIds, w.GetSubCategoryIds(categories[i].Id, categories)...)
		}
	}

	return subIds
}

func (w *Website) GetCategoryFromCache(categoryId uint) *model.Category {
	if categoryId == 0 {
		return nil
	}
	categories := w.GetCacheCategories()
	for i := range categories {
		if categories[i].Id == categoryId {
			return categories[i]
		}
	}

	return nil
}

func (w *Website) GetCategoryFromCacheByToken(urlToken string) *model.Category {
	categories := w.GetCacheCategories()
	for i := range categories {
		if categories[i].UrlToken == urlToken {
			return categories[i]
		}
	}

	return nil
}

func (w *Website) GetCategoriesFromCache(moduleId, parentId uint, pageType int) []*model.Category {
	var tmpCategories []*model.Category
	categories := w.GetCacheCategories()
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
			tmpCategories = append(tmpCategories, categories[i])
		}
	}

	return tmpCategories
}

func (w *Website) VerifyCategoryUrlToken(urlToken string, id uint) string {
	index := 0
	// 防止超出长度
	if len(urlToken) > 150 {
		urlToken = urlToken[:150]
	}
	for {
		tmpToken := urlToken
		if index > 0 {
			tmpToken = fmt.Sprintf("%s-%d", urlToken, index)
		}
		// 判断分类
		tmpCat, err := w.GetCategoryByUrlToken(tmpToken)
		if err == nil && tmpCat.Id != id {
			index++
			continue
		}
		// 判断archive
		_, err = w.GetArchiveByUrlToken(tmpToken)
		if err == nil {
			index++
			continue
		}
		urlToken = tmpToken
		break
	}

	return urlToken
}
