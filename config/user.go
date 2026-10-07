package config

type PluginUserConfig struct {
	Fields         []*CustomField `json:"fields"`           // 用户扩展字段列表。
	DefaultGroupId uint           `json:"default_group_id"` // 默认用户组 ID。
	DefaultStatus  string         `json:"default_status"`   // 默认正常，pending=待审核，blocked=禁止
}

func (p *PluginUserConfig) GetDefaultStatus() int {
	switch p.DefaultStatus {
	case "pending":
		return 0
	case "blocked":
		return -1
	default:
		return 1
	}
}

type PluginGoogleAuthConfig struct {
	RedirectUrl  string `json:"redirect_url"`  // 登录成功后跳转的URL
	ClientId     string `json:"client_id"`     // 谷歌应用
	ClientSecret string `json:"client_secret"` // 谷歌应用密钥
}
