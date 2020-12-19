package request

type Admin struct {
	UserName string `form:"user_name" validate:"required"`
	Password string `form:"password" validate:"required"`
}
