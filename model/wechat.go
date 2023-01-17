package model

type UserWechat struct {
	Model
	UserId    uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Nickname  string `json:"nickname" gorm:"column:nickname;type:varchar(64) not null;default:''"`
	AvatarURL string `json:"avatar_url" gorm:"column:avatar_url;type:varchar(255) not null;default:''"`
	Gender    int    `json:"gender" gorm:"column:gender;type:int(10) not null;default:0"`
	Openid    string `json:"openid" gorm:"column:openid;type:varchar(128) not null;default:'';index"`
	UnionId   string `json:"union_id" gorm:"column:union_id;type:varchar(128) not null;default:'';index"`
	Platform  string `json:"platform" gorm:"column:platform;type:varchar(32) not null;default:''"`
	Status    int    `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`
}

type WeappQrcode struct {
	Model
	UserId  uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Path    string `json:"path" gorm:"column:path;type:varchar(190) not null;default:'';index:idx_path"`
	CodeUrl string `json:"code_url" gorm:"column:code_url;type:varchar(255) not null;default:''"`
}

type SubscribedUser struct {
	Model
	UserId     uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index"`
	Openid     string `json:"openid" gorm:"column:openid;type:varchar(128) not null;default:'';index:idx_openid_template_id"`
	TemplateId string `json:"template_id" gorm:"column:template_id;type:varchar(64) not null;default:'';index:idx_openid_template_id"`
}

type WechatMessage struct {
	Model
	Openid    string `json:"openid" gorm:"column:openid;type:varchar(128) not null;default:''"`
	Content   string `json:"content" gorm:"column:content;type:varchar(255) not null;default:''"`
	Reply     string `json:"reply" gorm:"column:reply;type:varchar(255) not null;default:''"`
	ReplyTime int64  `json:"reply_time" gorm:"column:reply_time;type:int(10) not null;default:0"`
}

type WechatReplyRule struct {
	Model
	Keyword   string `json:"keyword" gorm:"column:keyword;type:varchar(128) not null;default:'';index"`
	Content   string `json:"content" gorm:"column:content;type:text default null;"`
	IsDefault int    `json:"is_default" gorm:"column:is_default;type:tinyint(1) not null;default:0"`
}

type WechatMenu struct {
	Model
	ParentId uint   `json:"parent_id" gorm:"column:parent_id;type:int(10) not null;default:0"`
	Name     string `json:"name" gorm:"column:name;type:varchar(20) not null;default:''"`
	Type     string `json:"type" gorm:"column:type;type:varchar(20) not null;default:''"`
	Value    string `json:"value" gorm:"column:value;type:varchar(255) not null;default:''"`
	Sort     uint   `json:"sort" gorm:"column:sort;type:int(10) not null;default:10;index"`

	Children []*WechatMenu `json:"children,omitempty" gorm:"-"`
}
