package config

type CollectorJson struct {
	ErrorTimes         int      `json:"error_times"` //预留
	Channels           int      `json:"channels"`    //预留
	TitleMinLength     int      `json:"title_min_length"`
	ContentMinLength   int      `json:"content_min_length"`
	TitleExclude       []string `json:"title_exclude"`
	TitleExcludePrefix []string `json:"title_exclude_prefix"`
	TitleExcludeSuffix []string `json:"title_exclude_suffix"`
	ContentExcludeLine []string `json:"content_exclude_line"`
	LinkExclude        []string `json:"link_exclude"`
	ContentReplace     []string `json:"content_replace"`
	AutoPseudo         bool     `json:"auto_pseudo"`      //是否伪原创
	AutoDigKeyword     bool     `json:"auto_dig_keyword"` //关键词是否自动拓词
	CategoryId         uint     `json:"category_id"`      //默认分类
	StartHour          int      `json:"start_hour"`       //每天开始时间
	EndHour            int      `json:"end_hour"`         //每天结束时间
	DailyLimit         int      `json:"daily_limit"`      //每日限额
}

var defaultCollectorConfig = CollectorJson{
	ErrorTimes:       5,
	Channels:         2,
	TitleMinLength:   10,
	ContentMinLength: 400,
	AutoPseudo:       false,
	AutoDigKeyword:   false,
	CategoryId:       0,
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
		"本文地址：",
		"本文地址:",
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
	},
	LinkExclude: []string{
		"查看更多",
	},
}
