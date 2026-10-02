package config

type CollectorJson struct {
	IsRunning          bool             `json:"-"`
	AutoCollect        bool             `json:"auto_collect"`         // 是否自动采集
	ErrorTimes         int              `json:"error_times"`          // 最大错误重试次数
	Channels           int              `json:"channels"`             // 采集通道数
	CollectMode        int              `json:"collect_mode"`         // 采集模式： 0 文章采集，1 问答组合，2 AI 生成
	Language           string           `json:"language"`             // 采集语言：zh|en
	InsertImage        int              `json:"insert_image"`         // 是否插入图片： 0 移除图片，1 保留图片，2 插入自定义图片，3 图片库分类
	Images             []string         `json:"images"`               // 自定义图片地址
	ImageCategoryId    int              `json:"image_category_id"`    // 选定的图片分类
	FromWebsite        string           `json:"from_website"`         // 指定采集网站（默认为空）
	TitleMinLength     int              `json:"title_min_length"`     // 标题最小长度
	ContentMinLength   int              `json:"content_min_length"`   // 内容最小长度
	TitleExclude       []string         `json:"title_exclude"`        // 标题关键词排除
	TitleExcludePrefix []string         `json:"title_exclude_prefix"` // 标题前缀排除
	TitleExcludeSuffix []string         `json:"title_exclude_suffix"` // 标题后缀排除
	ContentExcludeLine []string         `json:"content_exclude_line"` // 内容行关键词排除
	ContentExclude     []string         `json:"content_exclude"`      // 内容关键词排除
	LinkExclude        []string         `json:"link_exclude"`         // 链接关键词排除
	ContentReplace     []ReplaceKeyword `json:"content_replace"`      // 内容关键词替换
	AutoPseudo         bool             `json:"auto_pseudo"`          // 是否进行伪原创
	AutoTranslate      bool             `json:"auto_translate"`       // 是否进行翻译
	ToLanguage         string           `json:"to_language"`          // 翻译目标语言，支持谷歌翻译列表语言
	CategoryId         uint             `json:"category_id"`          // 默认文档分类
	CategoryIds        []uint           `json:"category_ids"`         // 默认分类，支持多个，和 category_id 二选一
	SaveType           uint             `json:"save_type"`            // 文档处理方式：0 草稿，1 正式发布
	StartHour          int              `json:"start_hour"`           // 每天开始时间（时）：0-23
	EndHour            int              `json:"end_hour"`             // 每天结束时间（时）：0-23
	DailyLimit         int              `json:"daily_limit"`          // 每日数量
	CustomPatten       []*CustomPatten  `json:"custom_patten"`        // 自定义采集匹配规则
	ProxyConfig        ProxyConfig      `json:"proxy_config"`         // 代理配置
}

type ReplaceKeyword struct {
	From string `json:"from"` // 待替换的词
	To   string `json:"to"`   // 替换的目标词
}

type CustomPatten struct {
	Domain         string           `json:"domain"`          // 匹配的域名
	TitlePatten    string           `json:"title_patten"`    // 标题匹配规则
	ContentPatten  string           `json:"content_patten"`  // 内容匹配规则
	TitleReplace   []ReplaceKeyword `json:"title_replace"`   // 标题关键词替换
	ContentReplace []ReplaceKeyword `json:"content_replace"` // 内容关键词替换
}

type ProxyConfig struct {
	Open       bool   `json:"open"`       // 是否使用代理
	Platform   string `json:"platform"`   // 提供IP的平台 默认为 juliangip
	ApiUrl     string `json:"api_url"`    // 请求地址
	Concurrent int    `json:"concurrent"` // 并发数量
	Expire     int    `json:"expire"`     // 过期时间，单位秒，填写了，过期时间程序能提前释放IP，提高效率
}

var DefaultCollectorConfig = CollectorJson{
	AutoCollect:      false,
	ErrorTimes:       5,
	Channels:         2,
	TitleMinLength:   10,
	ContentMinLength: 400,
	AutoPseudo:       false,
	CategoryId:       0,
	SaveType:         0,
	StartHour:        8,
	EndHour:          20,
	DailyLimit:       1000,
	TitleExclude: []string{
		"法律声明",
		"站点地图",
		"区长信箱",
		"政务服务",
		"政务公开",
		"领导介绍",
		"首页",
		"当前页",
		"当前位置",
		"来源：",
		"点击：",
		"关注我们",
		"浏览次数",
		"信息分类",
		"索引号",
	},
	TitleExcludePrefix: []string{
		"404",
		"403",
	},
	TitleExcludeSuffix: []string{
		"网",
		"政府",
		"门户",
	},
	ContentExcludeLine: []string{
		"背景色：",
		"时间：",
		"作者：",
		"来源：",
		"编辑：",
		"时间:",
		"来源:",
		"作者:",
		"编辑:",
		"摄影：",
		"摄影:",
		"本文地址",
		"原文地址",
		"微信：",
		"微信:",
		"官方微信",
		"一篇：",
		"相关附件",
		"qrcode",
		"微信扫一扫",
		"用手机浏览",
		"打印正文",
		"浏览次数",
		"举报/反馈",
		"展开全文",
		"资料来源/",
		"编辑/",
		"文/",
		"关注央视网",
		"©",
		"（记者",
		"相关文章",
		"相关推荐",
		"原作者所有",
		"专题推荐",
		"随机推荐",
		"了解详情",
		"了解更多",
		"查看更多",
		"来源网络",
		"转载请",
	},
	LinkExclude: []string{
		"查看更多",
	},
	CustomPatten: []*CustomPatten{
		{
			Domain:        "mp.weixin.qq.com",
			TitlePatten:   "h1",
			ContentPatten: "#js_content",
		},
		{
			Domain:        "zhihu.com",
			TitlePatten:   "h1",
			ContentPatten: ".RichContent-inner .RichText,.Post-RichTextContainer .RichText",
		},
		{
			Domain:        "toutiao.com",
			TitlePatten:   "h1",
			ContentPatten: ".article-content article",
		},
	},
}
