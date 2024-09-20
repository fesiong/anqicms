package config

type PluginUserConfig struct {
	Fields         []*CustomField `json:"fields"`
	DefaultGroupId uint           `json:"default_group_id"`
}
