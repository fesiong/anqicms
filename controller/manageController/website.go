package manageController

import (
	"fmt"
	"net/url"
	"os"
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

// GetWebsiteList 获取多站点的站点列表，支持分页和名称搜索。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "name": 站点名称模糊搜索。
//   - 查询参数 "base_url": 站点网址全匹配搜索。
func GetWebsiteList(ctx iris.Context) {
	// 当前页码
	currentPage := ctx.URLParamIntDefault("current", 1)
	// 每页条数
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	// 搜索站点名称
	name := ctx.URLParam("name")
	// 站点网址(全匹配)
	baseUrl := ctx.URLParam("base_url")
	dbSites, total := provider.GetDBWebsites(name, baseUrl, currentPage, pageSize)

	// 必须脱敏：此前直接返回 model.Website，`mysql.password`（实测 "root"）
	// 与 `token_secret` 都是明文。detail 接口早已改用 WebsiteDetailResponse，
	// list 漏了同一处——两个入口读同一批数据，只堵一个等于没堵。
	// 2026-10-27 由 MCP 全接口测试发现：siteops_website/list 能读到库密码。
	//
	// 与 detail 共用一个构造函数，避免两个入口各自拼接字段再次分叉
	// （历史上 list 就是这样漏掉脱敏的）。list 不需要管理员用户名，传空串。
	list := make([]request.WebsiteDetailResponse, 0, len(dbSites))
	for _, site := range dbSites {
		list = append(list, buildWebsiteDetailResponse(site, provider.GetWebsite(site.Id), ""))
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  list,
	})
}

// GetWebsiteInfo 获取指定站点的详细信息，包括站点配置、域名、初始化状态和管理员账号。
//
// 参数说明：
//   - 查询参数 "id": 站点 ID，默认为 0。
func GetWebsiteInfo(ctx iris.Context) {
	// 站点 ID
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
	// 用只读结构体而不是 WebsiteRequest：后者带 AdminPassword，
	// json tag 无 omitempty，空值也会出现在响应里（见 WebsiteDetailResponse 的注释）。
	result := buildWebsiteDetailResponse(dbSite, website, adminInfo.UserName)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// buildWebsiteDetailResponse 把库里的站点行与运行时注册表里的站点合成详情响应。
//
// 为什么要抽成纯函数：`GetWebsiteInfo` 原来只判 `website != nil` 就直接读
// `website.System.BaseUrl`。而注册表里可能存在**半初始化**站点——InitWebsite
// 先 `websites.Set(&w)` 再 `w.InitSetting()`，而 InitSetting 在 DB 连不上时
// 直接 return，于是该站点的 `System` 永远是 nil。点进这种站点的详情页就会空指针
// panic，被 recover 兜成 500「服务器内部错误」。
//
// 2026-11-15 的线上 error.log 里 139 条 panic 全是这一类（列表接口）；详情接口
// 是同一个坑的另一个入口，一并堵掉。抽出来后可以脱离数据库直接测。
func buildWebsiteDetailResponse(dbSite *model.Website, site *provider.Website, adminUser string) request.WebsiteDetailResponse {
	var result request.WebsiteDetailResponse
	if dbSite == nil {
		return result
	}
	result.Id = dbSite.Id
	result.RootPath = dbSite.RootPath
	result.Name = dbSite.Name
	result.Status = dbSite.Status
	result.Mysql = request.SafeMysql(dbSite.Mysql)
	result.AdminUser = adminUser
	result.CreatedTime = dbSite.CreatedTime
	// 站点 1 的库信息以配置为准（库里那行可能是旧值），按 id 判断而不是行号。
	if dbSite.Id == 1 {
		result.Mysql = request.SafeMysql(config.Server.Mysql)
	}
	if site != nil {
		result.Initialed = site.Initialed
		// System == nil 时不补运行时字段，退化成库里的值（Status 已由
		// provider.GetDBWebsiteInfo 归 0 表示不可用）。
		if site.System != nil {
			result.BaseUrl = site.System.BaseUrl
			result.FrontUrl = site.System.FrontUrl
		}
	}
	return result
}

// SaveWebsiteInfo 保存或创建多站点信息，仅默认站点（ID 为 1）可操作，保存后会重启对应站点。
//
// 参数说明：
//   - 请求体 "id": 站点 ID，大于 0 表示修改，否则为新建站点。
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
				if os.IsNotExist(err) {
					// 创建目录
					err = os.MkdirAll(req.RootPath, os.ModePerm)
					if err != nil {
						ctx.JSON(iris.Map{
							"code": config.StatusFailed,
							"msg":  ctx.Tr("FailedToReadTheSiteDirectory"),
						})
						return
					}
				} else {
					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  ctx.Tr("FailedToReadTheSiteDirectory"),
					})
					return
				}
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
					Id:      1,
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
				// 站点被禁用：这里不经过 InitWebsite，必须显式停掉渠道管理器，
				// 否则它会继续连平台、续租并往 Inbox 落库。
				current.StopChannelManager()
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
		} else if req.Mysql.Password == request.MysqlPasswordMask {
			// 回写保护：读接口的密码是掩码（见 request.SafeMysql）。
			// 调用方若把读到的详情原样保存，掩码会被写进库——
			// 数据库密码变成 "********"，站点直接连不上库且不报错。
			// 这里沿用库里的真值。
			req.Mysql.Password = dbSite.Mysql.Password
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
				// 站点被禁用：同上方分支，未经过 InitWebsite 也要停掉渠道管理器。
				current.StopChannelManager()
				current.Initialed = false
			}
		}
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateMultiSiteLog", dbSite.Id, dbSite.Name))
	// 重启
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: dbSite.Id}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SiteHasBeenSaved"),
		// 只回安全字段：dbSite 含 Mysql 密码与 TokenSecret，
		// 而本响应会经 MCP 进入模型上下文，不能整对象序列化出去。
		"data": iris.Map{
			"id":        dbSite.Id,
			"name":      dbSite.Name,
			"root_path": dbSite.RootPath,
			"status":    dbSite.Status,
			"parent_id": dbSite.ParentId,
		},
	})
}

// DeleteWebsite 删除多站点的数据库记录，仅默认站点（ID 为 1）可操作，默认站点不可删除。
//
// 参数说明：
//   - 请求体 "id": 待删除的站点 ID。
//   - 请求体 "remove_file": 是否同时删除站点文件。
func DeleteWebsite(ctx iris.Context) {
	var req request.WebsiteDeleteRequest
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
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: dbSite.Id}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

// GetCurrentSiteInfo 获取当前登录站点的 ID、域名和名称信息。
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

// LoginSubWebsite 以子站点身份登录后台，用于从默认站点或多语言主站点切换到指定子站点进行管理。
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
	if admin.Status != 1 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InsufficientPermissions"),
		})
		return
	}
	// 构造登录链接：票据用目标站点的密钥签发，短时有效且只能用一次
	nonce := provider.MintAdminSSONonce(provider.AdminSSOTTL)
	if nonce == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("LoginFailed"),
		})
		return
	}
	sign := provider.SignAdminSSO(subSite.TokenSecret, admin.UserName, nonce)

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
	link := fmt.Sprintf("%s/system/login?admin-login=true&site_id=%d&user_name=%s&sign=%s&nonce=%s",
		loginUrl, subSite.Id, url.QueryEscape(admin.UserName), sign, url.QueryEscape(nonce))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": link,
	})
}
