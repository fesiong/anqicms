package fate

import (
	"fmt"
	"github.com/godcong/chronos"
	"math"
	"strconv"
	"strings"
	"time"
)

type LunarDate struct {
	Year   string    `json:"year"`
	Month  string    `json:"month"`
	Day    string    `json:"day"`
	Hour   string    `json:"hour"`
	IsLeap bool      `json:"is_leap"`
	Solar  time.Time `json:"-"`
}

var number = []string{`一`, `二`, `三`, `四`, `五`, `六`, `七`, `八`, `九`, `十`, `十一`, `十二`}
var ten = []string{`初`, `十`, `廿`, `卅`}

// 月历月份
var chineseNumber = []string{`正`, `二`, `三`, `四`, `五`, `六`, `七`, `八`, `九`, `十`, `十一`, `腊`}

func (l *LunarDate) GetYear() string {
	return l.Year
}

func (l *LunarDate) GetMonth() string {
	return l.Month
}

func (l *LunarDate) GetDay() string {
	return l.Day
}

func (l *LunarDate) GetHour() string {
	return l.Hour
}

func (l *LunarDate) GetIsLeap() bool {
	return l.IsLeap
}

func (l *LunarDate) Format(format string) string {
	month := strings.Replace(strings.Replace(l.Month, "月", "", 1), "闰", "", 1)
	day := strings.Split(strings.Replace(l.Day, "日", "", 1), "")

	monthDay := ""
	for i, v := range chineseNumber {
		if v == month {
			monthDay = fmt.Sprintf("%02d", i+1)
		}
	}

	dayDay := ""
	if "初十" == strings.Join(day, "") {
		dayDay = "10"
	} else if "二十" == strings.Join(day, "") {
		dayDay = "20"
	} else if "三十" == strings.Join(day, "") {
		dayDay = "30"
	} else {
		for i, v := range ten {
			if v == day[0] {
				dayDay = strconv.Itoa(i)
			}
		}
		for i, v := range number {
			if v == day[1] {
				dayDay += strconv.Itoa(i + 1)
			}
		}
	}

	yearDay := l.Solar.Year()
	solarMonth := int(l.Solar.Month())
	monthInt, _ := strconv.Atoi(monthDay)
	if solarMonth < monthInt {
		yearDay--
	}

	format = strings.Replace(format, "02", dayDay, 1)
	format = strings.Replace(format, "01", monthDay, 1)
	format = strings.Replace(format, "2006", strconv.Itoa(yearDay), 1)

	return format
}

func NewLunarDate(born chronos.Calendar) *LunarDate {
	var lunar LunarDate
	lunar.Solar = born.Solar().Time()
	lunarDate := born.LunarDate()
	strs := strings.Split(lunarDate, "")
	ss := ""
	for _, v := range strs {
		ss += v
		if v == "年" {
			lunar.Year = ss
			ss = ""
		}
		if v == "月" {
			lunar.Month = ss
			ss = ""
		}
		if v == "日" {
			lunar.Day = ss
			ss = ""
		}
		if v == "时" {
			lunar.Hour = ss
			ss = ""
		}
	}
	lunar.IsLeap = strings.Contains(lunarDate, "闰")
	hour := born.Solar().Time().Hour()
	if hour == 23 {
		hour = 0
	}
	lunar.Hour = hourIndex[int(math.Ceil(float64(hour)/2))]

	return &lunar
}
