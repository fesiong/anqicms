package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"io/ioutil"
	"irisweb/config"
	"irisweb/library"
	"irisweb/request"
	"os"
	"path"
	"time"
)

func PluginFileUploadList(ctx iris.Context) {
	uploadFiles := config.JsonData.PluginUploadFiles

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": uploadFiles,
	})
}

func PluginFileUploadDelete(ctx iris.Context) {
	var req request.PluginFileUploadDelete
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	uploadFiles := config.JsonData.PluginUploadFiles

	fileName := ""
	for i, v := range uploadFiles {
		if v.Hash == req.Hash {
			fileName = v.FileName

			config.JsonData.PluginUploadFiles = append(config.JsonData.PluginUploadFiles[:i], config.JsonData.PluginUploadFiles[i+1:]...)
		}
	}

	if fileName == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "文件不正确",
		})
		return
	}

	//执行物理删除
	err := os.Remove(fmt.Sprintf("%spublic/%s", config.ExecPath, fileName))

	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//更新文件列表
	err = config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

//上传，只允许上传txt,htm,html
func PluginFileUploadUpload(ctx iris.Context) {
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}
	defer file.Close()

	ext := path.Ext(info.Filename)

	if ext != ".txt" && ext != ".htm" && ext != ".html" {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    "只允许上传txt/htm/html",
		})
		return
	}

	filePath := fmt.Sprintf("%spublic/%s", config.ExecPath, info.Filename)
	buff, err := ioutil.ReadAll(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    "读取失败",
		})
		return
	}

	err = ioutil.WriteFile(filePath, buff, 0644)
	if err != nil {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    "文件保存失败",
		})
		return
	}

	//检查是否已经在
	exists := false
	for _, v := range config.JsonData.PluginUploadFiles {
		if v.FileName == info.Filename {
			exists = true
		}
	}

	if !exists {
		//追加
		config.JsonData.PluginUploadFiles = append(config.JsonData.PluginUploadFiles, config.PluginUploadFile{
			Hash:        library.Md5(info.Filename),
			FileName:    info.Filename,
			CreatedTime: time.Now().Unix(),
		})
	}

	err = config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文件已上传完成",
	})
}