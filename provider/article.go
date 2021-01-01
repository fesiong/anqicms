package provider

import (
	"irisweb/config"
	"irisweb/model"
)

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
	article.Category = &model.Category{}
	db.Where("`id` = ?", article.CategoryId).First(article.Category)

	return &article, nil
}

func GetArticleList(categoryId uint, order string, currentPage int, pageSize int) ([]*model.Article, int64, error) {
	var articles []*model.Article
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := config.DB.Model(model.Article{})
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

func GetRelationArticleList(categoryId uint, id uint, limit int) ([]model.Article, error) {
	var articles []model.Article
	var articles2 []model.Article
	db := config.DB
	if err := db.Model(model.Article{}).Where("`status` = 1").Where("`id` > ?", id).Where("`category_id` = ?", categoryId).Order("id ASC").Limit(limit/2).Find(&articles).Error; err != nil {
		//no
	}
	if err := db.Model(model.Article{}).Where("`status` = 1").Where("`id` < ?", id).Where("`category_id` = ?", categoryId).Order("id DESC").Limit(limit/2).Find(&articles2).Error; err != nil {
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
	if err := db.Model(model.Article{}).Where("`category_id` = ?", categoryId).Where("`id` < ?", id).Where("`status` = 1").Last(&article).Error; err != nil {
		return nil, err
	}

	return &article, nil
}

func GetNextArticleById(categoryId uint, id uint) (*model.Article, error) {
	var article model.Article
	db := config.DB
	if err := db.Model(model.Article{}).Where("`category_id` = ?", categoryId).Where("`id` > ?", id).Where("`status` = 1").First(&article).Error; err != nil {
		return nil, err
	}

	return &article, nil
}