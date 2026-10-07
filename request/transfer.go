package request

type TransferWebsite struct {
	Name     string `json:"name"`      // 站点名称
	BaseUrl  string `json:"base_url"`  // 站点地址
	Token    string `json:"token"`     // 通信 Token
	TargetId string `json:"target_id"` // 目标站点 ID（迅睿多站点模式需填写）
	Provider string `json:"provider"`  // 待迁移站点的建站系统标识：dedecms=织梦CMS，empire=帝国CMS，pbootcms=PBootCMS，wordpress=WordPress,xunruicms=迅睿CMS
}

type TransferTypes struct {
	ModuleIds []uint   `json:"module_ids"` // 模型 ID
	Types     []string `json:"types"`      // 可操作的类型：module=模型，category=分类，tag=标签，keyword=锚文本，archive=文档，singlepage=单页面，static=静态资源
}
