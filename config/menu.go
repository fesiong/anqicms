package config

type MenuGroup struct {
	Key   string  `json:"key"`
	Name  string  `json:"name"`
	Menus []*Menu `json:"menus"`
}

type Menu struct {
	GroupKey string `json:"group_key"`
	Name     string `json:"name"`
	Path     string `json:"path"` // 前端路由
	Backend  string `json:"-"`    // 后端路由
}

var DefaultMenuGroups = []*MenuGroup{
	{
		Key:  "setting",
		Name: "后台设置",
		Menus: []*Menu{
			{
				Path:     "/setting/system",
				GroupKey: "setting",
				Name:     "全局设置",
				Backend:  "/setting/system",
			},
			{
				Path:     "/setting/content",
				GroupKey: "setting",
				Name:     "内容设置",
			},
			{
				Path:     "/setting/safe",
				GroupKey: "setting",
				Name:     "内容安全设置",
				Backend:  "/setting/safe",
			},
			{
				Path:     "/setting/sensitive",
				GroupKey: "setting",
				Name:     "敏感词设置",
				Backend:  "/setting/sensitive",
			},
			{
				Path:     "/setting/contact",
				GroupKey: "setting",
				Name:     "联系方式设置",
				Backend:  "/setting/contact",
			},
			{
				Path:     "/setting/tdk",
				GroupKey: "setting",
				Name:     "首页TDK设置",
				Backend:  "/setting/index",
			},
			{
				Path:     "/setting/banner",
				GroupKey: "setting",
				Name:     "首页幻灯片",
				Backend:  "/setting/banner",
			},
			{
				Path:     "/setting/nav",
				GroupKey: "setting",
				Name:     "导航设置",
				Backend:  "/setting/nav",
			},
			{
				Path:     "/setting/diyfield",
				GroupKey: "setting",
				Name:     "自定义字段",
				Backend:  "/setting/diyfield",
			},
			{
				Path:     "/setting/ai",
				GroupKey: "setting",
				Name:     "AI统一配置",
				Backend:  "/setting/ai",
			},
		},
	},
	{
		Key:  "archive",
		Name: "内容管理",
		Menus: []*Menu{
			{
				Path:     "/archive/list",
				GroupKey: "archive",
				Name:     "文档列表",
				Backend:  "/archive/list",
			},
			{
				Path:     "/archive/recycle",
				GroupKey: "archive",
				Name:     "文档回收站",
				Backend:  "/archive/list",
			},
			{
				Path:     "/archive/detail",
				GroupKey: "archive",
				Name:     "文档编辑",
				Backend:  "/archive/detail",
			},
			{
				Path:     "/archive/category",
				GroupKey: "archive",
				Name:     "文档分类",
				Backend:  "/category/list",
			},
			{
				Path:     "/archive/category/detail",
				GroupKey: "archive",
				Name:     "文档分类详情",
				Backend:  "/category/detail",
			},
			{
				Path:     "/archive/tag",
				GroupKey: "archive",
				Name:     "文档标签",
				Backend:  "/plugin/tag",
			},
			{
				Path:     "/archive/page",
				GroupKey: "archive",
				Name:     "单页面管理",
				Backend:  "/category/list",
			},
			{
				Path:     "/archive/page/detail",
				GroupKey: "archive",
				Name:     "单页面详情",
				Backend:  "/category/detail",
			},
			{
				Path:     "/archive/module",
				GroupKey: "archive",
				Name:     "内容模型",
				Backend:  "/module/detail",
			},
			{
				Path:     "/archive/attachment",
				GroupKey: "archive",
				Name:     "图片资源管理",
				Backend:  "/attachment",
			},
			{
				Path:     "/archive/place",
				GroupKey: "archive",
				Name:     "城市分站管理",
				Backend:  "/plugin/place",
			},
		},
	},
	{
		Key:  "plugin",
		Name: "功能资源",
		Menus: []*Menu{
			{
				Path:     "/plugin/index",
				GroupKey: "plugin",
				Name:     "功能列表",
				Backend:  "/plugin/index",
			},
			{
				Path:     "/plugin/seo/rewrite",
				GroupKey: "plugin",
				Name:     "伪静态规则管理",
				Backend:  "/plugin/rewrite",
			},
			{
				Path:     "/plugin/seo/push",
				GroupKey: "plugin",
				Name:     "链接推送管理",
				Backend:  "/plugin/push",
			},
			{
				Path:     "/plugin/seo/sitemap",
				GroupKey: "plugin",
				Name:     "Sitemap管理",
				Backend:  "/plugin/sitemap",
			},
			{
				Path:     "/plugin/seo/robots",
				GroupKey: "plugin",
				Name:     "Robots管理",
				Backend:  "/plugin/robots",
			},
			{
				Path:     "/plugin/system/friendlink",
				GroupKey: "plugin",
				Name:     "友情链接管理",
				Backend:  "/plugin/link",
			},
			{
				Path:     "/plugin/interaction/comment",
				GroupKey: "plugin",
				Name:     "内容评论管理",
				Backend:  "/plugin/comment",
			},
			{
				Path:     "/plugin/contentops/anchor",
				GroupKey: "plugin",
				Name:     "锚文本管理",
				Backend:  "/plugin/anchor",
			},
			{
				Path:     "/plugin/interaction/guestbook",
				GroupKey: "plugin",
				Name:     "网站留言管理",
				Backend:  "/plugin/guestbook",
			},
			{
				Path:     "/plugin/contentops/keyword",
				GroupKey: "plugin",
				Name:     "关键词库管理",
				Backend:  "/plugin/keyword",
			},
			{
				Path:     "/plugin/contentops/material",
				GroupKey: "plugin",
				Name:     "内容素材管理",
				Backend:  "/plugin/material",
			},
			{
				Path:     "/plugin/system/fileupload",
				GroupKey: "plugin",
				Name:     "验证文件上传",
				Backend:  "/plugin/fileupload",
			},
			{
				Path:     "/plugin/channel/sendmail",
				GroupKey: "plugin",
				Name:     "邮件提醒",
				Backend:  "/plugin/sendmail",
			},
			{
				Path:     "/plugin/channel/subscription",
				GroupKey: "plugin",
				Name:     "邮件订阅",
				Backend:  "/plugin/subscription",
			},
			{
				Path:     "/plugin/contentops/collector",
				GroupKey: "plugin",
				Name:     "内容采集管理",
			},
			{
				Path:     "/plugin/contentops/importapi",
				GroupKey: "plugin",
				Name:     "内容导入接口",
				Backend:  "/plugin/import",
			},
			{
				Path:     "/plugin/system/redirect",
				GroupKey: "plugin",
				Name:     "301跳转管理",
				Backend:  "/plugin/redirect",
			},
			{
				Path:     "/plugin/contentops/transfer",
				GroupKey: "plugin",
				Name:     "网站内容迁移",
				Backend:  "/plugin/transfer",
			},
			{
				Path:     "/plugin/system/storage",
				GroupKey: "plugin",
				Name:     "资源存储配置",
				Backend:  "/plugin/storage",
			},
			{
				Path:     "/plugin/commerce/user",
				GroupKey: "plugin",
				Name:     "用户管理",
				Backend:  "/plugin/user",
			},
			{
				Path:     "/plugin/commerce/group",
				GroupKey: "plugin",
				Name:     "用户组VIP",
				Backend:  "/plugin/user/group",
			},
			{
				Path:     "/plugin/channel/wechat",
				GroupKey: "plugin",
				Name:     "微信公众号",
				Backend:  "/plugin/wechat",
			},
			{
				Path:     "/plugin/channel/weapp",
				GroupKey: "plugin",
				Name:     "小程序配置",
				Backend:  "/plugin/weapp",
			},
			{
				Path:     "/plugin/commerce/order",
				GroupKey: "plugin",
				Name:     "订单管理",
				Backend:  "/plugin/order",
			},
			{
				Path:     "/plugin/commerce/pay",
				GroupKey: "plugin",
				Name:     "支付配置",
				Backend:  "/plugin/pay",
			},
			{
				Path:     "/plugin/commerce/finance",
				GroupKey: "plugin",
				Name:     "财务管理",
				Backend:  "/plugin/finance",
			},
			{
				Path:     "/plugin/commerce/retailer",
				GroupKey: "plugin",
				Name:     "分销管理",
				Backend:  "/plugin/retailer",
			},
			{
				Path:     "/plugin/withdraw",
				GroupKey: "plugin",
				Name:     "提现管理",
				Backend:  "/plugin/withdraw",
			},
			{
				Path:     "/plugin/佣金管理",
				GroupKey: "plugin",
				Name:     "提现管理",
				Backend:  "/plugin/commission",
			},
			{
				Path:     "/plugin/system/fulltext",
				GroupKey: "plugin",
				Name:     "全文搜索",
				Backend:  "/plugin/fulltext",
			},
			{
				Path:     "/plugin/system/backup",
				GroupKey: "plugin",
				Name:     "备份与恢复",
				Backend:  "/plugin/backup",
			},
			{
				Path:     "/plugin/system/replace",
				GroupKey: "plugin",
				Name:     "全站替换",
				Backend:  "/plugin/replace",
			},
			{
				Path:     "/plugin/contentops/titleimage",
				GroupKey: "plugin",
				Name:     "标题自动配图",
				Backend:  "/plugin/titleimage",
			},
			{
				Path:     "/plugin/system/htmlcache",
				GroupKey: "plugin",
				Name:     "静态页面缓存",
				Backend:  "/plugin/htmlcache",
			},
			{
				Path:     "/plugin/contentops/aigenerate",
				GroupKey: "plugin",
				Name:     "AI自动写作",
				Backend:  "/plugin/aigenerate",
			},
			{
				Path:     "/plugin/contentops/timefactor",
				GroupKey: "plugin",
				Name:     "时间因子-定时发布",
				Backend:  "/plugin/timefactor",
			},
			{
				Path:     "/plugin/system/interference",
				GroupKey: "plugin",
				Name:     "防采集干扰码",
				Backend:  "/plugin/interference",
			},
			{
				Path:     "/plugin/system/watermark",
				GroupKey: "plugin",
				Name:     "水印管理",
				Backend:  "/plugin/watermark",
			},
			{
				Path:     "/plugin/system/limiter",
				GroupKey: "plugin",
				Name:     "限流器",
				Backend:  "/plugin/limiter",
			},
			{
				Path:     "/plugin/system/translate",
				GroupKey: "plugin",
				Name:     "翻译功能",
				Backend:  "/plugin/translate",
			},
			{
				Path:     "/plugin/seo/jsonld",
				GroupKey: "plugin",
				Name:     "结构化数据标记",
				Backend:  "/plugin/jsonld",
			},
			{
				Path:     "/plugin/channel/akismet",
				GroupKey: "plugin",
				Name:     "Akismet/reCAPTCHA",
				Backend:  "/plugin/akismet",
			},
			{
				Path:     "/plugin/channel/google",
				GroupKey: "plugin",
				Name:     "谷歌授权",
				Backend:  "/plugin/google",
			},
			{
				Path:     "/plugin/seo/llms",
				GroupKey: "plugin",
				Name:     "LLMs 配置",
				Backend:  "/plugin/llms",
			},
			{
				Path:     "/plugin/subscriber",
				GroupKey: "plugin",
				Name:     "邮件订阅",
				Backend:  "/plugin/subscriber",
			},
		},
	},
	{
		Key:  "design",
		Name: "模板设计",
		Menus: []*Menu{
			{
				Path:     "/design/index",
				GroupKey: "design",
				Name:     "我的模板",
				Backend:  "/design/list",
			},
			{
				Path:     "/design/editor",
				GroupKey: "design",
				Name:     "修改代码",
				Backend:  "/design/file",
			},
			{
				Path:     "/design/detail",
				GroupKey: "design",
				Name:     "模板管理",
				Backend:  "/design/info",
			},
			{
				Path:     "/design/doc",
				GroupKey: "design",
				Name:     "开发文档",
				Backend:  "/design/docs",
			},
			{
				Path:     "/design/market",
				GroupKey: "design",
				Name:     "设计市场",
				Backend:  "/design/market",
			},
		},
	},
	{
		Key:  "statistic",
		Name: "数据统计",
		Menus: []*Menu{
			{
				Path:     "/statistic/spider",
				GroupKey: "statistic",
				Name:     "蜘蛛统计",
			},
			{
				Path:     "/statistic/traffic",
				GroupKey: "statistic",
				Name:     "流量统计",
			},
			{
				Path:     "/statistic/detail",
				GroupKey: "statistic",
				Name:     "访问详细记录",
			},
			{
				Path:     "/statistic/includes",
				GroupKey: "statistic",
				Name:     "收录统计",
			},
			{
				Path:     "/statistic/include/detail",
				GroupKey: "statistic",
				Name:     "收录详细记录",
			},
		},
	},
	{
		Key:  "account",
		Name: "管理员",
		Menus: []*Menu{
			{
				Path:     "/account/list",
				GroupKey: "account",
				Name:     "管理员列表",
				Backend:  "/admin/list",
			},
			{
				Path:     "/account/group/list",
				GroupKey: "account",
				Name:     "管理员分组列表",
				Backend:  "/admin/group/list",
			},
			{
				Path:     "/account/group/detail",
				GroupKey: "account",
				Name:     "管理员分组信息",
				Backend:  "/admin/group/detail",
			},
			{
				Path:     "/account/logs/login",
				GroupKey: "account",
				Name:     "登录记录",
				Backend:  "/admin/logs/login",
			},
			{
				Path:     "/account/logs/action",
				GroupKey: "account",
				Name:     "操作记录",
				Backend:  "/admin/logs/action",
			},
		},
	},
	{
		Key:  "website",
		Name: "多站点",
		Menus: []*Menu{
			{
				Path:     "/website/list",
				GroupKey: "tool",
				Name:     "多站点管理",
				Backend:  "/website/list",
			},
			{
				Path:     "/website/multilang",
				GroupKey: "tool",
				Name:     "多语言站点",
				Backend:  "/plugin/multilang",
			},
		},
	},
	{
		Key:  "tool",
		Name: "系统功能",
		Menus: []*Menu{
			{
				Path:     "/tool/upgrade",
				GroupKey: "tool",
				Name:     "系统升级",
				Backend:  "/version/upgrade",
			},
			{
				Path:     "/tool/cache",
				GroupKey: "tool",
				Name:     "更新缓存",
				Backend:  "/setting/cache",
			},
		},
	},
}
