package library

type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

var (
	// 语言列表
	Languages = []Language{
		{
			Code: "en",
			Name: "English",
			Icon: "🇺🇸",
		},
		{
			Code: "zh-CN",
			Name: "简体中文",
			Icon: "🇨🇳",
		},
		{
			Code: "zh-TW",
			Name: "繁体中文",
			Icon: "🇨🇳",
		},
		{
			Code: "vi",
			Name: "Tiếng Việt",
			Icon: "🇻🇳",
		},
		{
			Code: "id",
			Name: "Bahasa Indonesia",
			Icon: "🇮🇩",
		},
		{
			Code: "hi",
			Name: "Hindi",
			Icon: "🇮🇳",
		},
		{
			Code: "it",
			Name: "Italiano",
			Icon: "🇮🇹",
		},
		{
			Code: "el",
			Name: "Greek",
			Icon: "🇬🇷",
		},
		{
			Code: "es",
			Name: "Español",
			Icon: "🇪🇸",
		},
		{
			Code: "pt",
			Name: "Português",
			Icon: "🇵🇹",
		},
		{
			Code: "sr",
			Name: "Srpski",
			Icon: "🇷🇸",
		},
		{
			Code: "my",
			Name: "Burmese",
			Icon: "🇲🇲",
		},
		{
			Code: "bn",
			Name: "Bengali",
			Icon: "🇧🇩",
		},
		{
			Code: "th",
			Name: "Thai",
			Icon: "🇹🇭",
		},
		{
			Code: "tr",
			Name: "Türkçe",
			Icon: "🇹🇷",
		},
		{
			Code: "ja",
			Name: "Japanese",
			Icon: "🇯🇵",
		},
		{
			Code: "lo",
			Name: "Lao",
			Icon: "🇱🇦",
		},
		{
			Code: "ko",
			Name: "한국어",
			Icon: "🇰🇷",
		},
		{
			Code: "ru",
			Name: "Русский",
			Icon: "🇷🇺",
		},
		{
			Code: "fr",
			Name: "Français",
			Icon: "🇫🇷",
		},
		{
			Code: "de",
			Name: "Deutsch",
			Icon: "🇩🇪",
		},
		{
			Code: "fa",
			Name: "فارسی",
			Icon: "🇮🇷",
		},
		{
			Code: "ar",
			Name: "العربية",
			Icon: "🇸🇦",
		},
		{
			Code: "ms",
			Name: "Bahasa Melayu",
			Icon: "🇲🇾",
		},
		{
			Code: "jw",
			Name: "Jawa",
			Icon: "🇮🇩",
		},
		{
			Code: "te",
			Name: "Telugu",
			Icon: "🇮🇳",
		},
		{
			Code: "ta",
			Name: "Tamil",
			Icon: "🇮🇳",
		},
		{
			Code: "mr",
			Name: "Marathi",
			Icon: "🇮🇳",
		},
		{
			Code: "ur",
			Name: "Urdu",
			Icon: "🇵🇰",
		},
		{
			Code: "pl",
			Name: "Polski",
			Icon: "🇵🇱",
		},
		{
			Code: "uk",
			Name: "Українська",
			Icon: "🇺🇦",
		},
		{
			Code: "pa",
			Name: "Panjabi",
			Icon: "🇮🇳",
		},
		{
			Code: "ro",
			Name: "Română",
			Icon: "🇷🇴",
		},
		{
			Code: "et",
			Name: "Eesti",
			Icon: "🇪🇪",
		},
		{
			Code: "os",
			Name: "Ossetic",
			Icon: "🇷🇺",
		},
		{
			Code: "be",
			Name: "Беларуская",
			Icon: "🇧🇾",
		},
		{
			Code: "bg",
			Name: "Български",
			Icon: "🇧🇬",
		},
		{
			Code: "is",
			Name: "Icelandic",
			Icon: "🇮🇸",
		},
		{
			Code: "bs",
			Name: "Bosnian",
			Icon: "🇧🇦",
		},
		{
			Code: "bo",
			Name: "Tibetan",
			Icon: "🇨🇳",
		},
		{
			Code: "da",
			Name: "Dansk",
			Icon: "🇩🇰",
		},
		{
			Code: "tl",
			Name: "Filipino",
			Icon: "🇵🇭",
		},
		{
			Code: "fi",
			Name: "Suomi",
			Icon: "🇫🇮",
		},
		{
			Code: "sv",
			Name: "Swedish",
			Icon: "🇸🇪",
		},
		{
			Code: "kg",
			Name: "Kongo",
			Icon: "🇨🇬",
		},
		{
			Code: "ka",
			Name: "Georgian",
			Icon: "🇬🇪",
		},
		{
			Code: "kk",
			Name: "Kazakh",
			Icon: "🇰🇿",
		},
		{
			Code: "gl",
			Name: "Galician",
			Icon: "🇪🇸",
		},
		{
			Code: "ky",
			Name: "Kyrgyz",
			Icon: "🇰🇬",
		},
		{
			Code: "nl",
			Name: "Nederlands",
			Icon: "🇳🇱",
		},
		{
			Code: "ca",
			Name: "Catalan",
			Icon: "🇪🇸",
		},
		{
			Code: "cs",
			Name: "Čeština",
			Icon: "🇨🇿",
		},
		{
			Code: "kn",
			Name: "Kannada",
			Icon: "🇮🇳",
		},
		{
			Code: "mn",
			Name: "Mongolian",
			Icon: "🇲🇳",
		},
		{
			Code: "hr",
			Name: "Hrvatski",
			Icon: "🇭🇷",
		},
		{
			Code: "lv",
			Name: "Latvian",
			Icon: "🇱🇻",
		},
		{
			Code: "lt",
			Name: "Lettish",
			Icon: "🇱🇹",
		},
		{
			Code: "no",
			Name: "Norwegian",
			Icon: "🇳🇴",
		},
		{
			Code: "ne",
			Name: "Nepali",
			Icon: "🇳🇵",
		},
		{
			Code: "ps",
			Name: "Pashto",
			Icon: "🇦🇫",
		},
		{
			Code: "ks",
			Name: "Slovak",
			Icon: "🇸🇰",
		},
		{
			Code: "tk",
			Name: "Turkmen",
			Icon: "🇹🇲",
		},
		{
			Code: "uz",
			Name: "Uzbek",
			Icon: "🇺🇿",
		},
		{
			Code: "iw",
			Name: "Hebrew",
			Icon: "🇮🇱",
		},
		{
			Code: "hu",
			Name: "Hungarian",
			Icon: "🇭🇺",
		},
		{
			Code: "hy",
			Name: "Armenian",
			Icon: "🇦🇲",
		},
		{
			Code: "ht",
			Name: "Kreyòl Ayisyen",
			Icon: "🇭🇹",
		},
		{
			Code: "mg",
			Name: "Malagasy",
			Icon: "🇲🇬",
		},
		{
			Code: "mk",
			Name: "Македонски",
			Icon: "🇲🇰",
		},
		{
			Code: "ml",
			Name: "മലയാളം",
			Icon: "🇮🇳",
		},
		{
			Code: "af",
			Name: "Afrikaans",
			Icon: "🇿🇦",
		},
		{
			Code: "mt",
			Name: "Malti",
			Icon: "🇲🇹",
		},
		{
			Code: "eu",
			Name: "Euskara",
			Icon: "🇪🇸",
		},
		{
			Code: "pt-PT",
			Name: "Português-PT",
			Icon: "🇵🇹",
		},
		{
			Code: "az",
			Name: "Azərbaycan",
			Icon: "🇦🇿",
		},
		{
			Code: "en-GB",
			Name: "英国English",
			Icon: "🇬🇧",
		},
		{
			Code: "sd",
			Name: "سنڌي",
			Icon: "🇵🇰",
		},
		{
			Code: "se",
			Name: "Davvisámegiella",
			Icon: "🇳🇴",
		},
		{
			Code: "si",
			Name: "සිංහල",
			Icon: "🇱🇰",
		},
		{
			Code: "sk",
			Name: "Slovenčina",
			Icon: "🇸🇰",
		},
		{
			Code: "sl",
			Name: "Slovenščina",
			Icon: "🇸🇮",
		},
		{
			Code: "ga",
			Name: "Gaeilge",
			Icon: "🇮🇪",
		},
		{
			Code: "sn",
			Name: "Shona",
			Icon: "🇿🇼",
		},
		{
			Code: "so",
			Name: "Soomaali",
			Icon: "🇸🇴",
		},
		{
			Code: "gd",
			Name: "Gàidhlig",
			Icon: "🇬🇧",
		},
		{
			Code: "sq",
			Name: "Shqip",
			Icon: "🇦🇱",
		},
		{
			Code: "st",
			Name: "Sesotho",
			Icon: "🇱🇸",
		},
		{
			Code: "km",
			Name: "ភាសាខ្មែរ",
			Icon: "🇰🇭",
		},
		{
			Code: "sw",
			Name: "Kiswahili",
			Icon: "🇹🇿",
		},
		{
			Code: "pt-BR",
			Name: "Português-BR",
			Icon: "🇧🇷",
		},
		{
			Code: "co",
			Name: "Corsu",
			Icon: "🇫🇷",
		},
		{
			Code: "gu",
			Name: "ગુજરાતી",
			Icon: "🇮🇳",
		},
		{
			Code: "tg",
			Name: "Тоҷикӣ",
			Icon: "🇹🇯",
		},
		{
			Code: "la",
			Name: "Latina",
			Icon: "🇻🇦",
		},
		{
			Code: "cy",
			Name: "Cymraeg",
			Icon: "🇬🇧",
		},
	}
)

func GetLanguageName(lang string) string {
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
	var icon string
	for i := range Languages {
		if Languages[i].Code == lang {
			icon = Languages[i].Icon
			break
		}
	}

	return icon
}
