package model

import (
	"fmt"
	"github.com/godcong/chronos"
	"gorm.io/gorm"
	"time"
)

type Horoscope struct {
	Model
	Hash  string `json:"hash" gorm:"column:hash;type:varchar(32);unique;not null"`
	Born  int    `json:"born" gorm:"column:born;type:int(11);not null;default:0"`
	Views uint   `json:"views" gorm:"column:views;type:int(10);not null;default:0"`
}

func (horoscope *Horoscope) AddViews(db *gorm.DB) error {
	horoscope.Views = horoscope.Views + 1
	db.Model(&Horoscope{}).Where("`id` = ?", horoscope.Id).Update("views", horoscope.Views)
	return nil
}

func (horoscope *Horoscope) String() string {
	born := chronos.New(time.Unix(int64(horoscope.Born), 0))
	str := fmt.Sprintf("%s出生的八字", born.LunarDate())

	return str
}
