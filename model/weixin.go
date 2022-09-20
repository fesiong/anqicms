package model

type UserWeixin struct {
	Model
	UserId    uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Nickname  string `json:"nickname" gorm:"column:nickname;type:varchar(64) not null;default:''"`
	AvatarURL string `json:"avatar_url" gorm:"column:avatar_url;type:varchar(255) not null;default:''"`
	Gender    int    `json:"gender" gorm:"column:gender;type:int(10) not null;default:0"`
	Openid    string `json:"openid" gorm:"column:openid;type:varchar(128) not null;default:'';index"`
	UnionId   string `json:"union_id" gorm:"column:union_id;type:varchar(128) not null;default:'';index"`
	Status    int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
}

type WeappQrcode struct {
	Model
	UserId  uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Path    string `json:"path" gorm:"column:path;type:varchar(255) not null;default:'';index:idx_path"`
	CodeUrl string `json:"code_url" gorm:"column:code_url;type:varchar(255) not null;default:''"`
}

type SubscribedUser struct {
	Model
	UserId     uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Openid     string `json:"openid" gorm:"column:openid;type:varchar(255) not null;default:'';index:idx_openid_template_id"`
	TemplateId string `json:"template_id" gorm:"column:template_id;type:varchar(64) not null;default:'';index:idx_openid_template_id"`
}
