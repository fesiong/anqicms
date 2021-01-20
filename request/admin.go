package request

type Admin struct {
	UserName string `json:"user_name" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type ChangeAdmin struct {
	UserName    string `json:"user_name" validate:"required"`
	OldPassword string `json:"old_password"`
	Password    string `json:"password"`
	RePassword  string `json:"re_password"`
}
