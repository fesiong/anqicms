package provider

import (
	"fmt"
	"github.com/gocarina/gocsv"
	"irisweb/config"
	"irisweb/model"
	"mime/multipart"
)

type KeywordCSV struct {
	Title string `csv:"title"`
}

func GetKeywordList(keyword string, currentPage, pageSize int) ([]*model.Keyword, int64, error) {
	var keywords []*model.Keyword
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := config.DB.Model(&model.Keyword{}).Order("id desc")
	if keyword != "" {
		//模糊搜索
		builder = builder.Where("(`title` like ?)", "%"+keyword+"%")
	}

	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&keywords).Error
	if err != nil {
		return nil, 0, err
	}

	return keywords, total, nil
}

func GetAllKeywords() ([]*model.Keyword, error) {
	var keywords []*model.Keyword
	err := config.DB.Model(&model.Keyword{}).Order("id desc").Find(&keywords).Error
	if err != nil {
		return nil, err
	}

	return keywords, nil
}

func GetKeywordById(id uint) (*model.Keyword, error) {
	var keyword model.Keyword

	err := config.DB.Where("`id` = ?", id).First(&keyword).Error
	if err != nil {
		return nil, err
	}

	return &keyword, nil
}

func GetKeywordByTitle(title string) (*model.Keyword, error) {
	var keyword model.Keyword

	err := config.DB.Where("`title` = ?", title).First(&keyword).Error
	if err != nil {
		return nil, err
	}

	return &keyword, nil
}

func ImportKeywords(file multipart.File, info *multipart.FileHeader) (string, error) {
	var keywords []*KeywordCSV

	if err := gocsv.Unmarshal(file, &keywords); err != nil {
		return "", err
	}

	total := 0
	for _, item := range keywords {
		keyword, err := GetKeywordByTitle(item.Title)
		if err != nil {
			//表示不存在
			keyword = &model.Keyword{
				Title:  item.Title,
				Status: 1,
			}
			total++
		}

		keyword.Save(config.DB)
	}

	return fmt.Sprintf("成功导入了%d个关键词", total), nil
}

func DeleteKeyword(keyword *model.Keyword) error {
	err := config.DB.Delete(keyword).Error
	if err != nil {
		return err
	}

	return nil
}
