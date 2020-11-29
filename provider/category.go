package provider

import (
	"irisweb/config"
	"irisweb/model"
)

func GetCategories() ([]*model.Category, error) {
	var categories []*model.Category
	db := config.DB
	err := db.Where("`status` = ?", 1).Find(&categories).Error
	if err != nil {
		return nil, err
	}

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