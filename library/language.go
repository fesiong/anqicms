package library

import "strings"

type Language struct {
	Code  string `json:"code"`
	Baidu string `json:"baidu"` // 百度翻译的语言代码
	Name  string `json:"name"`
	Icon  string `json:"icon"`
}

var (
	// 语言列表
	Languages = []Language{
		{
			Code:  "en",
			Baidu: "en",
			Name:  "English",
			Icon:  "🇺🇸",
		},
		{
			Code:  "zh-CN",
			Baidu: "zh",
			Name:  "简体中文",
			Icon:  "🇨🇳",
		},
		{
			Code:  "zh-TW",
			Baidu: "cht",
			Name:  "繁体中文",
			Icon:  "🇨🇳",
		},
		{
			Code:  "vi",
			Baidu: "vie",
			Name:  "Tiếng Việt",
			Icon:  "🇻🇳",
		},
		{
			Code:  "id",
			Baidu: "id",
			Name:  "Bahasa Indonesia",
			Icon:  "🇮🇩",
		},
		{
			Code:  "hi",
			Baidu: "hi",
			Name:  "Hindi",
			Icon:  "🇮🇳",
		},
		{
			Code:  "it",
			Baidu: "it",
			Name:  "Italiano",
			Icon:  "🇮🇹",
		},
		{
			Code:  "el",
			Baidu: "el",
			Name:  "Greek",
			Icon:  "🇬🇷",
		},
		{
			Code:  "es",
			Baidu: "spa",
			Name:  "Español",
			Icon:  "🇪🇸",
		},
		{
			Code:  "pt",
			Baidu: "pt",
			Name:  "Português",
			Icon:  "🇵🇹",
		},
		{
			Code:  "sr",
			Baidu: "srp",
			Name:  "Srpski",
			Icon:  "🇷🇸",
		},
		{
			Code:  "my",
			Baidu: "bur",
			Name:  "Burmese",
			Icon:  "🇲🇲",
		},
		{
			Code:  "bn",
			Baidu: "ben",
			Name:  "Bengali",
			Icon:  "🇧🇩",
		},
		{
			Code:  "th",
			Baidu: "th",
			Name:  "Thai",
			Icon:  "🇹🇭",
		},
		{
			Code:  "tr",
			Baidu: "tr",
			Name:  "Türkçe",
			Icon:  "🇹🇷",
		},
		{
			Code:  "ja",
			Baidu: "jp",
			Name:  "Japanese",
			Icon:  "🇯🇵",
		},
		{
			Code:  "lo",
			Baidu: "lao",
			Name:  "Lao",
			Icon:  "🇱🇦",
		},
		{
			Code:  "ko",
			Baidu: "kor",
			Name:  "한국어",
			Icon:  "🇰🇷",
		},
		{
			Code:  "ru",
			Baidu: "ru",
			Name:  "Русский",
			Icon:  "🇷🇺",
		},
		{
			Code:  "fr",
			Baidu: "fra",
			Name:  "Français",
			Icon:  "🇫🇷",
		},
		{
			Code:  "de",
			Baidu: "de",
			Name:  "Deutsch",
			Icon:  "🇩🇪",
		},
		{
			Code:  "fa",
			Baidu: "per",
			Name:  "فارسی",
			Icon:  "🇮🇷",
		},
		{
			Code:  "ar",
			Baidu: "ara",
			Name:  "العربية",
			Icon:  "🇸🇦",
		},
		{
			Code:  "ms",
			Baidu: "",
			Name:  "Bahasa Melayu",
			Icon:  "🇲🇾",
		},
		{
			Code:  "jw",
			Baidu: "jav",
			Name:  "Jawa",
			Icon:  "🇮🇩",
		},
		{
			Code:  "te",
			Baidu: "tel",
			Name:  "Telugu",
			Icon:  "🇮🇳",
		},
		{
			Code:  "ta",
			Baidu: "tam",
			Name:  "Tamil",
			Icon:  "🇮🇳",
		},
		{
			Code:  "mr",
			Baidu: "mar",
			Name:  "Marathi",
			Icon:  "🇮🇳",
		},
		{
			Code:  "ur",
			Baidu: "urd",
			Name:  "Urdu",
			Icon:  "🇵🇰",
		},
		{
			Code:  "pl",
			Baidu: "pl",
			Name:  "Polski",
			Icon:  "🇵🇱",
		},
		{
			Code:  "uk",
			Baidu: "ukr",
			Name:  "Українська",
			Icon:  "🇺🇦",
		},
		{
			Code:  "pa",
			Baidu: "pan",
			Name:  "Panjabi",
			Icon:  "🇮🇳",
		},
		{
			Code:  "ro",
			Baidu: "rom",
			Name:  "Română",
			Icon:  "🇷🇴",
		},
		{
			Code:  "et",
			Baidu: "est",
			Name:  "Eesti",
			Icon:  "🇪🇪",
		},
		{
			Code:  "os",
			Baidu: "oss",
			Name:  "Ossetic",
			Icon:  "🇷🇺",
		},
		{
			Code:  "be",
			Baidu: "bel",
			Name:  "Беларуская",
			Icon:  "🇧🇾",
		},
		{
			Code:  "bg",
			Baidu: "bul",
			Name:  "Български",
			Icon:  "🇧🇬",
		},
		{
			Code:  "is",
			Baidu: "ice",
			Name:  "Icelandic",
			Icon:  "🇮🇸",
		},
		{
			Code:  "bs",
			Baidu: "bos",
			Name:  "Bosnian",
			Icon:  "🇧🇦",
		},
		{
			Code:  "bo",
			Baidu: "tib",
			Name:  "Tibetan",
			Icon:  "🇨🇳",
		},
		{
			Code:  "da",
			Baidu: "dan",
			Name:  "Dansk",
			Icon:  "🇩🇰",
		},
		{
			Code:  "tl",
			Baidu: "tgl",
			Name:  "Filipino",
			Icon:  "🇵🇭",
		},
		{
			Code:  "fi",
			Baidu: "fin",
			Name:  "Suomi",
			Icon:  "🇫🇮",
		},
		{
			Code:  "sv",
			Baidu: "swe",
			Name:  "Swedish",
			Icon:  "🇸🇪",
		},
		{
			Code: "kg",
			Name: "Kongo",
			Icon: "🇨🇬",
		},
		{
			Code:  "ka",
			Baidu: "geo",
			Name:  "Georgian",
			Icon:  "🇬🇪",
		},
		{
			Code:  "kk",
			Baidu: "kaz",
			Name:  "Kazakh",
			Icon:  "🇰🇿",
		},
		{
			Code:  "gl",
			Baidu: "glg",
			Name:  "Galician",
			Icon:  "🇪🇸",
		},
		{
			Code:  "ky",
			Baidu: "kir",
			Name:  "Kyrgyz",
			Icon:  "🇰🇬",
		},
		{
			Code:  "nl",
			Baidu: "nl",
			Name:  "Nederlands",
			Icon:  "🇳🇱",
		},
		{
			Code:  "ca",
			Baidu: "cat",
			Name:  "Catalan",
			Icon:  "🇪🇸",
		},
		{
			Code:  "cs",
			Baidu: "cs",
			Name:  "Čeština",
			Icon:  "🇨🇿",
		},
		{
			Code:  "kn",
			Baidu: "kan",
			Name:  "Kannada",
			Icon:  "🇮🇳",
		},
		{
			Code:  "mn",
			Baidu: "mon",
			Name:  "Mongolian",
			Icon:  "🇲🇳",
		},
		{
			Code:  "hr",
			Baidu: "hrv",
			Name:  "Hrvatski",
			Icon:  "🇭🇷",
		},
		{
			Code:  "lv",
			Baidu: "lav",
			Name:  "Latvian",
			Icon:  "🇱🇻",
		},
		{
			Code:  "lt",
			Baidu: "lit",
			Name:  "Lettish",
			Icon:  "🇱🇹",
		},
		{
			Code:  "no",
			Baidu: "nor",
			Name:  "Norwegian",
			Icon:  "🇳🇴",
		},
		{
			Code:  "ne",
			Baidu: "nep",
			Name:  "Nepali",
			Icon:  "🇳🇵",
		},
		{
			Code:  "ps",
			Baidu: "pus",
			Name:  "Pashto",
			Icon:  "🇦🇫",
		},
		{
			Code: "ks",
			Name: "Slovak",
			Icon: "🇸🇰",
		},
		{
			Code:  "tk",
			Baidu: "tuk",
			Name:  "Turkmen",
			Icon:  "🇹🇲",
		},
		{
			Code:  "uz",
			Baidu: "uzb",
			Name:  "Uzbek",
			Icon:  "🇺🇿",
		},
		{
			Code:  "iw",
			Baidu: "heb",
			Name:  "Hebrew",
			Icon:  "🇮🇱",
		},
		{
			Code:  "hu",
			Baidu: "hu",
			Name:  "Hungarian",
			Icon:  "🇭🇺",
		},
		{
			Code:  "hy",
			Baidu: "arm",
			Name:  "Armenian",
			Icon:  "🇦🇲",
		},
		{
			Code:  "ht",
			Baidu: "ht",
			Name:  "Kreyòl Ayisyen",
			Icon:  "🇭🇹",
		},
		{
			Code:  "mg",
			Baidu: "mg",
			Name:  "Malagasy",
			Icon:  "🇲🇬",
		},
		{
			Code:  "mk",
			Baidu: "mac",
			Name:  "Македонски",
			Icon:  "🇲🇰",
		},
		{
			Code:  "ml",
			Baidu: "mal",
			Name:  "മലയാളം",
			Icon:  "🇮🇳",
		},
		{
			Code:  "af",
			Baidu: "afr",
			Name:  "Afrikaans",
			Icon:  "🇿🇦",
		},
		{
			Code:  "mt",
			Baidu: "mlt",
			Name:  "Malti",
			Icon:  "🇲🇹",
		},
		{
			Code:  "eu",
			Baidu: "baq",
			Name:  "Euskara",
			Icon:  "🇪🇸",
		},
		{
			Code:  "pt-PT",
			Baidu: "pt",
			Name:  "Português-PT",
			Icon:  "🇵🇹",
		},
		{
			Code:  "az",
			Baidu: "aze",
			Name:  "Azərbaycan",
			Icon:  "🇦🇿",
		},
		{
			Code:  "en-GB",
			Baidu: "en",
			Name:  "英国English",
			Icon:  "🇬🇧",
		},
		{
			Code:  "sd",
			Baidu: "snd",
			Name:  "سنڌي",
			Icon:  "🇵🇰",
		},
		{
			Code: "se",
			Name: "Davvisámegiella",
			Icon: "🇳🇴",
		},
		{
			Code:  "si",
			Baidu: "sin",
			Name:  "සිංහල",
			Icon:  "🇱🇰",
		},
		{
			Code:  "sk",
			Baidu: "sk",
			Name:  "Slovenčina",
			Icon:  "🇸🇰",
		},
		{
			Code:  "sl",
			Baidu: "slo",
			Name:  "Slovenščina",
			Icon:  "🇸🇮",
		},
		{
			Code:  "ga",
			Baidu: "gle",
			Name:  "Gaeilge",
			Icon:  "🇮🇪",
		},
		{
			Code: "sn",
			Name: "Shona",
			Icon: "🇿🇼",
		},
		{
			Code:  "so",
			Baidu: "som",
			Name:  "Soomaali",
			Icon:  "🇸🇴",
		},
		{
			Code: "gd",
			Name: "Gàidhlig",
			Icon: "🇬🇧",
		},
		{
			Code:  "sq",
			Baidu: "alb",
			Name:  "Shqip",
			Icon:  "🇦🇱",
		},
		{
			Code: "st",
			Name: "Sesotho",
			Icon: "🇱🇸",
		},
		{
			Code:  "km",
			Baidu: "hkm",
			Name:  "ភាសាខ្មែរ",
			Icon:  "🇰🇭",
		},
		{
			Code:  "sw",
			Baidu: "swa",
			Name:  "Kiswahili",
			Icon:  "🇹🇿",
		},
		{
			Code:  "pt-BR",
			Baidu: "pt",
			Name:  "Português-BR",
			Icon:  "🇧🇷",
		},
		{
			Code: "co",
			Name: "Corsu",
			Icon: "🇫🇷",
		},
		{
			Code:  "gu",
			Baidu: "guj",
			Name:  "ગુજરાતી",
			Icon:  "🇮🇳",
		},
		{
			Code:  "tg",
			Baidu: "tgk",
			Name:  "Тоҷикӣ",
			Icon:  "🇹🇯",
		},
		{
			Code:  "la",
			Baidu: "lat",
			Name:  "Latina",
			Icon:  "🇻🇦",
		},
		{
			Code:  "cy",
			Baidu: "wel",
			Name:  "Cymraeg",
			Icon:  "🇬🇧",
		},
	}
)

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
