package provider

import (
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"irisweb/request"
	"irisweb/response"
)

func GetCategories(categoryType uint, parentId uint) ([]*model.Category, error) {
	var categories []*model.Category
	db := config.DB
	builder := db.Where("`status` = ?", 1)
	if categoryType > 0 {
		builder = builder.Where("`type` = ?", categoryType)
	}
	err := builder.Order("sort asc").Find(&categories).Error
	if err != nil {
		return nil, err
	}

	categoryTree := library.NewCategoryTree(categories)
	categories = categoryTree.GetTree(parentId, "")

	return categories, nil
}

func GetCategoryByTitle(title string) (*model.Category, error) {
	var category model.Category
	db := config.DB
	err := db.Where("`title` = ?", title).First(&category).Error
	if err != nil {
		return nil, err
	}

	return &category, nil
}

func GetCategoryById(id uint) (*model.Category, error) {
	var category model.Category
	db := config.DB
	err := db.Where("`id` = ?", id).First(&category).Error
	if err != nil {
		return nil, err
	}

	return &category, nil
}

func GetCategoryByUrlToken(urlToken string) (*model.Category, error) {
	var category model.Category
	db := config.DB
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
	category.Description = req.Description
	category.Content = req.Content
	category.Type = req.Type
	category.ParentId = req.ParentId
	category.Sort = req.Sort
	category.Status = 1
	category.Template = req.Template
	category.DetailTemplate = req.DetailTemplate
	//增加判断上级，强制类型与上级同步
	if category.ParentId > 0 {
		parent, err := GetCategoryById(category.ParentId)
		if err == nil {
			category.Type = parent.Type
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

	err = category.Save(config.DB)
	if err != nil {
		return
	}
	return
}

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
		parent, _ := GetCategoryById(category.ParentId)
		return GetCategoryTemplate(parent)
	}

	//不存在，则返回空
	return nil
}