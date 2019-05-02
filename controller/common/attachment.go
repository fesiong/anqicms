package common

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"

	"goblog/model"

	"github.com/gin-gonic/gin"
)

// Upload 文件上传
func Upload(c *gin.Context) (model.Attachment, error) {
	file, err := c.FormFile("upFile")

	var attachment model.Attachment

	if err != nil {
		fmt.Println(err.Error())
		return attachment, errors.New("参数无效")
	}

	var filename = file.Filename
	var index = strings.LastIndex(filename, ".")

	if index < 0 {
		return attachment, errors.New("无效的文件名")
	}

	var ext = filename[index:]
	if len(ext) == 1 {
		return attachment, errors.New("无效的扩展名")
	}
	var mimeType = mime.TypeByExtension(ext)

	fmt.Printf("filename %s, index %d, ext %s, mimeType %s\n", filename, index, ext, mimeType)
	if mimeType == "" && ext == ".jpeg" {
		mimeType = "image/jpeg"
	}
	if mimeType == "" {
		return attachment, errors.New("无效的图片类型")
	}

	imgUploadedInfo := model.GenerateImgUploadedInfo(ext)

	fmt.Println(imgUploadedInfo.UploadDir)

	if err := os.MkdirAll(imgUploadedInfo.UploadDir, 0777); err != nil {
		fmt.Println(err.Error())
		return attachment, errors.New("error")
	}

	if err := c.SaveUploadedFile(file, imgUploadedInfo.UploadFilePath); err != nil {
		fmt.Println(err.Error())
		return attachment, errors.New("error1")
	}

	attachment = model.Attachment{
		Title:    imgUploadedInfo.Filename,
		Location: imgUploadedInfo.ImgURL,
		FileSize: 0,
		Width:    0,
		Height:   0,
		Mime:     mimeType,
	}

	if err := model.DB.Create(&attachment).Error; err != nil {
		fmt.Println(err.Error())
		return attachment, errors.New("image error")
	}

	return attachment, nil
}

// UploadHandler 文件上传
func UploadHandler(c *gin.Context) {
	data, err := Upload(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": model.ErrorCode.ERROR,
			"msg":  err.Error(),
			"data": gin.H{},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": data,
	})
}

func DelateAttachment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		SendErrJSON("无效的id", c)
		return
	}

	var attachment model.Attachment
	if err := model.DB.First(&attachment, id).Error; err != nil {
		SendErrJSON("无效的id", c)
		return
	}

	if err := model.DB.Delete(&attachment).Error; err != nil {
		SendErrJSON("删除过程中出错", c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": model.ErrorCode.SUCCESS,
		"msg":  "success",
		"data": gin.H{
			"id": attachment.ID,
		},
	})
}
