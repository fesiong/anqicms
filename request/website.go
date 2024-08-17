package request

import "kandaoni.com/anqicms/config"

type WebsiteRequest struct {
	Id            uint               `json:"id"`
	RootPath      string             `json:"root_path"`
	Name          string             `json:"name"`
	Status        uint               `json:"status"`
	Mysql         config.MysqlConfig `json:"mysql"`
	AdminUser     string             `json:"admin_user" validate:"required"`
	AdminPassword string             `json:"admin_password" validate:"required"`
	BaseUrl       string             `json:"base_url"`
	PreviewData   bool               `json:"preview_data"`
	Initialed     bool               `json:"initialed"`
	RemoveFile    bool               `json:"remove_file"`
	Template      string             `json:"template"`
}
