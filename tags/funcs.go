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
func PriceFormat(in interface{}, args ...interface{}) string {
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
		format = fmt.Sprint(args[0])
		switch strings.ToLower(format) {
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
		}
	}

	return fmt.Sprintf(format, price)
}

// Range 实现 range(start, end, step) 功能
// Range 支持整数范围和字符范围
// Range 实现 range(start, end, step) 功能
// 支持整数范围和字符范围
func Range(params ...interface{}) []interface{} {
	// 解析参数
	paramCount := len(params)

	if paramCount < 1 || paramCount > 3 {
		return nil
	}

	// 默认值
	var start, stop interface{}
	var step int = 1

	switch paramCount {
	case 1:
		// 单个参数：可能是 stop（整数）或字符范围的结束值
		switch val := params[0].(type) {
		case int, int64, float64:
			start = 0
			stop = toInt(val)
		case string:
			if isChar(val) {
				start = 'a' // 默认从 'a' 开始
				stop = rune(val[0])
			} else {
				return nil // 参数非法
			}
		default:
			return nil // 参数非法
		}
	case 2:
		// 两个参数：start 和 stop
		startVal := params[0]
		stopVal := params[1]

		switch startVal.(type) {
		case int, int64, float64:
			if _, ok := stopVal.(int); !ok {
				return nil // 类型不匹配
			}
			start = toInt(startVal)
			stop = toInt(stopVal)
		case string:
			if stopStr, ok := stopVal.(string); ok && isChar(startVal.(string)) && isChar(stopStr) {
				start = rune(startVal.(string)[0])
				stop = rune(stopStr[0])
			} else {
				return nil // 参数非法
			}
		default:
			return nil // 参数非法
		}
	case 3:
		// 三个参数：start、stop 和 step
		startVal := params[0]
		stopVal := params[1]
		stepVal := params[2]

		// 提取 step
		step = toInt(stepVal)
		if step == 0 {
			return nil // step 非法
		}

		switch startVal.(type) {
		case int, int64, float64:
			if _, ok := stopVal.(int); !ok {
				return nil // 类型不匹配
			}
			start = toInt(startVal)
			stop = toInt(stopVal)
		case string:
			if stopStr, ok := stopVal.(string); ok && isChar(startVal.(string)) && isChar(stopStr) {
				start = rune(startVal.(string)[0])
				stop = rune(stopStr[0])
			} else {
				return nil // 参数非法
			}
		default:
			return nil // 参数非法
		}
	}

	// 生成范围
	result := make([]interface{}, 0)

	switch s := start.(type) {
	case int:
		// 整数范围
		end := stop.(int)
		for i := s; (step > 0 && i <= end) || (step < 0 && i > end); i += step {
			result = append(result, i)
		}
	case rune:
		// 字符范围
		end := stop.(rune)
		for ch := s; (step > 0 && ch <= end) || (step < 0 && ch > end); ch += rune(step) {
			result = append(result, string(ch))
		}
	}

	return result
}

// toInt 将 interface{} 转换为 int
func toInt(val interface{}) int {
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// isChar 判断字符串是否为单个字符
func isChar(s string) bool {
	return len(s) == 1 && ((s[0] >= 'a' && s[0] <= 'z' || s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= '0' && s[0] <= '9'))
}

type MyFunc struct {
}

func CustomFunc() *MyFunc {
	return &MyFunc{}
}
