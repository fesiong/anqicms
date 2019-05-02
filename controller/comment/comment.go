package comment

import (
	"goblog/model"
	"goblog/utils"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"goblog/controller/common"

	"github.com/gin-gonic/gin"
)

func List(c *gin.Context) {
	fmt.Println("comment list")
	SendErrJSON := common.SendErrJSON

	articleID, err := strconv.Atoi(c.Param("articleID"))

	if err != nil {
		SendErrJSON("无效的文章id", c)
		return
	}

	var page int

	if page, err = strconv.Atoi(c.Query("page")); err != nil {
		page = 1
		err = nil
	}

	if page < 1 {
		page = 1
	}

	pageSize := 20
	offset := (page - 1) * pageSize

	type TotalCountResult struct {
		TotalCount int
	}

	var totalCountResult TotalCountResult

	var article model.Article

	if err := model.DB.First(&article, articleID).Error; err != nil {
		SendErrJSON("无效的文章id", c)
		return
	}

	var comments []model.Comment

	if err := model.DB.Where("article_id = ?", articleID).Preload("User").Find(&comments).Offset(offset).Limit(pageSize).Error; err != nil {
		SendErrJSON("error", c)
		return
	}

	if err := model.DB.Model(&model.Comment{}).Where("article_id = ?", articleID).
		Count(&totalCountResult.TotalCount).Error; err != nil {
		SendErrJSON("error", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": gin.H{
			"comments":   comments,
			"page":       page,
			"pageSize":   pageSize,
			"totalPage":  math.Ceil(float64(totalCountResult.TotalCount) / float64(pageSize)),
			"totalCount": totalCountResult.TotalCount,
		},
	})
}

func Save(c *gin.Context) {
	SendErrJSON := common.SendErrJSON

	var comment model.Comment

	if err := c.ShouldBindJSON(&comment); err != nil {
		fmt.Println(err.Error())
		SendErrJSON("参数无效", c)
		return
	}

	if comment.ArticleID == 0 {
		SendErrJSON("文章id不能为空", c)
		return
	}

	if comment.Message == "" {
		SendErrJSON("评论内容不能为空", c)
		return
	}

	var article model.Article
	if err := model.DB.First(&article, comment.ArticleID).Error; err != nil {
		SendErrJSON("无效的文章id", c)
		return
	}

	var queryComment model.Comment
	if comment.ID > 0 {
		if err := model.DB.First(&queryComment, comment.ID).Error; err != nil {
			SendErrJSON("无效的评论ID", c)
			return
		}
		tempComment := comment
		comment = queryComment
		comment.Message = tempComment.Message
	} else {
		comment.AddTime = time.Now().Unix()
		comment.UserID = 0
	}

	comment.Message = strings.TrimSpace(utils.AvoidXSS(comment.Message))

	if err := model.DB.Save(&comment).Error; err != nil {
		SendErrJSON("评论保存失败", c)
		return
	}

	if err := model.DB.Model(&article).Update("comment_count", article.CommentCount+1).Error; err != nil {
		SendErrJSON("评论保存失败", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": comment,
	})
}

func Delete(c *gin.Context) {
	SendErrJSON := common.SendErrJSON
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		SendErrJSON("无效的id", c)
		return
	}

	var comment model.Comment
	if err := model.DB.First(&comment, id).Error; err != nil {
		SendErrJSON("无效的id", c)
		return
	}

	if err := model.DB.Delete(&comment).Error; err != nil {
		SendErrJSON("删除过程中出错", c)
		return
	}

	var article model.Article
	if err := model.DB.First(&article, comment.ArticleID).Error; err != nil {
		SendErrJSON("删除过程中出错", c)
		return
	}

	if err := model.DB.Model(&article).Update("comment_count", article.CommentCount-1).Error; err != nil {
		SendErrJSON("删除过程中出错", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": gin.H{
			"id": comment.ID,
		},
	})
}
