package provider

import (
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"irisweb/request"
)

func GetCategories(categoryType uint) ([]*model.Category, error) {
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
	categories = categoryTree.GetTree(0, "")

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
			Status:      1,
		}
	}
	category.Title = req.Title
	category.Description = req.Description
	category.Content = req.Content
	category.Type = req.Type
	category.ParentId = req.ParentId
	category.Sort = req.Sort
	category.Status = 1
	//增加判断上级，强制类型与上级同步
	if category.ParentId > 0 {
		parent, err := GetCategoryById(category.ParentId)
		if err == nil {
			category.Type = parent.Type
		}
	}

	err = category.Save(config.DB)
	if err != nil {
		return
	}
	return
}