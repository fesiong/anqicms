package manageController

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kataras/iris/v12"
	"io"
	"io/ioutil"
	"irisweb/config"
	"irisweb/provider"
	"irisweb/request"
	"os"
	"regexp"
)

// HandleCollectSetting 全局配置
func HandleCollectSetting(ctx iris.Context) {
	var collector config.CollectorJson
	//再根据用户配置来覆盖
	buf, err := ioutil.ReadFile(fmt.Sprintf("%scollector.json", config.ExecPath))
	configStr := ""
	if err != nil {
		//文件不存在
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": collector,
		})
		return
	}
	configStr = string(buf[:])
	reg := regexp.MustCompile(`/\*.*\*/`)

	configStr = reg.ReplaceAllString(configStr, "")
	buf = []byte(configStr)

	if err = json.Unmarshal(buf, &collector); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": collector,
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": collector,
	})
}

// HandleSaveCollectSetting 全局配置保存
func HandleSaveCollectSetting(ctx iris.Context) {
	var req config.CollectorJson
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//将现有配置写回文件
	configFile, err := os.OpenFile(fmt.Sprintf("%scollector.json", config.ExecPath), os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	defer configFile.Close()

	buff := &bytes.Buffer{}

	buf, err := json.MarshalIndent(req, "", "\t")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	buff.Write(buf)

	_, err = io.Copy(configFile, buff)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//重新读取配置
	config.LoadCollectorConfig()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
	})
}

func HandleReplaceArticles(ctx iris.Context) {
	var req request.ArticleReplaceRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if len(req.ContentReplace) == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "替换关键词为空",
		})
		return
	}

	go provider.ReplaceArticles(req.ContentReplace)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "替换任务已触发",
	})
}

func HandleArticlePseudo(ctx iris.Context) {
	var req request.Article
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	article, err := provider.GetArticleById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = provider.PseudoOriginalArticle(article)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "伪原创已完成",
	})
}

func HandleDigKeywords(ctx iris.Context) {
	go provider.StartDigKeywords()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "关键词拓词任务已触发",
	})
}
