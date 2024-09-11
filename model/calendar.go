package model

type Calendar struct {
	Model
	SolarDate string `json:"solar_date" gorm:"column:solar_date;type:varchar(10);not null;default:''"`
	LunarDate string `json:"lunar_date" gorm:"column:lunar_date;type:varchar(10);not null;default:''"`
	LunarLeap int    `json:"lunar_leap" gorm:"column:lunar_leap;type:tinyint(1);not null;default:0"`
	NianZhu   string `json:"nian_zhu" gorm:"column:nian_zhu;type:varchar(4);not null;default:''"`
	YueZhu    string `json:"yue_zhu" gorm:"column:yue_zhu;type:varchar(4);not null;default:''"`
	RiZhu     string `json:"ri_zhu" gorm:"column:ri_zhu;type:varchar(4);not null;default:''"`
	ShiZhu    string `json:"shi_zhu" gorm:"column:shi_zhu;type:varchar(4);not null;default:''"`
	Zodiac    string `json:"zodiac" gorm:"column:zodiac;type:varchar(4);not null;default:''"`
}
