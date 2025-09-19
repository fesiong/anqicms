package tags

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func TimestampToDate(in interface{}, layout string, args ...string) string {
	in2, _ := strconv.ParseInt(fmt.Sprint(in), 10, 64)
	if in2 == 0 {
		return ""
	}
	t := time.Unix(in2, 0)
	if layout == "diff" {
		now := time.Now()
		diff := now.Sub(t)
		diffType := ""
		if len(args) > 0 {
			diffType = strings.ToLower(args[0])
		}
		var result string
		if diffType == "year" {
			result = strconv.Itoa(now.Year() - t.Year())
		} else if diffType == "month" {
			result = strconv.Itoa(int(diff.Hours()) / 24 / 30)
		} else if diffType == "week" {
			result = strconv.Itoa(int(diff.Hours()) / 24 / 7)
		} else if diffType == "day" {
			result = strconv.Itoa(int(diff.Hours()) / 24)
		} else if diffType == "hour" {
			result = strconv.Itoa(int(diff.Hours()))
		} else if diffType == "minute" {
			result = strconv.Itoa(int(diff.Minutes()))
		} else {
			result = strconv.FormatInt(int64(diff.Seconds()), 10)
		}
		return result
	} else if layout == "friendly" {
		lang := "zh"
		if len(args) > 0 {
			lang = strings.ToLower(args[0])
		}
		now := time.Now()
		diff := now.Sub(t)

		var result string
		// 多语言支持
		var hourStr, minuteStr, secondStr, justStr string
		var todayStr, yesterdayStr, daysAgoStr, weekAgoStr, monthAgoStr, yearAgoStr string

		// 默认中文
		hourStr = "小时前"
		minuteStr = "分钟前"
		secondStr = "秒前"
		justStr = "刚刚"

		todayStr = "今天"
		yesterdayStr = "昨天"
		daysAgoStr = "天前"
		weekAgoStr = "周前"
		monthAgoStr = "月前"
		yearAgoStr = "年前"
		// 其它按英文处理
		if !strings.HasPrefix(lang, "zh") {
			hourStr = " hour ago"
			minuteStr = " minute ago"
			secondStr = " second ago"
			justStr = " Just"

			todayStr = "Today"
			yesterdayStr = "Yesterday"
			daysAgoStr = " days ago"
			weekAgoStr = " weeks ago"
			monthAgoStr = " months ago"
			yearAgoStr = " years ago"
		}

		if diff < 1 {
			return justStr
		} else if diff < time.Minute {
			seconds := int(diff.Seconds())
			result = fmt.Sprintf("%d%s", seconds, secondStr)
		} else if diff < time.Hour {
			minutes := int(diff.Minutes())
			result = fmt.Sprintf("%d%s", minutes, minuteStr)
		} else if diff < 10*time.Hour {
			hours := int(diff.Hours())
			result = fmt.Sprintf("%d%s", hours, hourStr)
		} else if t.YearDay() == now.YearDay() {
			result = fmt.Sprintf("%s %s", todayStr, t.Format("15:04"))
		} else if t.YearDay()+1 == now.YearDay() && t.Year() == now.Year() {
			result = yesterdayStr
		} else if t.YearDay()+7 > now.YearDay() && t.Year() == now.Year() {
			days := now.Day() - t.Day()
			if days < 0 {
				days = -days
			}
			result = fmt.Sprintf("%d%s", days, daysAgoStr)
		} else if t.Month() == now.Month() && t.Year() == now.Year() {
			weeks := int(diff.Hours() / 24 / 7)
			if weeks == 0 {
				weeks = 1
			}
			result = fmt.Sprintf("%d%s", weeks, weekAgoStr)
		} else if t.Year() == now.Year() {
			months := int(now.Month() - t.Month())
			if months < 0 {
				months = -months
			}
			result = fmt.Sprintf("%d%s", months, monthAgoStr)
		} else {
			years := now.Year() - t.Year()
			if years <= 1 {
				months := 12 - int(t.Month()) + int(now.Month())
				if months <= 12 {
					return fmt.Sprintf("%d%s", months, monthAgoStr)
				}
			}
			result = fmt.Sprintf("%d%s", years, yearAgoStr)
		}

		return result
	}
	return t.Format(layout)
}

type MyFunc struct {
}

func CustomFunc() *MyFunc {
	return &MyFunc{}
}
