package library

import "strings"

type Language struct {
	Code   string `json:"code"`
	Baidu  string `json:"baidu"` // 百度翻译的语言代码
	Name   string `json:"name"`
	CnName string `json:"cnName"`
	Icon   string `json:"icon"`
}

var (
	// 语言列表
	Languages = []Language{
		{
			Code:   "en",
			Baidu:  "en",
			Name:   "English",
			CnName: "英语",
			Icon:   "🇺🇸",
		},
		{
			Code:   "zh-CN",
			Baidu:  "zh",
			Name:   "简体中文",
			CnName: "中文",
			Icon:   "🇨🇳",
		},
		{
			Code:   "zh-TW",
			Baidu:  "cht",
			Name:   "繁体中文",
			CnName: "繁体中文",
			Icon:   "🇨🇳",
		},
		{
			Code:   "vi",
			Baidu:  "vie",
			Name:   "Tiếng Việt",
			CnName: "越南语",
			Icon:   "🇻🇳",
		},
		{
			Code:   "id",
			Baidu:  "id",
			Name:   "Bahasa Indonesia",
			CnName: "印度尼西亚语",
			Icon:   "🇮🇩",
		},
		{
			Code:   "hi",
			Baidu:  "hi",
			Name:   "Hindi",
			CnName: "印地语",
			Icon:   "🇮🇳",
		},
		{
			Code:   "it",
			Baidu:  "it",
			Name:   "Italiano",
			CnName: "意大利语",
			Icon:   "🇮🇹",
		},
		{
			Code:   "el",
			Baidu:  "el",
			Name:   "Greek",
			CnName: "希腊语",
			Icon:   "🇬🇷",
		},
		{
			Code:   "es",
			Baidu:  "spa",
			Name:   "Español",
			CnName: "西班牙语",
			Icon:   "🇪🇸",
		},
		{
			Code:   "pt",
			Baidu:  "pt",
			Name:   "Português",
			CnName: "葡萄牙语",
			Icon:   "🇵🇹",
		},
		{
			Code:   "sr",
			Baidu:  "srp",
			Name:   "Srpski",
			CnName: "塞尔维亚语",
			Icon:   "🇷🇸",
		},
		{
			Code:   "my",
			Baidu:  "bur",
			Name:   "Burmese",
			CnName: "缅甸语",
			Icon:   "🇲🇲",
		},
		{
			Code:   "bn",
			Baidu:  "ben",
			Name:   "Bengali",
			CnName: "孟加拉语",
			Icon:   "🇧🇩",
		},
		{
			Code:   "th",
			Baidu:  "th",
			Name:   "Thai",
			CnName: "泰语",
			Icon:   "🇹🇭",
		},
		{
			Code:   "tr",
			Baidu:  "tr",
			Name:   "Türkçe",
			CnName: "土耳其语",
			Icon:   "🇹🇷",
		},
		{
			Code:   "ja",
			Baidu:  "jp",
			Name:   "Japanese",
			CnName: "日语",
			Icon:   "🇯🇵",
		},
		{
			Code:   "lo",
			Baidu:  "lao",
			Name:   "Lao",
			CnName: "老挝语",
			Icon:   "🇱🇦",
		},
		{
			Code:   "ko",
			Baidu:  "kor",
			Name:   "한국어",
			CnName: "韩语",
			Icon:   "🇰🇷",
		},
		{
			Code:   "ru",
			Baidu:  "ru",
			Name:   "Русский",
			CnName: "俄语",
			Icon:   "🇷🇺",
		},
		{
			Code:   "fr",
			Baidu:  "fra",
			Name:   "Français",
			CnName: "法语",
			Icon:   "🇫🇷",
		},
		{
			Code:   "de",
			Baidu:  "de",
			Name:   "Deutsch",
			CnName: "德语",
			Icon:   "🇩🇪",
		},
		{
			Code:   "fa",
			Baidu:  "per",
			Name:   "فارسی",
			CnName: "波斯语",
			Icon:   "🇮🇷",
		},
		{
			Code:   "ar",
			Baidu:  "ara",
			Name:   "العربية",
			CnName: "阿拉伯语",
			Icon:   "🇸🇦",
		},
		{
			Code:   "ms",
			Baidu:  "",
			Name:   "Bahasa Melayu",
			CnName: "马来语",
			Icon:   "🇲🇾",
		},
		{
			Code:   "jw",
			Baidu:  "jav",
			Name:   "Jawa",
			CnName: "爪哇语",
			Icon:   "🇮🇩",
		},
		{
			Code:   "te",
			Baidu:  "tel",
			Name:   "Telugu",
			CnName: "泰卢固语",
			Icon:   "🇮🇳",
		},
		{
			Code:   "ta",
			Baidu:  "tam",
			Name:   "Tamil",
			CnName: "泰米尔语",
			Icon:   "🇮🇳",
		},
		{
			Code:   "mr",
			Baidu:  "mar",
			Name:   "Marathi",
			CnName: "马拉地语",
			Icon:   "🇮🇳",
		},
		{
			Code:   "ur",
			Baidu:  "urd",
			Name:   "Urdu",
			CnName: "乌尔都语",
			Icon:   "🇵🇰",
		},
		{
			Code:   "pl",
			Baidu:  "pl",
			Name:   "Polski",
			CnName: "波兰语",
			Icon:   "🇵🇱",
		},
		{
			Code:   "uk",
			Baidu:  "ukr",
			Name:   "Українська",
			CnName: "乌克兰语",
			Icon:   "🇺🇦",
		},
		{
			Code:   "pa",
			Baidu:  "pan",
			Name:   "Panjabi",
			CnName: "旁遮普语",
			Icon:   "🇮🇳",
		},
		{
			Code:   "ro",
			Baidu:  "rom",
			Name:   "Română",
			CnName: "罗马尼亚语",
			Icon:   "🇷🇴",
		},
		{
			Code:   "et",
			Baidu:  "est",
			Name:   "Eesti",
			CnName: "爱沙尼亚语",
			Icon:   "🇪🇪",
		},
		{
			Code:   "os",
			Baidu:  "oss",
			Name:   "Ossetic",
			CnName: "奥塞梯语",
			Icon:   "🇷🇺",
		},
		{
			Code:   "be",
			Baidu:  "bel",
			Name:   "Беларуская",
			CnName: "白俄罗斯语",
			Icon:   "🇧🇾",
		},
		{
			Code:   "bg",
			Baidu:  "bul",
			Name:   "Български",
			CnName: "保加利亚语",
			Icon:   "🇧🇬",
		},
		{
			Code:   "is",
			Baidu:  "ice",
			Name:   "Icelandic",
			CnName: "冰岛语",
			Icon:   "🇮🇸",
		},
		{
			Code:   "bs",
			Baidu:  "bos",
			Name:   "Bosnian",
			CnName: "波斯尼亚语",
			Icon:   "🇧🇦",
		},
		{
			Code:   "bo",
			Baidu:  "tib",
			Name:   "Tibetan",
			CnName: "藏语",
			Icon:   "🇨🇳",
		},
		{
			Code:   "da",
			Baidu:  "dan",
			Name:   "Dansk",
			CnName: "丹麦语",
			Icon:   "🇩🇰",
		},
		{
			Code:   "tl",
			Baidu:  "tgl",
			Name:   "Filipino",
			CnName: "菲律宾语",
			Icon:   "🇵🇭",
		},
		{
			Code:   "fi",
			Baidu:  "fin",
			Name:   "Suomi",
			CnName: "芬兰语",
			Icon:   "🇫🇮",
		},
		{
			Code:   "sv",
			Baidu:  "swe",
			Name:   "Swedish",
			CnName: "瑞典语",
			Icon:   "🇸🇪",
		},
		{
			Code:   "kg",
			Name:   "Kongo",
			CnName: "刚果语",
			Icon:   "🇨🇬",
		},
		{
			Code:   "ka",
			Baidu:  "geo",
			Name:   "Georgian",
			CnName: "格鲁吉亚语",
			Icon:   "🇬🇪",
		},
		{
			Code:   "kk",
			Baidu:  "kaz",
			Name:   "Kazakh",
			CnName: "哈萨克语",
			Icon:   "🇰🇿",
		},
		{
			Code:   "gl",
			Baidu:  "glg",
			Name:   "Galician",
			CnName: "加利西亚语",
			Icon:   "🇪🇸",
		},
		{
			Code:   "ky",
			Baidu:  "kir",
			Name:   "Kyrgyz",
			CnName: "吉尔吉斯语",
			Icon:   "🇰🇬",
		},
		{
			Code:   "nl",
			Baidu:  "nl",
			Name:   "Nederlands",
			CnName: "荷兰语",
			Icon:   "🇳🇱",
		},
		{
			Code:   "ca",
			Baidu:  "cat",
			Name:   "Catalan",
			CnName: "加泰罗尼亚语",
			Icon:   "🇪🇸",
		},
		{
			Code:   "cs",
			Baidu:  "cs",
			Name:   "Čeština",
			CnName: "捷克语",
			Icon:   "🇨🇿",
		},
		{
			Code:   "kn",
			Baidu:  "kan",
			Name:   "Kannada",
			CnName: "卡纳达语",
			Icon:   "🇮🇳",
		},
		{
			Code:   "mn",
			Baidu:  "mon",
			Name:   "Mongolian",
			CnName: "蒙古语",
			Icon:   "🇲🇳",
		},
		{
			Code:   "hr",
			Baidu:  "hrv",
			Name:   "Hrvatski",
			CnName: "克罗地亚语",
			Icon:   "🇭🇷",
		},
		{
			Code:   "lv",
			Baidu:  "lav",
			Name:   "Latvian",
			CnName: "拉脱维亚语",
			Icon:   "🇱🇻",
		},
		{
			Code:   "lt",
			Baidu:  "lit",
			Name:   "Lettish",
			CnName: "拉脱维亚语",
			Icon:   "🇱🇹",
		},
		{
			Code:   "no",
			Baidu:  "nor",
			Name:   "Norwegian",
			CnName: "挪威语",
			Icon:   "🇳🇴",
		},
		{
			Code:   "ne",
			Baidu:  "nep",
			Name:   "Nepali",
			CnName: "尼泊尔语",
			Icon:   "🇳🇵",
		},
		{
			Code:   "ps",
			Baidu:  "pus",
			Name:   "Pashto",
			CnName: "普什图语",
			Icon:   "🇦🇫",
		},
		{
			Code:   "ks",
			Name:   "Slovak",
			CnName: "斯洛伐克语",
			Icon:   "🇸🇰",
		},
		{
			Code:   "tk",
			Baidu:  "tuk",
			Name:   "Turkmen",
			CnName: "土库曼语",
			Icon:   "🇹🇲",
		},
		{
			Code:   "uz",
			Baidu:  "uzb",
			Name:   "Uzbek",
			CnName: "乌兹别克语",
			Icon:   "🇺🇿",
		},
		{
			Code:   "iw",
			Baidu:  "heb",
			Name:   "Hebrew",
			CnName: "希伯来语",
			Icon:   "🇮🇱",
		},
		{
			Code:   "hu",
			Baidu:  "hu",
			Name:   "Hungarian",
			CnName: "匈牙利语",
			Icon:   "🇭🇺",
		},
		{
			Code:   "hy",
			Baidu:  "arm",
			Name:   "Armenian",
			CnName: "亚美尼亚语",
			Icon:   "🇦🇲",
		},
		{
			Code:   "ht",
			Baidu:  "ht",
			Name:   "Kreyòl Ayisyen",
			CnName: "海地克里奥尔语",
			Icon:   "🇭🇹",
		},
		{
			Code:   "mg",
			Baidu:  "mg",
			Name:   "Malagasy",
			CnName: "毛里求斯克语",
			Icon:   "🇲🇬",
		},
		{
			Code:   "mk",
			Baidu:  "mac",
			Name:   "Македонски",
			CnName: "马其顿语",
			Icon:   "🇲🇰",
		},
		{
			Code:   "ml",
			Baidu:  "mal",
			Name:   "മലയാളം",
			CnName: "马拉雅拉姆语",
			Icon:   "🇮🇳",
		},
		{
			Code:   "af",
			Baidu:  "afr",
			Name:   "Afrikaans",
			CnName: "南非荷兰语",
			Icon:   "🇿🇦",
		},
		{
			Code:   "mt",
			Baidu:  "mlt",
			Name:   "Malti",
			CnName: "马耳他语",
			Icon:   "🇲🇹",
		},
		{
			Code:   "eu",
			Baidu:  "baq",
			Name:   "Euskara",
			CnName: "巴斯克语",
			Icon:   "🇪🇸",
		},
		{
			Code:   "pt-PT",
			Baidu:  "pt",
			Name:   "Português-PT",
			CnName: "葡萄牙语-PT",
			Icon:   "🇵🇹",
		},
		{
			Code:   "az",
			Baidu:  "aze",
			Name:   "Azərbaycan",
			CnName: "阿塞拜疆语",
			Icon:   "🇦🇿",
		},
		{
			Code:   "en-GB",
			Baidu:  "en",
			Name:   "英国English",
			CnName: "英国英语",
			Icon:   "🇬🇧",
		},
		{
			Code:   "sd",
			Baidu:  "snd",
			Name:   "سنڌي",
			CnName: "斯南地语",
			Icon:   "🇵🇰",
		},
		{
			Code:   "se",
			Name:   "Davvisámegiella",
			CnName: "斯瓦西里语",
			Icon:   "🇳🇴",
		},
		{
			Code:   "si",
			Baidu:  "sin",
			Name:   "සිංහල",
			CnName: "僧伽罗语",
			Icon:   "🇱🇰",
		},
		{
			Code:   "sk",
			Baidu:  "sk",
			Name:   "Slovenčina",
			CnName: "斯洛文尼亚语",
			Icon:   "🇸🇰",
		},
		{
			Code:   "sl",
			Baidu:  "slo",
			Name:   "Slovenščina",
			CnName: "斯洛文尼亚语",
			Icon:   "🇸🇮",
		},
		{
			Code:   "ga",
			Baidu:  "gle",
			Name:   "Gaeilge",
			CnName: "爱尔兰语",
			Icon:   "🇮🇪",
		},
		{
			Code:   "sn",
			Name:   "Shona",
			CnName: "斯瓦西里语",
			Icon:   "🇿🇼",
		},
		{
			Code:   "so",
			Baidu:  "som",
			Name:   "Soomaali",
			CnName: "索马里语",
			Icon:   "🇸🇴",
		},
		{
			Code:   "gd",
			Name:   "Gàidhlig",
			CnName: "苏格兰语",
			Icon:   "🇬🇧",
		},
		{
			Code:   "sq",
			Baidu:  "alb",
			Name:   "Shqip",
			CnName: "阿尔巴尼亚语",
			Icon:   "🇦🇱",
		},
		{
			Code:   "st",
			Name:   "Sesotho",
			CnName: "塞索托语",
			Icon:   "🇱🇸",
		},
		{
			Code:   "km",
			Baidu:  "hkm",
			Name:   "ភាសាខ្មែរ",
			CnName: "高棉语",
			Icon:   "🇰🇭",
		},
		{
			Code:   "sw",
			Baidu:  "swa",
			Name:   "Kiswahili",
			CnName: "斯瓦西里语",
			Icon:   "🇹🇿",
		},
		{
			Code:   "pt-BR",
			Baidu:  "pt",
			Name:   "Português-BR",
			CnName: "葡萄牙语-BR",
			Icon:   "🇧🇷",
		},
		{
			Code:   "co",
			Name:   "Corsu",
			CnName: "科西嘉语",
			Icon:   "🇫🇷",
		},
		{
			Code:   "gu",
			Baidu:  "guj",
			Name:   "ગુજરાતી",
			CnName: "古吉拉特语",
			Icon:   "🇮🇳",
		},
		{
			Code:   "tg",
			Baidu:  "tgk",
			Name:   "Тоҷикӣ",
			CnName: "塔吉克语",
			Icon:   "🇹🇯",
		},
		{
			Code:   "la",
			Baidu:  "lat",
			Name:   "Latina",
			CnName: "拉丁语",
			Icon:   "🇻🇦",
		},
		{
			Code:  "cy",
			Baidu: "wel",
			Name:  "Cymraeg",

			Icon: "🇬🇧",
		},
	}
)

func GetLanguageCnName(lang string) string {
	if strings.Contains(lang, "-") {
		langs := strings.Split(lang, "-")
		langs[1] = strings.ToUpper(langs[1])
		lang = strings.Join(langs, "-")
	}
	name := lang
	for i := range Languages {
		if Languages[i].Code == lang {
			name = Languages[i].CnName
			break
		}
	}

	return name
}

func GetLanguageName(lang string) string {
	if strings.Contains(lang, "-") {
		langs := strings.Split(lang, "-")
		langs[1] = strings.ToUpper(langs[1])
		lang = strings.Join(langs, "-")
	}
	name := lang
	for i := range Languages {
		if Languages[i].Code == lang {
			name = Languages[i].Name
			break
		}
	}

	return name
}

func GetLanguageIcon(lang string) string {
	if strings.Contains(lang, "-") {
		langs := strings.Split(lang, "-")
		langs[1] = strings.ToUpper(langs[1])
		lang = strings.Join(langs, "-")
	}
	var icon string
	for i := range Languages {
		if Languages[i].Code == lang {
			icon = Languages[i].Icon
			break
		}
	}

	return icon
}

func GetLanguageBaiduCode(lang string) string {
	if strings.Contains(lang, "-") {
		langs := strings.Split(lang, "-")
		langs[1] = strings.ToUpper(langs[1])
		lang = strings.Join(langs, "-")
	}
	var baiduCode = "auto"
	for i := range Languages {
		if Languages[i].Code == lang {
			baiduCode = Languages[i].Baidu
			break
		}
	}

	return baiduCode
}
