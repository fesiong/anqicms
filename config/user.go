package config

type PluginUserConfig struct {
	Fields         []*CustomField `json:"fields"`
	DefaultGroupId uint           `json:"default_group_id"`
	DefaultStatus  string         `json:"default_status"` // 默认正常，pending 待审核，blocked 禁止
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
	RedirectUrl  string `json:"redirect_url"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	PlacesApiKey string `json:"places_api_key"`
}
