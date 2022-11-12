package request

type Install struct {
	Database      string `json:"database" validate:"required"`
	User          string `json:"user" validate:"required"`
	Password      string `json:"password" validate:"required"`
	Host          string `json:"host" validate:"required"`
	Port          int    `json:"port" validate:"required"`
	AdminUser     string `json:"admin_user" validate:"required"`
	AdminPassword string `json:"admin_password" validate:"required"`
	BaseUrl       string `json:"base_url"`
	PreviewData   bool   `json:"preview_data"`
}
