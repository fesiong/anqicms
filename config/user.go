package config

type PluginUserConfig struct {
	Fields         []*CustomField `json:"fields"`
	DefaultGroupId uint           `json:"default_group_id"`
}

type PluginGoogleAuthConfig struct {
	RedirectUrl  string `json:"redirect_url"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	PlacesApiKey string `json:"places_api_key"`
}
