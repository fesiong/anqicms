package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"net/url"
	"strings"
)

func Install(ctx iris.Context) {
	if dao.DB != nil {
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
  </style>
</head>

<body>
  <div class="container">
    <h1 class="title">安企CMS(AnqiCMS)初始化安装</h1>
    <form class="layui-form" action="/install" method="post">
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
          <div class="layui-form-mid layui-aux-word">指该网站的网址，如：https://www.anqicms.com，用来生成全站的绝对地址</div>
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
</body>

</html>`)
}

var installRunning bool

func InstallForm(ctx iris.Context) {
	if dao.DB != nil {
		ShowMessage(ctx, "已初始化完成，无需再处理", []Button{
			{Name: "点击继续", Link: "/"},
		})
		return
	}
	if installRunning {
		ShowMessage(ctx, "已初始化任务正在进行中", nil)
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

	if len(req.AdminPassword) < 6 {
		ShowMessage(ctx, "请填写6位以上的管理员密码", nil)
		return
	}

	if req.BaseUrl == "" {
		urlPath, err := url.Parse(ctx.FullRequestURI())
		if err == nil {
			req.BaseUrl = urlPath.Scheme + "://" + urlPath.Host
		}
	}
	config.JsonData.System.BaseUrl = strings.TrimRight(req.BaseUrl, "/")
	config.JsonData.PluginStorage.StorageUrl = config.JsonData.System.BaseUrl

	config.Server.Mysql.Database = req.Database
	config.Server.Mysql.User = req.User
	config.Server.Mysql.Password = req.Password
	config.Server.Mysql.Host = req.Host
	config.Server.Mysql.Port = req.Port

	err := dao.InitDB()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	//自动迁移数据库
	err = dao.AutoMigrateDB(dao.DB)
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
	_ = provider.SaveSettingValue(provider.SystemSettingKey, config.JsonData.System)
	_ = provider.SaveSettingValue(provider.StorageSettingKey, config.JsonData.PluginStorage)

	//创建管理员
	err = provider.InitAdmin(req.AdminUser, req.AdminPassword, true)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ShowMessage(ctx, "AnqiCMS安装成功", []Button{
		{Name: "访问管理后台", Link: "/system/"},
		{Name: "访问首页", Link: "/"},
	})
}
