package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

type Character struct {
	Model
	Hash                     string      `json:"hash" gorm:"column:hash;type:varchar(64);not null;default:'';primary_key"`
	PinYin                   ArrayString `json:"pin_yin" gorm:"column:pin_yin;type:varchar(64);default null"`                                            //拼音
	Ch                       string      `json:"ch" gorm:"column:ch;type:varchar(4);not null;default:''"`                                                //字符
	ScienceStroke            int         `json:"science_stroke" gorm:"column:science_stroke;type:tinyint(2);not null;default:0"`                         //科学笔画
	Radical                  string      `json:"radical" gorm:"column:radical;type:varchar(4);not null;default:''"`                                      //部首
	RadicalStroke            int         `json:"radical_stroke" gorm:"column:radical_stroke;type:tinyint(2);not null;default:0"`                         //部首笔画
	Stroke                   int         `json:"stroke" gorm:"column:stroke;type:tinyint(2);not null;default:0"`                                         //总笔画数
	IsKangXi                 int         `json:"is_kang_xi" gorm:"column:is_kang_xi;type:tinyint(1);not null;default:0"`                                 //是否康熙字典
	KangXi                   string      `json:"kang_xi" gorm:"column:kang_xi;type:varchar(4);not null;default:''"`                                      //康熙
	KangXiStroke             int         `json:"kang_xi_stroke" gorm:"column:kang_xi_stroke;type:tinyint(2);not null;default:0"`                         //康熙笔画
	SimpleRadical            string      `json:"simple_radical" gorm:"column:simple_radical;type:varchar(4);not null;default:''"`                        //简体部首
	SimpleRadicalStroke      int         `json:"simple_radical_stroke" gorm:"column:simple_radical_stroke;type:tinyint(2);not null;default:0"`           //简体部首笔画
	SimpleTotalStroke        int         `json:"simple_total_stroke" gorm:"column:simple_total_stroke;type:tinyint(2);not null;default:0"`               //简体笔画
	TraditionalRadical       string      `json:"traditional_radical" gorm:"column:traditional_radical;type:varchar(4);not null;default:''"`              //繁体部首
	TraditionalRadicalStroke int         `json:"traditional_radical_stroke" gorm:"column:traditional_radical_stroke;type:tinyint(2);not null;default:0"` //繁体部首笔画
	TraditionalTotalStroke   int         `json:"traditional_total_stroke" gorm:"column:traditional_total_stroke;type:tinyint(2);not null;default:0"`     //简体部首笔画
	NameScience              int         `json:"name_science" gorm:"column:name_science;type:tinyint(1);not null;default:0"`                             //姓名学
	WuXing                   string      `json:"wu_xing" gorm:"column:wu_xing;type:varchar(4);not null;default:''"`                                      //五行
	Lucky                    string      `json:"lucky" gorm:"column:lucky;type:varchar(4);not null;default:''"`                                          //吉凶寓意
	Regular                  bool        `json:"regular" gorm:"column:regular;type:tinyint(1);not null;default:0"`                                       //常用
	TraditionalCharacter     ArrayString `json:"traditional_character" gorm:"column:traditional_character;type:varchar(64);default null"`                //繁体字
	VariantCharacter         ArrayString `json:"variant_character" gorm:"column:variant_character;type:varchar(64);default null"`                        //异体字
	Comment                  ArrayString `json:"comment" gorm:"column:comment;type:text;default null"`                                                   //解释
	Male                     int         `json:"male" gorm:"column:male;type:int(10);not null;default:0"`
	Female                   int         `json:"female" gorm:"column:female;type:int(10);not null;default:0"`
	Total                    int         `json:"total" gorm:"column:total;type:int(10);not null;default:0"`
}

type ArrayString []string

func (a ArrayString) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func (a *ArrayString) Scan(data interface{}) error {
	_ = json.Unmarshal(data.([]byte), &a)
	return nil
}

func (c *Character) GetPinYin() string {
	return strings.Join(c.PinYin, "、")
}

func (c *Character) GetTraditionalCharacter() string {
	return strings.Join(c.TraditionalCharacter, "、")
}

func (c *Character) GetComment(full bool) string {
	str := ""
	if full {
		for _, v := range c.Comment {
			str += fmt.Sprintf("<p>%s</p>", strings.Replace(v, "\n", "<br>", -1))
		}
	} else {
		if len(c.Comment) > 0 {
			str = c.Comment[0]
		}
	}

	return str
}
