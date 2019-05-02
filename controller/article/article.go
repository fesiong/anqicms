package article

import (
	"goblog/config"
	"goblog/model"
	"goblog/utils"
	"fmt"

	//	"html"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"goblog/controller/common"

	"github.com/gin-gonic/gin"
)

func List(c *gin.Context) {
	fmt.Println("article list")
	SendErrJSON := common.SendErrJSON

	var articles []model.Article
	var categoryID int
	//当前文章id，用来获取相关文章
	var articleID int
	var pageSize int
	var page int
	var err error

	categoryID, err = strconv.Atoi(c.Query("categoryID"))
	if err != nil {
		err = nil
	}

	articleID, err = strconv.Atoi(c.Query("articleID"))
	if err != nil {
		err = nil
	}

	if page, err = strconv.Atoi(c.Query("page")); err != nil {
		page = 1
		err = nil
	}

	if pageSize, err = strconv.Atoi(c.Query("pageSize")); err != nil {
		pageSize = 20
		err = nil
	}

	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	type TotalCountResult struct {
		TotalCount int
	}

	var totalCountResult TotalCountResult

	if categoryID > 0 {
		var category model.Category
		if err := model.DB.First(&category).Error; err != nil {
			SendErrJSON("分类不正确", c)
			return
		}
		var sql = "SELECT distinct(a.id),a.title,a.views,a.description,a.logo,a.add_time,a.comment_count FROM " + config.DBConfig.TablePrefix + "article AS a LEFT JOIN " + config.DBConfig.TablePrefix + "relation AS r ON a.id = r.article_id WHERE a.status = 1 AND r.category_id = {categoryID} ORDER BY a.ID DESC LIMIT {offset}, {pageSize}"
		sql = strings.Replace(sql, "{categoryID}", strconv.Itoa(categoryID), -1)
		sql = strings.Replace(sql, "{offset}", strconv.Itoa(offset), -1)
		sql = strings.Replace(sql, "{pageSize}", strconv.Itoa(pageSize), -1)
		if err := model.DB.Raw(sql).Scan(&articles).Error; err != nil {
			SendErrJSON("error", c)
			return
		}

		for i := 0; i < len(articles); i++ {
			articles[i].Categories = []model.Category{category}
		}

		countSQL := "SELECT COUNT(distinct(a.id)) AS total_count FROM " + config.DBConfig.TablePrefix + "article AS a LEFT JOIN " + config.DBConfig.TablePrefix + "relation AS r ON a.id = r.article_id WHERE a.status = 1 AND r.category_id = {categoryID}"
		if err := model.DB.Raw(countSQL).Scan(&totalCountResult).Error; err != nil {
			SendErrJSON("error", c)
			return
		}
	} else if articleID > 0 {
		//暂时获取比这篇文章更久的文章
		if err := model.DB.Where("status = 1 and id < ?", articleID).Find(&articles).Offset(offset).Limit(pageSize).Order("id desc").Error; err != nil {
			SendErrJSON("error", c)
			return
		}

		if err := model.DB.Model(&model.Article{}).Where("status = 1 and id < ?", articleID).
			Count(&totalCountResult.TotalCount).Error; err != nil {
			SendErrJSON("error", c)
			return
		}
	} else {
		if err := model.DB.Where("status = 1").Find(&articles).Offset(offset).Limit(pageSize).Error; err != nil {
			SendErrJSON("error", c)
			return
		}

		if err := model.DB.Model(&model.Article{}).Where("status = 1").
			Count(&totalCountResult.TotalCount).Error; err != nil {
			SendErrJSON("error", c)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": gin.H{
			"articles":   articles,
			"page":       page,
			"pageSize":   pageSize,
			"totalPage":  math.Ceil(float64(totalCountResult.TotalCount) / float64(pageSize)),
			"totalCount": totalCountResult.TotalCount,
		},
	})
}

func Detail(c *gin.Context) {
	SendErrJSON := common.SendErrJSON
	fmt.Println("article detail")
	articleID, err := strconv.Atoi(c.Param("id"))

	if err != nil {
		SendErrJSON("错误的文章id", c)
		return
	}

	var article model.Article

	if err := model.DB.First(&article, articleID).Error; err != nil {
		fmt.Printf(err.Error())
		SendErrJSON("错误的文章ID", c)
		return
	}

	if article.Status != 1 {
		SendErrJSON("错误的文章id", c)
		return
	}

	article.Views++
	if err := model.DB.Save(&article).Error; err != nil {
		SendErrJSON("error", c)
		return
	}

	//获取分类
	if err := model.DB.Model(&article).Related(&article.Categories, "categories").Error; err != nil {
		fmt.Println(err.Error())
		SendErrJSON("error", c)
		return
	}

	//获取评论

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": article,
	})
}

func Save(c *gin.Context) {
	SendErrJSON := common.SendErrJSON

	var article model.Article

	if err := c.ShouldBindJSON(&article); err != nil {
		fmt.Println(err.Error())
		SendErrJSON("参数无效", c)
		return
	}

	if article.Title == "" {
		SendErrJSON("文章名称不能为空", c)
		return
	}

	if article.Categories == nil || len(article.Categories) <= 0 {
		SendErrJSON("请选择文章分类", c)
		return
	}

	var queryArticle model.Article
	if article.ID > 0 {
		if err := model.DB.First(&queryArticle, article.ID).Error; err != nil {
			SendErrJSON("无效的文章ID", c)
			return
		}
		tempArticle := article
		article = queryArticle
		article.Title = tempArticle.Title
		article.SeoTitle = tempArticle.SeoTitle
		article.Keywords = tempArticle.Keywords
		article.Description = tempArticle.Description
		article.Message = tempArticle.Message
		article.Categories = tempArticle.Categories
		if tempArticle.Logo != "" {
			article.Logo = tempArticle.Logo
		}

		var sql = "DELETE FROM " + config.DBConfig.TablePrefix + "relation WHERE article_id = ?"
		if err := model.DB.Exec(sql, article.ID).Error; err != nil {
			fmt.Println(err.Error())
			SendErrJSON("error", c)
			return
		}

	} else {
		article.AddTime = time.Now().Unix()
		article.Status = 1
	}

	article.Title = strings.TrimSpace(utils.AvoidXSS(article.Title))
	article.SeoTitle = strings.TrimSpace(utils.AvoidXSS(article.SeoTitle))
	article.Keywords = strings.TrimSpace(utils.AvoidXSS(article.Keywords))
	article.Description = strings.TrimSpace(utils.AvoidXSS(article.Description))
	//article.Message = strings.TrimSpace(utils.AvoidXSS(article.Message))
	//存数据库的时候不转
	//article.Message = html.UnescapeString(article.Message)

	for i := 0; i < len(article.Categories); i++ {
		var category model.Category
		category.Title = article.Categories[i].Title
		if category.Title == "" {
			SendErrJSON("分类无效", c)
			return
		}
		if err := model.DB.Where("title = ?", category.Title).First(&category).Error; err != nil {
			fmt.Println(err)
			if err := model.DB.Save(&category).Error; err != nil {
				SendErrJSON("分类保存失败", c)
				return
			}
		}
		article.Categories[i] = category
	}

	if err := model.DB.Save(&article).Error; err != nil {
		SendErrJSON("文章保存失败", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": article,
	})
}

/**
 * 删除文章，并同时删除对应的评论
 */
func Delete(c *gin.Context) {
	SendErrJSON := common.SendErrJSON
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		SendErrJSON("无效的id", c)
		return
	}

	var article model.Article
	if err := model.DB.First(&article, id).Error; err != nil {
		SendErrJSON("无效的id", c)
		return
	}

	if err := model.DB.Delete(&article).Error; err != nil {
		SendErrJSON("删除过程中出错", c)
		return
	}

	var sql = "DELETE FROM " + config.DBConfig.TablePrefix + "relation WHERE article_id = ?"
	if err := model.DB.Exec(sql, article.ID).Error; err != nil {
		fmt.Println(err.Error())
		SendErrJSON("error", c)
		return
	}

	sql = "DELETE FROM " + config.DBConfig.TablePrefix + "comments WHERE article_id = ?"
	if err := model.DB.Exec(sql, article.ID).Error; err != nil {
		fmt.Println(err.Error())
		SendErrJSON("error", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": gin.H{
			"id": article.ID,
		},
	})
}
