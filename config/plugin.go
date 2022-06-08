package config

import (
	"fmt"
	"strings"
)

type pluginPushConfig struct {
	BaiduApi string `json:"baidu_api"`
	BingApi  string `json:"bing_api"`
	JsCode   string `json:"js_code"`
}

type pluginSitemapConfig struct {
	AutoBuild   int    `json:"auto_build"`
	UpdatedTime int64  `json:"updated_time"`
	SitemapURL  string `json:"sitemap_url"`
}

type pluginAnchorConfig struct {
	AnchorDensity int `json:"anchor_density"`
	ReplaceWay    int `json:"replace_way"`
	KeywordWay    int `json:"keyword_way"`
}

type pluginGuestbookConfig struct {
	ReturnMessage string         `json:"return_message"`
	Fields        []*CustomField `json:"fields"`
}

type CustomField struct {
	Name      string   `json:"name"`
	FieldName string   `json:"field_name"`
	Type      string   `json:"type"`
	Required  bool     `json:"required"`
	IsSystem  bool     `json:"is_system"`
	IsFilter  bool     `json:"is_filter"`
	Content   string   `json:"content"`
	Items     []string `json:"-"`
}

type PluginUploadFile struct {
	Hash        string `json:"hash"`
	FileName    string `json:"file_name"`
	CreatedTime int64  `json:"created_time"`
	Link        string `json:"link"`
}

type pluginSendmail struct {
	Server    string `json:"server"`
	UseSSL    int    `json:"use_ssl"`
	Port      int    `json:"port"`
	Account   string `json:"account"`
	Password  string `json:"password"`
	Recipient string `json:"recipient"`
}

type pluginImportApiConfig struct {
	Token string `json:"token"`
}

func (g *CustomField) SplitContent() []string {
	var items []string
	contents := strings.Split(g.Content, "\n")
	for _, v := range contents {
		v = strings.TrimSpace(v)
		if v != "" {
			items = append(items, v)
		}
	}

	g.Items = items

	return items
}

// CheckSetFilter 支付允许筛选
func (g *CustomField) CheckSetFilter() bool {
	if g.Type != CustomFieldTypeRadio && g.Type != CustomFieldTypeCheckbox && g.Type != CustomFieldTypeSelect {
		g.IsFilter = false
		return false
	}

	return true
}

func (g *CustomField) GetFieldColumn() string {
	column := fmt.Sprintf("`%s`", g.FieldName)

	if g.Type == CustomFieldTypeNumber {
		column += " int(10)"
	} else if g.Type == CustomFieldTypeTextarea {
		column += " text"
	} else {
		column += " varchar(250)"
	}

	//if g.Required {
	//	column += " NOT NULL"
	//} else {
	//	column += " DEFAULT NULL"
	//}
	// 因为是后插值，因此这里默认都是null
	column += " DEFAULT NULL"

	return column
}

func GetGuestbookFields() []*CustomField {
	//这里有默认的设置
	defaultFields := []*CustomField{
		{
			Name:      Lang("用户名"),
			FieldName: "user_name",
			Type:      "text",
			Required:  true,
			IsSystem:  true,
		},
		{
			Name:      Lang("联系方式"),
			FieldName: "contact",
			Type:      "text",
			Required:  true,
			IsSystem:  true,
		},
		{
			Name:      Lang("留言内容"),
			FieldName: "content",
			Type:      "textarea",
			Required:  true,
			IsSystem:  true,
		},
	}

	fields := append(defaultFields, JsonData.PluginGuestbook.Fields...)

	return fields
}
