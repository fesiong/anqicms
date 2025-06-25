package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func Install(ctx iris.Context) {
	if provider.GetDefaultDB() != nil {
		ctx.Redirect("/")
		return
	}
	defaultSite := provider.CurrentSite(ctx)

	viewTpl := `<!DOCTYPE html>
<html lang="zh-CN">

<head>
  <meta charset="UTF-8">
  <meta http-equiv="X-UA-Compatible" content="IE=edge">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>安企CMS(AnqiCMS)初始化安装</title>
  <style>
    .container {
      padding: 30px;
    }

    .title {
      text-align: center;
      padding: 20px;
    }

    .layui-form {
      max-width: 600px;
      margin: 50px auto;
      padding: 20px;
      max-width: 600px;
      box-shadow: 0 1px 5px rgba(0, 0, 0, 0.3);
      border-radius: 5px;
    }

    .layui-form-item {
      display: flex;
      margin-bottom: 20px;
      align-items: top;
    }

    .layui-form-label {
      padding: 6px 0;
      width: 100px;
    }

    .layui-input-block {
      flex: 1;
    }

    .layui-form-text {
      padding: 6px 0;
      flex: 1;
    }

    .layui-input {
      box-sizing: border-box;
      width: 100%;
      padding: 2px 10px;
      border: 1px solid #eaeaea;
      border-radius: 4px;
      height: 36px;
      font-size: 15px;
    }

    input:focus,
    textarea:focus {
      outline: 1px solid #29d;
    }

    .layui-form-mid {
      padding: 3px 0;
    }

    .layui-aux-word {
      color: #999;
      font-size: 12px;
    }

    .layui-btn {
      display: inline-block;
      cursor: pointer;
      border-radius: 2px;
      color: #555;
      background-color: #fff;
      padding: 10px 15px;
      margin: 0 5px;
      border: 1px solid #eaeaea;
    }

    .layui-btn.btn-primary {
      color: #fff;
      background-color: #3f90f9;
    }

    .submit-buttons {
      text-align: center;
    }

    a {
      text-decoration: none;
      line-height: 1;
    }

    #loading {
      display: none;
      position: fixed;
      top: 50%;
      left: 50%;
      margin-left: -120px;
      margin-top: -40px;
      padding: 30px;
      width: 180px;
      background: #ffffff;
      box-shadow: 0 1px 5px rgb(0 0 0 / 30%);
      border-radius: 5px;
    }

    .loading-icon {
      margin-left: 5px;
      -webkit-animation-name: typing;
      animation-name: typing;
      -webkit-animation-duration: 3s;
      animation-duration: 3s;
      -webkit-animation-timing-function: steps(14, end);
      animation-timing-function: steps(14, end);
      -webkit-animation-iteration-count: infinite;
      animation-iteration-count: infinite;
      display: inline-block;
      width: 24px;
      overflow: hidden;
      vertical-align: middle;
    }

    @keyframes typing {
      from {
        width: 0
      }
    }

    #alert {
      display: none;
      position: fixed;
      width: 560px;
      top: 50%;
      left: 50%;
      margin-top: -66px;
      margin-left: -300px;
      padding: 20px;
      box-shadow: 0 1px 5px rgb(0 0 0 / 30%);
      border-radius: 5px;
      background: #fff;
    }

    .alert-buttons {
      margin-top: 30px;
      text-align: right;
    }
  </style>
</head>

<body>
  <div class="container">
    <h1 class="title">{% tr "AnqiCMS(AnqiCMS)InitializationInstallation" %}</h1>
    <form class="layui-form" id="install-form" action="/install" method="post" onsubmit="return checkSubmit(this);">
      <div>
        <div class="layui-form-item">
          <label class="layui-form-label">{% tr "DatabaseAddress" %}</label>
          <div class="layui-input-block">
            <input type="text" name="host" value="localhost" required placeholder="{% tr "UsuallyLocalhost" %}" autocomplete="off" class="layui-input">
          </div>
        </div>
        <div class="layui-form-item">
          <label class="layui-form-label">{% tr "DatabasePort" %}</label>
          <div class="layui-input-block">
            <input type="text" name="port" value="3306" required placeholder="{% tr "Usually3306" %}" autocomplete="off" class="layui-input">
          </div>
        </div>
        <div class="layui-form-item">
          <label class="layui-form-label">{% tr "DatabaseName" %}</label>
          <div class="layui-input-block">
            <input type="text" name="database" value="anqicms" required placeholder="{% tr "WhichDatabaseToInstall" %}" autocomplete="off" class="layui-input">
            <div class="layui-form-mid layui-aux-word">{% tr "IfTheDatabaseDoesNotExistTheProgramWillTryToCreateIt" %}</div>
          </div>
        </div>
        <div class="layui-form-item">
          <label class="layui-form-label">{% tr "DatabaseUser" %}</label>
          <div class="layui-input-block">
            <input type="text" name="user" required placeholder="{% tr "FillInTheDatabaseUserName" %}" autocomplete="off" class="layui-input">
          </div>
        </div>
        <div class="layui-form-item">
          <label class="layui-form-label">{% tr "DatabasePassword" %}</label>
          <div class="layui-input-block">
            <input type="password" name="password" placeholder="{% tr "FillInTheDatabasePassword" %}" autocomplete="off" class="layui-input">
          </div>
        </div>
      </div>
      <div class="layui-form-item">
        <label class="layui-form-label">{% tr "BackstageUserName" %}</label>
        <div class="layui-input-block">
          <input type="text" name="admin_user" value="admin" required placeholder="{% tr "UsedToLogInToTheManagementBackstage" %}" autocomplete="off" class="layui-input">
        </div>
      </div>
      <div class="layui-form-item">
        <label class="layui-form-label">{% tr "BackstagePassword" %}</label>
        <div class="layui-input-block">
          <input type="password" name="admin_password" minlength="6" maxlength="20" required placeholder="{% tr "PleaseFillInAPasswordOfMoreThan6Digits" %}" autocomplete="off" class="layui-input">
        </div>
      </div>
      <div class="layui-form-item">
        <label class="layui-form-label">{% tr "WebsiteAddress" %}</label>
        <div class="layui-input-block">
          <input type="text" name="base_url" value="" autocomplete="off" class="layui-input">
          <div class="layui-form-mid layui-aux-word">{% tr "RefersToTheWebsitesUrl" %}</div>
        </div>
      </div>
      <div class="layui-form-item">
        <label class="layui-form-label">{% tr "DemoData" %}</label>
        <div class="layui-form-text">
          <label><input type="checkbox" name="preview_data" value="1">{% tr "Install" %}</label>
          <span class="layui-form-mid layui-aux-word">{% tr "AfterCheckingTheDefaultDemoDataWillBeInstalled" %}</span>
        </div>
      </div>
      <div class="layui-form-item">
        <div class="layui-input-block submit-buttons">
          <button type="reset" class="layui-btn">{% tr "Reset" %}</button>
          <button class="layui-btn btn-primary" type="submit">{% tr "ConfirmInitialization" %}</button>
        </div>
      </div>
    </form>
  </div>
  <div id="loading">{% tr "InstallingInProgress" %}<span class="loading-icon">···</span></div>
  <div id="alert"></div>
</body>
<script>
  let installing = false;
  function checkSubmit(form) {
    if (installing) {
      return false;
    }
    let el = document.getElementById("loading");
    el.style.display = "block";
    installing = true;
    let formData = new FormData(form);
    var xhr = new XMLHttpRequest();
    xhr.open("POST", "/install");
    xhr.send(formData);
    xhr.onload = function () {
      let res = JSON.parse(xhr.responseText);
      if (res.code !== 0) {
        showAlert(res.msg, []);
      } else {
        showAlert(res.msg, [{ name: '{% tr "AccessTheManagementBackstage" %}', link: '/system/' }, { name: '{% tr "AccessTheHomepage" %}', link: '/' }]);
      }
      el.style.display = "none";
      installing = false;
    }
    return false;
  }
  function closeAlert() {
    let el = document.getElementById("alert");
    el.style.display = "none";
  }
  function showAlert(message, buttons) {
    let el = document.getElementById("alert");
    el.style.display = "block";
    let text = "<div>" + message + "</div><div class=\"alert-buttons\"><a class=\"layui-btn\" href=\"javascript:closeAlert();\">{% tr "Confirm" %}</a>";
    if (buttons.length > 0) {
      for (let i in buttons) {
        text += "<a class=\"layui-btn btn-primary\" href=\"" + buttons[i].link + "\">" + buttons[i].name + "</a>";
      }
    }
    el.innerHTML = text;
  }
</script>

</html>`
	// translate
	re, _ := regexp.Compile(`{%\s*tr "(.+?)"\s*%}`)
	viewTpl = re.ReplaceAllStringFunc(viewTpl, func(s string) string {
		match := re.FindStringSubmatch(s)
		return defaultSite.Tr(match[1])
	})

	ctx.WriteString(viewTpl)
}

var installRunning bool

func InstallForm(ctx iris.Context) {
	defaultSite := provider.CurrentSite(ctx)
	if provider.GetDefaultDB() != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  defaultSite.Tr("InitializationCompleted"),
		})
		return
	}
	if installRunning {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  defaultSite.Tr("InitializedTaskInProgress"),
		})
		return
	}
	installRunning = true
	defer func() {
		installRunning = false
	}()
	var req request.Install
	// 采用post提交
	req.Database = ctx.PostValueTrim("database")
	req.User = ctx.PostValueTrim("user")
	req.Password = ctx.PostValueTrim("password")
	req.Host = ctx.PostValueTrim("host")
	req.Port = ctx.PostValueIntDefault("port", 3306)
	req.AdminUser = ctx.PostValueTrim("admin_user")
	req.AdminPassword = ctx.PostValueTrim("admin_password")
	req.BaseUrl = ctx.PostValueTrim("base_url")
	req.PreviewData, _ = ctx.PostValueBool("preview_data")

	if len(req.AdminPassword) < 6 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  defaultSite.Tr("PleaseFillInTheAdministratorPasswordOfMoreThan6Digits"),
		})
		return
	}

	if req.BaseUrl == "" {
		urlPath, err := url.Parse(ctx.FullRequestURI())
		if err == nil {
			host := urlPath.Host
			if strings.HasSuffix(host, ":80") || strings.HasSuffix(host, ":443") {
				host = strings.Split(host, ":")[0]
			}
			req.BaseUrl = urlPath.Scheme + "://" + host
		}
	}

	var mysqlConfig = config.MysqlConfig{
		Database: req.Database,
		User:     req.User,
		Password: req.Password,
		Host:     req.Host,
		Port:     req.Port,
	}

	db, err := provider.InitDB(&mysqlConfig)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	config.Server.Mysql = mysqlConfig
	provider.SetDefaultDB(db)

	//自动迁移数据库
	err = provider.AutoMigrateDB(db, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	dbWebsite := model.Website{
		// 首个站点ID为1
		Model: model.Model{Id: 1},
		// 首个站点为安装目录
		RootPath:    config.ExecPath,
		Name:        defaultSite.Tr("AnqiCMS(AnqiCMS)"),
		Mysql:       config.Server.Mysql,
		TokenSecret: config.GenerateRandString(32),
		Status:      1,
	}
	db.Save(&dbWebsite)

	provider.InitWebsite(&dbWebsite)
	website := provider.GetWebsite(dbWebsite.Id)

	website.System.BaseUrl = strings.TrimRight(req.BaseUrl, "/")
	website.PluginStorage.StorageUrl = website.System.BaseUrl
	_ = website.SaveSettingValue(provider.SystemSettingKey, website.System)
	_ = website.SaveSettingValue(provider.StorageSettingKey, website.PluginStorage)

	if req.PreviewData {
		_ = website.RestoreDesignData(website.System.TemplateName)
	}
	// 安装时间
	_ = website.SaveSettingValue(provider.InstallTimeKey, time.Now().Unix())
	// 读入配置
	website.InitSetting()
	// 初始化数据
	website.InitModelData()
	//创建管理员
	err = website.InitAdmin(req.AdminUser, req.AdminPassword, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	config.RestartChan <- 0

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  defaultSite.Tr("AnqiCMSInstalledSuccessfully"),
	})
}
