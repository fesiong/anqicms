package manageController

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func GetWebsiteList(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	name := ctx.URLParam("name")
	baseUrl := ctx.URLParam("base_url")
	dbSites, total := provider.GetDBWebsites(name, baseUrl, currentPage, pageSize)

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
	var adminInfo *model.Admin
	if website != nil {
		adminInfo, err = website.GetAdminInfoById(1)
		if err != nil {
			adminInfo = &model.Admin{}
		}
	} else {
		adminInfo = &model.Admin{}
	}
	result := request.WebsiteRequest{
		Id:        dbSite.Id,
		RootPath:  dbSite.RootPath,
		Name:      dbSite.Name,
		Status:    dbSite.Status,
		Mysql:     dbSite.Mysql,
		AdminUser: adminInfo.UserName,
	}
	if website != nil {
		result.BaseUrl = website.System.BaseUrl
		result.Initialed = website.Initialed
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
			"msg":  ctx.Tr("InsufficientPermissions"),
		})
		return
	}
	var dbSite *model.Website
	var err error
	if !strings.HasPrefix(req.BaseUrl, "http") {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheCorrectSiteDomainName"),
		})
		return
	}
	req.BaseUrl = strings.TrimRight(req.BaseUrl, "/")
	if req.Id > 0 {
		dbSite, err = provider.GetDBWebsiteInfo(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("SiteDoesNotExist"),
			})
			return
		}
		if req.Id != 1 {
			req.RootPath = strings.TrimRight(strings.TrimSpace(strings.ReplaceAll(req.RootPath, "\\", "/")), "/")
			// 全新安装
			if req.RootPath == currentSite.RootPath {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("CannotUseTheDefaultSiteDirectory"),
				})
				return
			}
			if !strings.Contains(req.RootPath, "/") {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("PleaseFillInTheCorrectSiteDirectory"),
				})
				return
			}
			req.RootPath = req.RootPath + "/"
			_, err = os.Stat(req.RootPath)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("FailedToReadTheSiteDirectory"),
				})
				return
			}
			dbSite.RootPath = req.RootPath
		}

		current := provider.GetWebsite(dbSite.Id)
		//修改站点，可以修改全部信息，但是不再同步内容
		dbSite.Name = req.Name
		dbSite.Status = req.Status
		if dbSite.TokenSecret == "" {
			dbSite.TokenSecret = config.GenerateRandString(32)
		}
		err = provider.GetDefaultDB().Save(dbSite).Error
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("ErrorSavingSite"),
			})
			return
		}
		// 修改数据库信息，只有在数据库无法访问的时候才能修改
		if req.Id != 1 {
			// mysql 检查是否与第一个库重名
			if req.Mysql.Database == config.Server.Mysql.Database {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("CannotOverwriteTheDefaultSiteDatabase"),
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
					"msg":  ctx.Tr("DatabaseError"),
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
				"msg":  ctx.Tr("PleaseFillInTheAdministratorPasswordOfMoreThan6Digits"),
			})
			return
		}
		// mysql 检查是否与第一个库重名
		if req.Mysql.Database == config.Server.Mysql.Database {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("CannotOverwriteTheDefaultSiteDatabase"),
			})
			return
		}
		// 如果没有填写数据库名称
		if len(req.Mysql.Database) == 0 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("PleaseFillInTheDatabaseName"),
			})
			return
		}
		req.RootPath = strings.TrimRight(strings.TrimSpace(strings.ReplaceAll(req.RootPath, "\\", "/")), "/")
		if !strings.Contains(req.RootPath, "/") {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("PleaseFillInTheCorrectSiteDirectory"),
			})
			return
		}
		req.RootPath = req.RootPath + "/"
		// 全新安装
		if req.RootPath == currentSite.RootPath {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("CannotUseTheDefaultSiteDirectory"),
			})
			return
		}
		// 先检查数据库
		dbSite = &model.Website{
			RootPath:    req.RootPath,
			Name:        req.Name,
			Status:      req.Status,
			TokenSecret: config.GenerateRandString(32),
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
				"msg":  ctx.Tr("DatabaseError"),
			})
			return
		}

		_, err = os.Stat(req.RootPath)
		if err != nil && os.IsNotExist(err) {
			err = os.MkdirAll(req.RootPath, os.ModePerm)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("FailedToCreateTheSiteDirectory"),
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
		copyTemplate := req.Template
		if copyTemplate == "" {
			// 从模板中选择一套
			copyTemplate = "default"
		}
		designList := currentSite.GetDesignList()
		var designInfo *response.DesignPackage
		for i := range designList {
			if designList[i].Package == copyTemplate {
				designInfo = &designList[i]
				break
			}
		}
		if designInfo == nil && len(designList) > 0 {
			designInfo = &designList[0]
		}
		if copyTemplate != "" {
			dstTplDir := req.RootPath + "template/" + designInfo.Package
			srcTplDir := config.ExecPath + "template/" + designInfo.Package
			_ = library.CopyDir(dstTplDir, srcTplDir)
			dstStaticDir := req.RootPath + "public/static/" + designInfo.Package
			srcStaticDir := config.ExecPath + "public/static/" + designInfo.Package
			_ = library.CopyDir(dstStaticDir, srcStaticDir)
		}

		// 检查通过
		dbSite.Mysql = req.Mysql
		provider.GetDefaultDB().Save(dbSite)
		provider.InitWebsite(dbSite)
		current := provider.GetWebsite(dbSite.Id)
		if current.Initialed {
			// 修改 baseUrl
			if designInfo != nil {
				current.System.TemplateName = designInfo.Package
				current.System.TemplateType = designInfo.TemplateType
			}
			current.System.BaseUrl = req.BaseUrl
			_ = current.SaveSettingValue(provider.SystemSettingKey, current.System)
			current.PluginStorage.StorageUrl = current.System.BaseUrl
			_ = current.SaveSettingValue(provider.StorageSettingKey, current.PluginStorage)
			if req.PreviewData {
				_ = current.RestoreDesignData(current.System.TemplateName)
			}
			// 安装时间
			_ = current.SaveSettingValue(provider.InstallTimeKey, time.Now().Unix())
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
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateMultiSiteLog", dbSite.Id, dbSite.Name))
	// 重启
	config.RestartChan <- 0

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SiteHasBeenSaved"),
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
			"msg":  ctx.Tr("InsufficientPermissions"),
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
			"msg":  ctx.Tr("DefaultSiteCannotBeDeleted"),
		})
		return
	}
	// 只删除数据库记录，不删除实际文件
	provider.GetDefaultDB().Delete(dbSite)
	provider.RemoveWebsite(dbSite.Id, req.RemoveFile)
	// 重载模板
	config.RestartChan <- 0
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
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

func LoginSubWebsite(ctx iris.Context) {
	var req request.WebsiteLoginRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.SiteId == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("SiteDoesNotExist"),
		})
		return
	}
	// 只有默认站点才可以进行站点的创建
	currentSite := provider.CurrentSite(ctx)
	// 只有默认站点和多语言站点的主站点才可以进行站点的创建
	if currentSite.Id != 1 {
		isSub := false
		if currentSite.MultiLanguage.Open {
			// 需要判断是不是子站
			for _, sub := range currentSite.MultiLanguage.SubSites {
				if sub.Id == req.SiteId {
					// 存在这样的子站点
					isSub = true
					break
				}
			}
		}
		if !isSub {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("InsufficientPermissions"),
			})
			return
		}
	}
	subSite := provider.GetWebsite(req.SiteId)
	if subSite == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("SiteDoesNotExist"),
		})
		return
	}
	// 登录第一个账号
	var admin model.Admin
	err := subSite.DB.First(&admin).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("UserDoesNotExist"),
		})
		return
	}
	// 构造登录链接
	nonce := strconv.FormatInt(time.Now().UnixMicro(), 10)
	signHash := sha256.New()
	signHash.Write([]byte(admin.Password + nonce))
	sign := signHash.Sum(nil)

	loginUrl := subSite.System.BaseUrl
	if subSite.System.AdminUrl != "" {
		loginUrl = subSite.System.AdminUrl
	}
	// 如果loginUrl包含了目录，则需要将目录清除
	parsed, err := url.Parse(loginUrl)
	if err == nil {
		if len(parsed.Path) > 1 {
			parsed.Path = ""
			loginUrl = parsed.String()
		}
	}
	link := fmt.Sprintf("%s/system/login?admin-login=true&site_id=%d&user_name=%s&sign=%s&nonce=%s", loginUrl, subSite.Id, admin.UserName, hex.EncodeToString(sign), nonce)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": link,
	})
}
