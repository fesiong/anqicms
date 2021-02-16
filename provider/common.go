package provider

import (
    "irisweb/config"
    "irisweb/model"
)

type statistics struct {
    ArticleCount  int64 `json:"article_count"`
    CategoryCount int64 `json:"category_count"`
    LinkCount     int64 `json:"link_count"`
    CommentCount  int64 `json:"comment_count"`
    ProductCount  int64 `json:"product_count"`
}

func Statistics() *statistics {
    var articleCount int64
    var categoryCount int64
    var linkCount int64
    var commentCount int64
    var productCount int64
    config.DB.Model(&model.Article{}).Where("status != 99").Count(&articleCount)
    config.DB.Model(&model.Category{}).Where("status != 99").Count(&categoryCount)
    config.DB.Model(&model.Link{}).Where("status != 99").Count(&linkCount)
    config.DB.Model(&model.Comment{}).Where("status != 99").Count(&commentCount)
    config.DB.Model(&model.Product{}).Where("status != 99").Count(&productCount)

    return &statistics{
        ArticleCount:  articleCount,
        CategoryCount: categoryCount,
        LinkCount:     linkCount,
        CommentCount:  categoryCount,
        ProductCount:  productCount,
    }
}
