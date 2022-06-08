package provider

import (
	"fmt"
	"io/ioutil"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"mime/multipart"
	"strconv"
	"strings"
)

type KeywordCSV struct {
	Title      string `csv:"title"`
	CategoryId uint   `csv:"category_id"`
}

func GetKeywordList(keyword string, currentPage, pageSize int) ([]*model.Keyword, int64, error) {
	var keywords []*model.Keyword
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := dao.DB.Model(&model.Keyword{}).Order("id desc")
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
	err := dao.DB.Model(&model.Keyword{}).Order("id desc").Find(&keywords).Error
	if err != nil {
		return nil, err
	}

	return keywords, nil
}

func GetKeywordById(id uint) (*model.Keyword, error) {
	var keyword model.Keyword

	err := dao.DB.Where("`id` = ?", id).First(&keyword).Error
	if err != nil {
		return nil, err
	}

	return &keyword, nil
}

func GetKeywordByTitle(title string) (*model.Keyword, error) {
	var keyword model.Keyword

	err := dao.DB.Where("`title` = ?", title).First(&keyword).Error
	if err != nil {
		return nil, err
	}

	return &keyword, nil
}

func ImportKeywords(file multipart.File, info *multipart.FileHeader) (string, error) {
	buff, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(buff), "\n")
	var total int
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// 格式：title, category_id
		if i == 0 {
			continue
		}
		values := strings.Split(line, ",")
		if len(values) < 2 {
			continue
		}
		title := strings.TrimSpace(values[0])
		if title == "" {
			continue
		}
		keyword, err := GetKeywordByTitle(title)
		if err != nil {
			//表示不存在
			keyword = &model.Keyword{
				Title: title,
				Status: 1,
			}
			total++
		}
		categoryId, _ := strconv.Atoi(values[1])
		keyword.CategoryId = uint(categoryId)

		keyword.Save(dao.DB)
	}

	return fmt.Sprintf(config.Lang("成功导入了%d个关键词"), total), nil
}

func DeleteKeyword(keyword *model.Keyword) error {
	err := dao.DB.Delete(keyword).Error
	if err != nil {
		return err
	}

	return nil
}
