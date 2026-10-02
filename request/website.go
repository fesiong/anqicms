package request

import "kandaoni.com/anqicms/config"

type WebsiteRequest struct {
	Id            uint               `json:"id,omitempty"`   // 站点ID
	RootPath      string             `json:"root_path"`      // 站点根目录
	Name          string             `json:"name"`           // 站点名称
	Status        uint               `json:"status"`         // 站点状态
	Mysql         config.MysqlConfig `json:"mysql"`          // 站点数据库配置。
	AdminUser     string             `json:"admin_user"`     // 管理员用户名
	AdminPassword string             `json:"admin_password"` // 管理员密码，新建站点必填。
	BaseUrl       string             `json:"base_url"`       // 站点网址
	FrontUrl      string             `json:"front_url"`      // 前台网址（前后端分离的站点可配置）
	PreviewData   bool               `json:"preview_data"`   // 创建站点时是否导入模板数据
	Initialed     bool               `json:"initialed" ast:"-"`
	Template      string             `json:"template"` // 站点模板包名，默认 default
}

type WebsiteDeleteRequest struct {
	Id         uint `json:"id" validate:"required"` // 站点ID
	RemoveFile bool `json:"remove_file,omitempty"`  // 删除站点时是否删除文件
}
