package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"os"
	"path/filepath"
	"strings"
)

func AdminFileServ(ctx iris.Context) {
	uri := ctx.RequestPath(false)
	if uri != "/" {
		baseDir := config.ExecPath
		uriFile := baseDir + uri
		_, err := os.Stat(uriFile)
		if err == nil {
			ctx.ServeFile(uriFile, false)
			return
		}

		if !strings.Contains(filepath.Base(uri), ".") {
			uriFile = uriFile + "/index.html"
			_, err = os.Stat(uriFile)
			if err == nil {
				ctx.ServeFile(uriFile, false)
				return
			}
		}
	}

	ctx.Next()
}

func Version(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"version": config.Version,
		},
	})
}

func Statistics(ctx iris.Context) {
	statistics := provider.Statistics()
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": statistics,
	})
}

// CheckVersion 检查新版
func CheckVersion(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "检查版本功能正在开发",
		"data": iris.Map{
			"version": config.Version,
		},
	})
}

func VersionUpgrade(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "版本升级功能正在开发",
	})
}
