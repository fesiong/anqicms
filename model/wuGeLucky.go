package model

type WuGeLucky struct {
	Model
	LastStroke1  int    `json:"last_stroke_1" gorm:"column:last_stroke_1;type:tinyint(2);not null;default:0"`   //姓1 笔画数
	LastStroke2  int    `json:"last_stroke_2" gorm:"column:last_stroke_2;type:tinyint(2);not null;default:0"`   //姓2 笔画数
	FirstStroke1 int    `json:"first_stroke_1" gorm:"column:first_stroke_1;type:tinyint(2);not null;default:0"` //名1 笔画数
	FirstStroke2 int    `json:"first_stroke_2" gorm:"column:first_stroke_2;type:tinyint(2);not null;default:0"` //名2 笔画数
	TianGe       int    `json:"tian_ge" gorm:"column:tian_ge;type:tinyint(2);not null;default:0"`
	TianDaYan    string `json:"tian_da_yan" gorm:"column:tian_da_yan;type:varchar(4);not null;default:''"`
	RenGe        int    `json:"ren_ge" gorm:"column:ren_ge;type:tinyint(2);not null;default:0"`
	RenDaYan     string `json:"ren_da_yan" gorm:"column:ren_da_yan;type:varchar(4);not null;default:''"`
	DiGe         int    `json:"di_ge" gorm:"column:di_ge;type:tinyint(2);not null;default:0"`
	DiDaYan      string `json:"di_da_yan" gorm:"column:di_da_yan;type:varchar(4);not null;default:''"`
	WaiGe        int    `json:"wai_ge" gorm:"column:wai_ge;type:tinyint(2);not null;default:0"`
	WaiDaYan     string `json:"wai_da_yan" gorm:"column:wai_da_yan;type:varchar(4);not null;default:''"`
	ZongGe       int    `json:"zong_ge" gorm:"column:zong_ge;type:tinyint(2);not null;default:0"`
	ZongDaYan    string `json:"zong_da_yan" gorm:"column:zong_da_yan;type:varchar(4);not null;default:''"`
	ZongLucky    bool   `json:"zong_lucky" gorm:"column:zong_lucky;type:tinyint(2);not null;default:0"`
	ZongSex      bool   `json:"zong_sex" gorm:"column:zong_sex;type:tinyint(1);not null;default:0"`
	ZongMax      bool   `json:"zong_max" gorm:"column:zong_max;type:tinyint(1);not null;default:0"`
}
