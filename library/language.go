package library

func GetLanguageName(lang string) string {
	name := lang
	switch lang {
	case "en":
		// 英语
		name = "English"
	case "zh-CN":
		// 简体中文
		name = "简体中文"
	case "zh-TW":
		// 繁体中文
		name = "繁体中文"
	case "vi":
		// 越南语
		name = "Tiếng Việt"
	case "id":
		// 印尼语
		name = "Bahasa Indonesia"
	case "hi":
		// 印地语
		name = "Hindi"
	case "it":
		// 意大利语
		name = "Italiano"
	case "el":
		// 希腊语
		name = "Greek"
	case "es":
		// 西班牙语
		name = "Español"
	case "pt":
		// 葡萄牙语
		name = "Português"
	case "sr":
		// 塞尔维亚语
		name = "Srpski"
	case "my":
		// 缅甸语
		name = "Burmese"
	case "bn":
		// 孟加拉语
		name = "Bengali"
	case "th":
		// 泰语
		name = "Thai"
	case "tr":
		// 土耳其语
		name = "Türkçe"
	case "ja":
		// 日语
		name = "Japanese"
	case "lo":
		// 老挝语
		name = "Lao"
	case "ko":
		// 韩语
		name = "한국어"
	case "ru":
		// 俄语
		name = "Русский"
	case "fr":
		// 法语
		name = "Français"
	case "de":
		// 德语
		name = "Deutsch"
	case "fa":
		// 波斯语
		name = "فارسی"
	case "ar":
		// 阿拉伯语
		name = "العربية"
	case "ms":
		// 马来语
		name = "Bahasa Melayu"
	case "jw":
		// 爪哇语
		name = "Jawa"
	case "te":
		// 泰卢固语
		name = "Telugu"
	case "ta":
		// 泰米尔语
		name = "Tamil"
	case "mr":
		// 马拉地语
		name = "Marathi"
	case "ur":
		// 乌尔都语
		name = "Urdu"
	case "pl":
		// 波兰语
		name = "Polski"
	case "uk":
		// 乌克兰语
		name = "Українська"
	case "pa":
		// 旁遮普语
		name = "Panjabi"
	case "ro":
		// 罗马尼亚语
		name = "Română"
	case "et":
		// 爱沙尼亚语
		name = "Eesti"
	case "os":
		// 奥塞梯语
		name = "Ossetic"
	case "be":
		// 白俄罗斯语
		name = "Беларуская"
	case "bg":
		// 保加利亚语
		name = "Български"
	case "is":
		// 冰岛语
		name = "Icelandic"
	case "bs":
		// 波斯尼亚语
		name = "Bosnian"
	case "bo":
		// 藏语
		name = "Tibetan"
	case "da":
		// 丹麦语
		name = "Dansk"
	case "tl":
		// 菲律宾语
		name = "Filipino"
	case "fi":
		// 芬兰语
		name = "Suomi"
	case "sv":
		// 瑞典语
		name = "Swedish"
	case "kg":
		// 刚果语
		name = "Kongo"
	case "ka":
		// 格鲁吉亚语
		name = "Georgian"
	case "kk":
		// 哈萨克语
		name = "Kazakh"
	case "gl":
		// 加利西亚语
		name = "Galician"
	case "ky":
		// 吉尔吉斯语
		name = "Kyrgyz"
	case "nl":
		// 荷兰语
		name = "Nederlands"
	case "ca":
		// 加泰罗尼亚语
		name = "Catalan"
	case "cs":
		// 捷克语
		name = "Čeština"
	case "kn":
		// 卡纳达语
		name = "Kannada"
	case "mn":
		// 蒙古语
		name = "Mongolian"
	case "hr":
		// 克罗地亚语
		name = "Hrvatski"
	case "lv":
		// 拉脱维亚语
		name = "Latvian"
	case "lt":
		// 立陶宛语
		name = "Lettish"
	case "no":
		// 挪威语
		name = "Norwegian"
	case "ne":
		// 尼泊尔语
		name = "Nepali"
	case "ps":
		// 普什图语
		name = "Pashto"
	case "ks":
		// 斯洛伐克语
		name = "Slovak"
	case "tk":
		// 土库曼语
		name = "Turkmen"
	case "uz":
		// 乌兹别克语
		name = "Uzbek"
	case "iw":
		// 希伯来语
		name = "Hebrew"
	case "hu":
		// 匈牙利语
		name = "Hungarian"
	case "hy":
		// 亚美尼亚语
		name = "Armenian"
	case "ht":
		// 海地克里奥尔语
		name = "Kreyòl Ayisyen"
	case "mg":
		// 马尔加什语
		name = "Malagasy"
	case "mk":
		// 马其顿语
		name = "Македонски"
	case "ml":
		// 马拉雅拉姆语
		name = "മലയാളം"
	case "af":
		// 南非荷兰语
		name = "Afrikaans"
	case "mt":
		// 马耳他语
		name = "Malti"
	case "eu":
		// 巴斯克语
		name = "Euskara"
	case "pt-PT":
		// 葡萄牙语
		name = "Português-PT"
	case "az":
		// 阿塞拜疆语
		name = "Azərbaycan"
	case "en-GB":
		// 英语
		name = "英国English"
	case "sd":
		// 信德语
		name = "سنڌي"
	case "se":
		// 北萨米语
		name = "Davvisámegiella"
	case "si":
		// 僧伽罗语
		name = "සිංහල"
	case "sk":
		// 斯洛伐克语
		name = "Slovenčina"
	case "sl":
		// 斯洛文尼亚语
		name = "Slovenščina"
	case "ga":
		// 爱尔兰语
		name = "Gaeilge"
	case "sn":
		// 修纳语
		name = "Shona"
	case "so":
		// 索马里语
		name = "Soomaali"
	case "gd":
		// 苏格兰盖尔语
		name = "Gàidhlig"
	case "sq":
		// 阿尔巴尼亚语
		name = "Shqip"
	case "st":
		// 塞索托语
		name = "Sesotho"
	case "km":
		// 高棉语
		name = "ភាសាខ្មែរ"
	case "sw":
		// 斯瓦希里语
		name = "Kiswahili"
	case "pt-BR":
		// 葡萄牙语
		name = "Português-BR"
	case "co":
		// 科西嘉语
		name = "Corsu"
	case "gu":
		// 古吉拉特语
		name = "ગુજરાતી"
	case "tg":
		// 塔吉克语
		name = "Тоҷикӣ"
	case "la":
		// 拉丁语
		name = "Latina"
	case "cy":
		// 威尔士语
		name = "Cymraeg"
	}

	return name
}

func GetLanguageIcon(lang string) string {
	switch lang {
	case "en":
		return "🇺🇸" // 美国（英语主要地区）
	case "zh-CN":
		return "🇨🇳" // 中国（简体中文）
	case "zh-TW":
		return "🇨🇳" // 台湾（繁体中文）
	case "vi":
		return "🇻🇳" // 越南
	case "id":
		return "🇮🇩" // 印度尼西亚
	case "hi":
		return "🇮🇳" // 印度（印地语）
	case "it":
		return "🇮🇹" // 意大利
	case "el":
		return "🇬🇷" // 希腊
	case "es":
		return "🇪🇸" // 西班牙
	case "pt":
		return "🇵🇹" // 葡萄牙
	case "sr":
		return "🇷🇸" // 塞尔维亚
	case "my":
		return "🇲🇲" // 缅甸
	case "bn":
		return "🇧🇩" // 孟加拉国
	case "th":
		return "🇹🇭" // 泰国
	case "tr":
		return "🇹🇷" // 土耳其
	case "ja":
		return "🇯🇵" // 日本
	case "lo":
		return "🇱🇦" // 老挝
	case "ko":
		return "🇰🇷" // 韩国
	case "ru":
		return "🇷🇺" // 俄罗斯
	case "fr":
		return "🇫🇷" // 法国
	case "de":
		return "🇩🇪" // 德国
	case "fa":
		return "🇮🇷" // 伊朗（波斯语）
	case "ar":
		return "🇸🇦" // 沙特阿拉伯（阿拉伯语）
	case "ms":
		return "🇲🇾" // 马来西亚
	case "jw":
		return "🇮🇩" // 印尼（爪哇语）
	case "te":
		return "🇮🇳" // 印度（泰卢固语）
	case "ta":
		return "🇮🇳" // 印度（泰米尔语）
	case "mr":
		return "🇮🇳" // 印度（马拉地语）
	case "ur":
		return "🇵🇰" // 巴基斯坦（乌尔都语）
	case "pl":
		return "🇵🇱" // 波兰
	case "uk":
		return "🇺🇦" // 乌克兰
	case "pa":
		return "🇮🇳" // 印度（旁遮普语）
	case "ro":
		return "🇷🇴" // 罗马尼亚
	case "et":
		return "🇪🇪" // 爱沙尼亚
	case "os":
		return "🇷🇺" // 俄罗斯（奥塞梯语）
	case "be":
		return "🇧🇾" // 白俄罗斯
	case "bg":
		return "🇧🇬" // 保加利亚
	case "is":
		return "🇮🇸" // 冰岛
	case "bs":
		return "🇧🇦" // 波斯尼亚和黑塞哥维那
	case "bo":
		return "🇨🇳" // 中国（藏语）
	case "da":
		return "🇩🇰" // 丹麦
	case "tl":
		return "🇵🇭" // 菲律宾
	case "fi":
		return "🇫🇮" // 芬兰
	case "sv":
		return "🇸🇪" // 瑞典
	case "kg":
		return "🇨🇬" // 刚果
	case "ka":
		return "🇬🇪" // 格鲁吉亚
	case "kk":
		return "🇰🇿" // 哈萨克斯坦
	case "gl":
		return "🇪🇸" // 西班牙（加利西亚语）
	case "ky":
		return "🇰🇬" // 吉尔吉斯斯坦
	case "nl":
		return "🇳🇱" // 荷兰
	case "ca":
		return "🇪🇸" // 西班牙（加泰罗尼亚语）
	case "cs":
		return "🇨🇿" // 捷克
	case "kn":
		return "🇮🇳" // 印度（卡纳达语）
	case "mn":
		return "🇲🇳" // 蒙古
	case "hr":
		return "🇭🇷" // 克罗地亚
	case "lv":
		return "🇱🇻" // 拉脱维亚
	case "lt":
		return "🇱🇹" // 立陶宛
	case "no":
		return "🇳🇴" // 挪威
	case "ne":
		return "🇳🇵" // 尼泊尔
	case "ps":
		return "🇦🇫" // 阿富汗（普什图语）
	case "ks":
		return "🇸🇰" // 斯洛伐克
	case "tk":
		return "🇹🇲" // 土库曼斯坦
	case "uz":
		return "🇺🇿" // 乌兹别克斯坦
	case "iw":
		return "🇮🇱" // 以色列（希伯来语）
	case "hu":
		return "🇭🇺" // 匈牙利
	case "hy":
		return "🇦🇲" // 亚美尼亚
	case "ht":
		return "🇭🇹"
	case "mg":
		return "🇲🇬"
	case "mk":
		return "🇲🇰"
	case "ml":
		return "🇮🇳"
	case "af":
		return "🇿🇦"
	case "mt":
		return "🇲🇹"
	case "eu":
		return "🇪🇸"
	case "pt-PT":
		return "🇵🇹"
	case "az":
		return "🇦🇿"
	case "en-GB":
		return "🇬🇧"
	case "sd":
		return "🇵🇰"
	case "se":
		return "🇳🇴"
	case "si":
		return "🇱🇰"
	case "sk":
		return "🇸🇰"
	case "sl":
		return "🇸🇮"
	case "ga":
		return "🇮🇪"
	case "sn":
		return "🇿🇼"
	case "so":
		return "🇸🇴"
	case "gd":
		return "🇬🇧"
	case "sq":
		return "🇦🇱"
	case "st":
		return "🇱🇸"
	case "km":
		return "🇰🇭"
	case "sw":
		return "🇹🇿"
	case "pt-BR":
		return "🇧🇷"
	case "co":
		return "🇫🇷"
	case "gu":
		return "🇮🇳"
	case "tg":
		return "🇹🇯"
	case "la":
		return "🇻🇦"
	case "cy":
		return "🇬🇧"
	default:
		return "🏳️" // 默认返回未知旗帜
	}
}
