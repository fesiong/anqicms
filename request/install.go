package request

type Install struct {
	Database      string `form:"database" validate:"required"`
	User          string `form:"user" validate:"required"`
	Password      string `form:"password" validate:"required"`
	Host          string `form:"host" validate:"required"`
	Port          int    `form:"port" validate:"required"`
	AdminUser     string `form:"admin_user" validate:"required"`
	AdminPassword string `form:"admin_password" validate:"required"`
}
