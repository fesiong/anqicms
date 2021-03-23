package request

import "irisweb/config"

type PluginPushConfig struct {
	BaiduApi string `json:"baidu_api"`
	BingApi  string `json:"bing_api"`
	JsCode   string `json:"js_code"`
}

type PluginRobotsConfig struct {
	Robots string `json:"robots"`
}

type PluginSitemapConfig struct {
	AutoBuild int `json:"auto_build"`
}

type PluginRewriteConfig struct {
	Mode   int    `json:"mode"`
	Patten string `json:"patten"`
}

type PluginLink struct {
	Id       uint   `json:"id"`
	Title    string `json:"title"`
	Link     string `json:"link"`
	BackLink string `json:"back_link"`
	MyTitle  string `json:"my_title"`
	MyLink   string `json:"my_link"`
	Contact  string `json:"contact"`
	Remark   string `json:"remark"`
	Nofollow uint   `json:"nofollow"`
	Sort     uint   `json:"sort"`
	Status   uint   `json:"status"`
}

type PluginComment struct {
	Id        uint   `json:"id"`
	ItemType  string `json:"item_type"`
	ItemId    uint   `json:"item_id"`
	UserId    uint   `json:"user_id"`
	UserName  string `json:"user_name"`
	Ip        string `json:"ip"`
	VoteCount uint   `json:"vote_count"`
	Content   string `json:"content"`
	ParentId  uint   `json:"parent_id"`
	ToUid     uint   `json:"to_uid"`
	Status    uint   `json:"status"`
}

type PluginAnchor struct {
	Id     uint   `json:"id"`
	Title  string `json:"title"`
	Link   string `json:"link"`
	Weight int    `json:"weight"`
}

type PluginAnchorDelete struct {
	Id  uint   `json:"id"`
	Ids []uint `json:"ids"`
}

type PluginAnchorSetting struct {
	AnchorDensity int `json:"anchor_density"`
	ReplaceWay    int `json:"replace_way"`
	KeywordWay    int `json:"keyword_way"`
}

type PluginGuestbookSetting struct {
	ReturnMessage string                   `json:"return_message"`
	Fields        []*config.GuestbookField `json:"fields"`
}

type PluginGuestbookDelete struct {
	Id  uint   `json:"id"`
	Ids []uint `json:"ids"`
}

type PluginKeyword struct {
	Id    uint   `json:"id"`
	Title string `json:"title"`
}

type PluginKeywordDelete struct {
	Id  uint   `json:"id"`
	Ids []uint `json:"ids"`
}

type PluginFileUploadDelete struct {
	Hash string `json:"hash"`
}

type PluginMaterial struct {
	Id         uint   `json:"id"`
	Title      string `json:"title"`
	CategoryId uint   `json:"category_id"`
	Content    string `json:"content"`
	Status     uint   `json:"status"`
	AutoUpdate uint   `json:"auto_update"`
}

type PluginMaterialCategory struct {
	Id    uint   `json:"id"`
	Title string `json:"title"`
}
