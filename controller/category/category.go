package category

import (
	"goblog/config"
	"goblog/model"
	"goblog/utils"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"goblog/controller/common"

	"github.com/gin-gonic/gin"
)

func List(c *gin.Context) {
	fmt.Println("category list")

	SendErrJSON := common.SendErrJSON

	title := c.Query("title")
	fmt.Println(title)
	var categories []model.Category

	if err := model.DB.Where("title like ?", "%"+title+"%").Find(&categories).Error; err != nil {
		SendErrJSON("error", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": categories,
	})
}

func Detail(c *gin.Context) {
	SendErrJSON := common.SendErrJSON
	fmt.Println("category detail")
	categoryID, err := strconv.Atoi(c.Param("id"))

	if err != nil {
		SendErrJSON("错误的分类id", c)
		return
	}

	var category model.Category

	if err := model.DB.First(&category, categoryID).Error; err != nil {
		fmt.Printf(err.Error())
		SendErrJSON("错误的分类ID", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": category,
	})
}

func Save(c *gin.Context) {
	SendErrJSON := common.SendErrJSON

	var category model.Category

	if err := c.ShouldBindJSON(&category); err != nil {
		fmt.Println(err.Error())
		SendErrJSON("参数无效", c)
		return
	}

	if category.Title == "" {
		SendErrJSON("分类名称不能为空", c)
		return
	}

	var queryCategory model.Category
	if category.ID > 0 {
		if err := model.DB.First(&queryCategory, category.ID).Error; err != nil {
			SendErrJSON("无效的分类ID", c)
			return
		}
		tempCategory := category
		category = queryCategory
		category.Title = tempCategory.Title
		category.Description = tempCategory.Description
		if tempCategory.Logo != "" {
			category.Logo = tempCategory.Logo
		}
	} else {
		category.AddTime = time.Now().Unix()
	}

	category.Title = strings.TrimSpace(utils.AvoidXSS(category.Title))
	category.Description = strings.TrimSpace(utils.AvoidXSS(category.Description))

	if err := model.DB.Save(&category).Error; err != nil {
		SendErrJSON("分类保存失败", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": category,
	})
}

//删除分类
func Delete(c *gin.Context) {
	SendErrJSON := common.SendErrJSON
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		SendErrJSON("无效的id", c)
		return
	}

	var category model.Category
	if err := model.DB.First(&category, id).Error; err != nil {
		SendErrJSON("无效的id", c)
		return
	}

	if err := model.DB.Delete(&category).Error; err != nil {
		SendErrJSON("删除过程中出错", c)
		return
	}

	var sql = "DELETE FROM " + config.DBConfig.TablePrefix + "relation WHERE category_id = ?"
	if err := model.DB.Exec(sql, category.ID).Error; err != nil {
		fmt.Println(err.Error())
		SendErrJSON("error", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": gin.H{
			"id": category.ID,
		},
	})
}
