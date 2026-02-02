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
		// 计算时间差，如果是过去时间则显示已过去多久，如果是未来时间则显示还剩多久
		var diff time.Duration
		var isFuture bool
		if t.After(now) {
			// 未来时间，计算剩余时间
			diff = t.Sub(now)
			isFuture = true
		} else {
			// 过去时间，计算已过去时间
			diff = now.Sub(t)
			isFuture = false
		}

		var result string
		// 多语言支持
		var hourStr, minuteStr, secondStr, justStr string
		var daysAfterStr, weekAfterStr, monthAfterStr, yearAfterStr string
		var daysAgoStr, weekAgoStr, monthAgoStr, yearAgoStr string

		// 默认中文
		if isFuture {
			hourStr = "小时后"
			minuteStr = "分钟后"
			secondStr = "秒后"
			justStr = "马上到期"

			daysAfterStr = "天后"
			weekAfterStr = "周后"
			monthAfterStr = "月后"
			yearAfterStr = "年后"
		} else {
			hourStr = "小时前"
			minuteStr = "分钟前"
			secondStr = "秒前"
			justStr = "刚刚"

			daysAgoStr = "天前"
			weekAgoStr = "周前"
			monthAgoStr = "月前"
			yearAgoStr = "年前"
		}

		// 其它按英文处理
		if !strings.HasPrefix(lang, "zh") {
			if isFuture {
				hourStr = " hours remaining"
				minuteStr = " minutes remaining"
				secondStr = " seconds remaining"
				justStr = " Imminent"

				daysAfterStr = " days remaining"
				weekAfterStr = " weeks remaining"
				monthAfterStr = " months remaining"
				yearAfterStr = " years remaining"
			} else {
				hourStr = " hours ago"
				minuteStr = " minutes ago"
				secondStr = " seconds ago"
				justStr = " Just now"

				daysAgoStr = " days ago"
				weekAgoStr = " weeks ago"
				monthAgoStr = " months ago"
				yearAgoStr = " years ago"
			}
		}

		if diff < 1*time.Second && isFuture {
			return justStr
		} else if diff < time.Minute {
			seconds := int(diff.Seconds())
			result = fmt.Sprintf("%d%s", seconds, secondStr)
		} else if diff < time.Hour {
			minutes := int(diff.Minutes())
			result = fmt.Sprintf("%d%s", minutes, minuteStr)
		} else if diff < 24*time.Hour {
			hours := int(diff.Hours())
			result = fmt.Sprintf("%d%s", hours, hourStr)
		} else if diff < 7*24*time.Hour {
			// 不足一周，显示天数
			days := int(diff.Hours()) / 24
			result = fmt.Sprintf("%d%s", days, func() string {
				if isFuture {
					return daysAfterStr
				} else {
					return daysAgoStr
				}
			}())
		} else if diff < 30*24*time.Hour {
			// 不足一个月，显示周数
			weeks := int(diff.Hours()) / 24 / 7
			result = fmt.Sprintf("%d%s", weeks, func() string {
				if isFuture {
					return weekAfterStr
				} else {
					return weekAgoStr
				}
			}())
		} else if diff < 365*24*time.Hour {
			// 不足一年，显示月数
			months := int(diff.Hours()) / 24 / 30
			result = fmt.Sprintf("%d%s", months, func() string {
				if isFuture {
					return monthAfterStr
				} else {
					return monthAgoStr
				}
			}())
		} else {
			// 显示年数
			years := int(diff.Hours()) / 24 / 365
			result = fmt.Sprintf("%d%s", years, func() string {
				if isFuture {
					return yearAfterStr
				} else {
					return yearAgoStr
				}
			}())
		}

		return result
	}
	return t.Format(layout)
}

// 价格格式化，输入的是分
func PriceFormat(in interface{}, args ...string) string {
	in2, _ := strconv.ParseInt(fmt.Sprint(in), 10, 64)
	if in2 == 0 {
		return ""
	}
	// 将分转换为元（除以100）
	price := float64(in2) / 100.0

	// 默认格式为保留两位小数
	format := "%.2f"

	// 如果提供了格式参数，则使用提供的格式
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "int", "integer", "0":
			// 只显示整数部分
			return fmt.Sprintf("%.0f", price)
		case "one", "1":
			// 保留一位小数
			return fmt.Sprintf("%.1f", price)
		case "two", "2":
			// 保留两位小数（默认）
			return fmt.Sprintf("%.2f", price)
		default:
			// 使用自定义格式
			format = args[0]
		}
	}

	return fmt.Sprintf(format, price)
}

type MyFunc struct {
}

func CustomFunc() *MyFunc {
	return &MyFunc{}
}
