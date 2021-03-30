package provider

import (
	"github.com/jinzhu/now"
	"irisweb/config"
	"irisweb/model"
	"time"
)

type SpiderData struct {
	Total         int64  `json:"total"`
	StatisticDate string `json:"statistic_date"`
}

func StatisticSpider(separate string) []*SpiderData {
	//支持按天，按小时区分
	var timeStamps []int64

	var result []*SpiderData

	if separate == "hour" {
		//按小时展示，展示24小时
		todayStamp := now.BeginningOfDay().Unix()
		nowHour := now.BeginningOfHour().Hour()
		for i := 0; i <= nowHour; i++ {
			timeStamps = append(timeStamps, todayStamp + int64(i) * 3600)
		}
		var tmpResult []*SpiderData
		config.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", todayStamp).Where("`spider` != ''").
			Select("count(1) AS total, FROM_UNIXTIME(created_time, '%h:00') AS statistic_date").
			Group("statistic_date").Find(&tmpResult)

		for _, v := range timeStamps {
			formatTime := time.Unix(v, 0).Format("15:04")
			item := &SpiderData{StatisticDate: formatTime}
			for _, s := range tmpResult {
				if s.StatisticDate == formatTime {
					item = s
					break
				}
			}
			result = append(result, item)
		}
	} else {
		//其他情况，按天展示，展示15天
		currTimeStamp := now.BeginningOfDay().Unix()
		for i := 15; i >= 0; i-- {
			timeStamps = append(timeStamps, currTimeStamp-int64(i)*86400)
		}
		var tmpResult []*SpiderData
		config.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", timeStamps[0]).Where("`spider` != ''").
			Select("count(1) AS total, FROM_UNIXTIME(created_time, '%m-%d') AS statistic_date").
			Group("statistic_date").Find(&tmpResult)

		for _, v := range timeStamps {
			formatTime := time.Unix(v, 0).Format("01-02")
			item := &SpiderData{StatisticDate: formatTime}
			for _, s := range tmpResult {
				if s.StatisticDate == formatTime {
					item = s
					break
				}
			}
			result = append(result, item)
		}
	}

	return result
}

func StatisticTraffic(separate string) []*SpiderData {
	//支持按天，按小时区分
	var timeStamps []int64

	var result []*SpiderData

	if separate == "hour" {
		//按小时展示，展示24小时
		todayStamp := now.BeginningOfDay().Unix()
		nowHour := now.BeginningOfHour().Hour()
		for i := 0; i <= nowHour; i++ {
			timeStamps = append(timeStamps, todayStamp + int64(i) * 3600)
		}
		var tmpResult []*SpiderData
		config.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", todayStamp).Where("`spider` = ''").
			Select("count(1) AS total, FROM_UNIXTIME(created_time, '%h:00') AS statistic_date").
			Group("statistic_date").Find(&tmpResult)

		for _, v := range timeStamps {
			formatTime := time.Unix(v, 0).Format("15:04")
			item := &SpiderData{StatisticDate: formatTime}
			for _, s := range tmpResult {
				if s.StatisticDate == formatTime {
					item = s
					break
				}
			}
			result = append(result, item)
		}
	} else {
		//其他情况，按天展示，展示15天
		currTimeStamp := now.BeginningOfDay().Unix()
		for i := 15; i >= 0; i-- {
			timeStamps = append(timeStamps, currTimeStamp-int64(i)*86400)
		}
		var tmpResult []*SpiderData
		config.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", timeStamps[0]).Where("`spider` = ''").
			Select("count(1) AS total, FROM_UNIXTIME(created_time, '%m-%d') AS statistic_date").
			Group("statistic_date").Find(&tmpResult)

		for _, v := range timeStamps {
			formatTime := time.Unix(v, 0).Format("01-02")
			item := &SpiderData{StatisticDate: formatTime}
			for _, s := range tmpResult {
				if s.StatisticDate == formatTime {
					item = s
					break
				}
			}
			result = append(result, item)
		}
	}

	return result
}

func StatisticDetail(isSpider bool, currentPage, limit int) ([]*model.Statistic, int64, error) {
	var statistics []*model.Statistic
	var total int64

	if limit < 1 {
		limit = 20
	}

	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * limit

	builder := config.DB.Model(&model.Statistic{})
	if isSpider {
		builder = builder.Where("`spider` != ''")
	}

	builder.Count(&total).Limit(limit).Offset(offset).Order("`id` desc").Find(&statistics)

	return statistics, total, nil
}