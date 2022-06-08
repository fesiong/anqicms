package provider

import (
	"errors"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"strings"
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
	if req.Id > 0 {
		category, err = GetCategoryById(req.Id)
		if err != nil {
			return nil, err
		}
	} else {
		category = &model.Category{
			Status: 1,
		}
	}
	category.Title = req.Title
	category.SeoTitle = req.SeoTitle
	category.Keywords = req.Keywords
	category.Description = req.Description
	category.Content = req.Content
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

	err = category.Save(dao.DB)
	if err != nil {
		return
	}

	DeleteCacheCategories()

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

	dao.DB.Where(model.Category{}).Find(&categories)

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
