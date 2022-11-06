package config

type pluginUserConfig struct {
	Fields         []*CustomField `json:"fields"`
	DefaultGroupId uint           `json:"default_group_id"`
}

func GetUserFields() []*CustomField {
	//这里有默认的设置
	fields := JsonData.PluginUser.Fields

	return fields
}
