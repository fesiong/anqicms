package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"os"
	"strings"
)

func GetWebsiteList(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	dbSites, total := provider.GetDBWebsites(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  dbSites,
	})
}

func GetWebsiteInfo(ctx iris.Context) {
	id := uint(ctx.URLParamIntDefault("id", 0))

	dbSite, err := provider.GetDBWebsiteInfo(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
			"data": nil,
		})
		return
	}

	website := provider.GetWebsite(dbSite.Id)
	adminInfo, err := website.GetAdminInfoById(1)
	if err != nil {
		adminInfo = &model.Admin{}
	}
	result := request.WebsiteRequest{
		Id:        dbSite.Id,
		RootPath:  dbSite.RootPath,
		Name:      dbSite.Name,
		Status:    dbSite.Status,
		Mysql:     dbSite.Mysql,
		AdminUser: adminInfo.UserName,
		BaseUrl:   website.System.BaseUrl,
		Initialed: website.Initialed,
	}
	if dbSite.Id == 1 {
		result.Mysql = config.Server.Mysql
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

func SaveWebsiteInfo(ctx iris.Context) {
	var req request.WebsiteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite := provider.CurrentSite(ctx)
	// 只有默认站点才可以进行站点的创建
	if currentSite.Id != 1 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "权限不足",
		})
		return
	}
	var dbSite *model.Website
	var err error
	if !strings.HasPrefix(req.BaseUrl, "http") {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请填写正确的站点域名",
		})
		return
	}
	req.BaseUrl = strings.TrimRight(req.BaseUrl, "/")
	if req.Id > 0 {
		dbSite, err = provider.GetDBWebsiteInfo(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "站点不存在",
			})
			return
		}
		if req.Id != 1 {
			req.RootPath = strings.TrimRight(strings.TrimSpace(strings.ReplaceAll(req.RootPath, "\\", "/")), "/")
			// 全新安装
			if req.RootPath == currentSite.RootPath {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  "不能使用默认站点目录",
				})
				return
			}
			if !strings.Contains(req.RootPath, "/") {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  "请填写站点正确的目录",
				})
				return
			}
			req.RootPath = req.RootPath + "/"
			_, err = os.Stat(req.RootPath)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  "站点目录读取失败",
				})
				return
			}
		}

		current := provider.GetWebsite(dbSite.Id)
		//修改站点，可以修改全部信息，但是不再同步内容
		dbSite.Name = req.Name
		dbSite.Status = req.Status
		err = provider.GetDefaultDB().Save(dbSite).Error
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "保存站点出错",
			})
			return
		}
		// 修改数据库信息，只有在数据库无法访问的时候才能修改
		if req.Id != 1 {
			// mysql 检查是否与第一个库重名
			if req.Mysql.Database == config.Server.Mysql.Database {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  "不能覆盖默认站点数据库",
				})
				return
			}
			if req.Mysql.UseDefault {
				req.Mysql.User = config.Server.Mysql.User
				req.Mysql.Password = config.Server.Mysql.Password
				req.Mysql.Host = config.Server.Mysql.Host
				req.Mysql.Port = config.Server.Mysql.Port
			}
			_, err = provider.InitDB(&req.Mysql)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  "数据库信息错误",
				})
				return
			}
			// 检查通过
			dbSite.Mysql = req.Mysql
			provider.GetDefaultDB().Save(dbSite)
			provider.InitWebsite(dbSite)
			current = provider.GetWebsite(dbSite.Id)
		}
		if current.Initialed {
			// 修改信息
			adminInfo, err := current.GetAdminInfoById(1)
			if err != nil {
				adminInfo = &model.Admin{
					Model:   model.Model{Id: 1},
					Status:  1,
					GroupId: 1,
				}
			}
			if adminInfo.UserName != req.AdminUser {
				adminInfo.UserName = req.AdminUser
			}
			if req.AdminPassword != "" {
				adminInfo.EncryptPassword(req.AdminPassword)
			}
			current.DB.Save(adminInfo)
			// 修改 baseUrl
			if current.System.BaseUrl != req.BaseUrl {
				current.System.BaseUrl = req.BaseUrl
				_ = current.SaveSettingValue(provider.SystemSettingKey, current.System)
				if current.PluginStorage.StorageType == config.StorageTypeLocal {
					current.PluginStorage.StorageUrl = req.BaseUrl
					_ = current.SaveSettingValue(provider.StorageSettingKey, current.PluginStorage)
				}
			}
			if dbSite.Status != 1 {
				current.Initialed = false
			}
		}
	} else {
		if len(req.AdminPassword) < 6 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "请填写6位以上的管理员密码",
			})
			return
		}
		// mysql 检查是否与第一个库重名
		if req.Mysql.Database == config.Server.Mysql.Database {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "不能覆盖默认站点数据库",
			})
			return
		}
		req.RootPath = strings.TrimRight(strings.TrimSpace(strings.ReplaceAll(req.RootPath, "\\", "/")), "/")
		// 全新安装
		if req.RootPath == currentSite.RootPath {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "不能使用默认站点目录",
			})
			return
		}
		if !strings.Contains(req.RootPath, "/") {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "请填写站点正确的目录",
			})
			return
		}
		req.RootPath = req.RootPath + "/"
		_, err = os.Stat(req.RootPath)
		if err != nil && os.IsNotExist(err) {
			err = os.MkdirAll(req.RootPath, os.ModePerm)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  "创建站点目录失败",
				})
				return
			}
		}
		// 复制基本信息
		os.MkdirAll(req.RootPath+"cache", os.ModePerm)
		os.MkdirAll(req.RootPath+"data", os.ModePerm)
		dstXslFile1 := req.RootPath + "public/anqi-index.xsl"
		srcXslFile1 := config.ExecPath + "public/anqi-index.xsl"
		_, _ = library.CopyFile(dstXslFile1, srcXslFile1)
		dstXslFile2 := req.RootPath + "public/anqi-style.xsl"
		srcXslFile2 := config.ExecPath + "public/anqi-style.xsl"
		_, _ = library.CopyFile(dstXslFile2, srcXslFile2)
		// 创建站点的模板信息
		dstTplDir := req.RootPath + "template/default"
		srcTplDir := config.ExecPath + "template/default"
		_ = library.CopyDir(dstTplDir, srcTplDir)
		dstStaticDir := req.RootPath + "public/static/default"
		srcStaticDir := config.ExecPath + "public/static/default"
		_ = library.CopyDir(dstStaticDir, srcStaticDir)

		dbSite = &model.Website{
			RootPath: req.RootPath,
			Name:     req.Name,
			Status:   req.Status,
		}
		if req.Mysql.UseDefault {
			req.Mysql.User = config.Server.Mysql.User
			req.Mysql.Password = config.Server.Mysql.Password
			req.Mysql.Host = config.Server.Mysql.Host
			req.Mysql.Port = config.Server.Mysql.Port
		}
		_, err := provider.InitDB(&req.Mysql)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "数据库信息错误",
			})
			return
		}
		// 检查通过
		dbSite.Mysql = req.Mysql
		provider.GetDefaultDB().Save(dbSite)
		provider.InitWebsite(dbSite)
		current := provider.GetWebsite(dbSite.Id)
		if current.Initialed {
			// 修改 baseUrl
			current.System.BaseUrl = req.BaseUrl
			_ = current.SaveSettingValue(provider.SystemSettingKey, current.System)
			current.PluginStorage.StorageUrl = current.System.BaseUrl
			_ = current.SaveSettingValue(provider.StorageSettingKey, current.PluginStorage)
			if req.PreviewData {
				_ = current.RestoreDesignData(current.System.TemplateName)
			}
			//创建管理员
			err = current.InitAdmin(req.AdminUser, req.AdminPassword, true)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
			if dbSite.Status != 1 {
				current.Initialed = false
			}
		}
	}
	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新多站点信息：%d => %s", dbSite.Id, dbSite.Name))
	// 重启
	config.RestartChan <- false

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "站点信息已保存",
	})
}

func DeleteWebsite(ctx iris.Context) {
	var req request.WebsiteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite := provider.CurrentSite(ctx)
	// 只有默认站点才可以进行站点的创建
	if currentSite.Id != 1 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "权限不足",
		})
		return
	}
	dbSite, err := provider.GetDBWebsiteInfo(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if dbSite.Id == 1 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "默认站点不可删除",
		})
		return
	}
	// 只删除数据库记录，不删除实际文件
	provider.GetDefaultDB().Delete(dbSite)
	provider.RemoveWebsite(dbSite.Id)
	// 重载模板
	config.RestartChan <- false
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func GetCurrentSiteInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	website, err := provider.GetDBWebsiteInfo(currentSite.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"id":       website.Id,
			"base_url": currentSite.System.BaseUrl,
			"name":     website.Name,
		},
	})
}
