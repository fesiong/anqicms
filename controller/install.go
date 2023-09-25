package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"net/url"
	"strings"
)

func Install(ctx iris.Context) {
	if provider.GetDefaultDB() != nil {
		ctx.Redirect("/")
		return
	}

	ctx.WriteString(`<!DOCTYPE html>
<html lang="zh-cn">

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
    <h1 class="title">安企CMS(AnqiCMS)初始化安装</h1>
    <form class="layui-form" id="install-form" action="/install" method="post" onsubmit="return checkSubmit(this);">
      <div>
        <div class="layui-form-item">
          <label class="layui-form-label">数据库地址</label>
          <div class="layui-input-block">
            <input type="text" name="host" value="localhost" required placeholder="一般是localhost" autocomplete="off" class="layui-input">
          </div>
        </div>
        <div class="layui-form-item">
          <label class="layui-form-label">数据库端口</label>
          <div class="layui-input-block">
            <input type="text" name="port" value="3306" required placeholder="一般是3306" autocomplete="off" class="layui-input">
          </div>
        </div>
        <div class="layui-form-item">
          <label class="layui-form-label">数据库名称</label>
          <div class="layui-input-block">
            <input type="text" name="database" value="anqicms" required placeholder="安装到哪个数据库" autocomplete="off" class="layui-input">
            <div class="layui-form-mid layui-aux-word">如果数据库不存在，程序则会尝试创建它</div>
          </div>
        </div>
        <div class="layui-form-item">
          <label class="layui-form-label">数据库用户</label>
          <div class="layui-input-block">
            <input type="text" name="user" required placeholder="填写数据库用户名" autocomplete="off" class="layui-input">
          </div>
        </div>
        <div class="layui-form-item">
          <label class="layui-form-label">数据库密码</label>
          <div class="layui-input-block">
            <input type="password" name="password" required placeholder="填写数据库密码" autocomplete="off" class="layui-input">
          </div>
        </div>
      </div>
      <div class="layui-form-item">
        <label class="layui-form-label">后台用户名</label>
        <div class="layui-input-block">
          <input type="text" name="admin_user" value="admin" required placeholder="用于登录管理后台" autocomplete="off" class="layui-input">
        </div>
      </div>
      <div class="layui-form-item">
        <label class="layui-form-label">后台密码</label>
        <div class="layui-input-block">
          <input type="password" name="admin_password" minlength="6" maxlength="20" required placeholder="请填写6位以上的密码" autocomplete="off" class="layui-input">
        </div>
      </div>
      <div class="layui-form-item">
        <label class="layui-form-label">网站地址</label>
        <div class="layui-input-block">
          <input type="text" name="base_url" value="" autocomplete="off" class="layui-input">
          <div class="layui-form-mid layui-aux-word">指该网站的网址，如：http://www.anqicms.com，如本地测试请勿填写。</div>
        </div>
      </div>
      <div class="layui-form-item">
        <label class="layui-form-label">演示数据</label>
        <div class="layui-form-text">
          <label><input type="checkbox" name="preview_data" value="1" checked>安装</label>
          <span class="layui-form-mid layui-aux-word">勾选后，将安装默认演示数据</span>
        </div>
      </div>
      <div class="layui-form-item">
        <div class="layui-input-block submit-buttons">
          <button type="reset" class="layui-btn">重置</button>
          <button class="layui-btn btn-primary" type="submit">确认初始化</button>
        </div>
      </div>
    </form>
  </div>
  <div id="loading">正在安装中，请稍候<span class="loading-icon">···</span></div>
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
        showAlert(res.msg, [{ name: '访问管理后台', link: '/system/' }, { name: '访问首页', link: '/' }]);
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
    let text = "<div>" + message + "</div><div class=\"alert-buttons\"><a class=\"layui-btn\" href=\"javascript:closeAlert();\">确定</a>";
    if (buttons.length > 0) {
      for (let i in buttons) {
        text += "<a class=\"layui-btn btn-primary\" href=\"" + buttons[i].link + "\">" + buttons[i].name + "</a>";
      }
    }
    el.innerHTML = text;
  }
</script>

</html>`)
}

var installRunning bool

func InstallForm(ctx iris.Context) {
	if provider.GetDefaultDB() != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "已初始化完成，无需再处理",
		})
		return
	}
	if installRunning {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "已初始化任务正在进行中",
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
			"msg":  "请填写6位以上的管理员密码",
		})
		return
	}

	if req.BaseUrl == "" {
		urlPath, err := url.Parse(ctx.FullRequestURI())
		if err == nil {
			req.BaseUrl = urlPath.Scheme + "://" + urlPath.Host
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
	err = provider.AutoMigrateDB(db)
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
		// 收个站点为安装目录
		RootPath: config.ExecPath,
		Name:     "安企CMS(AnqiCMS)",
		Mysql:    config.Server.Mysql,
		Status:   1,
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
		"msg":  "AnqiCMS安装成功",
	})
}
