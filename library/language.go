package library

func GetLanguageName(lang string) string {
	name := lang
	switch lang {
	case "en":
		// 英语
		name = "English"
	case "zh-cn":
		// 简体中文
		name = "简体中文"
	case "zh-tw":
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
	}

	return name
}
