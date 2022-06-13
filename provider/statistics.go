package provider

import (
	"github.com/jinzhu/now"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
)

type SpiderData struct {
	Total         int64  `json:"total"`
	Ips           int64  `json:"ips"`
	StatisticDate string `json:"statistic_date"`
	Spider        string
}

func StatisticSpider(separate string) []response.ChartData {
	//支持按天，按小时区分
	var result []response.ChartData

	if separate == "hour" {
		//按小时展示，展示24小时
		todayStamp := now.BeginningOfDay().Unix()
		var tmpResult []*SpiderData
		dao.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", todayStamp).Where("`spider` != ''").
			Select("count(1) AS total, FROM_UNIXTIME(created_time, '%h:00') AS statistic_date,spider").
			Group("statistic_date,spider").Order("statistic_date asc").Find(&tmpResult)

		for _, v := range tmpResult {
			result = append(result,
				response.ChartData{
					Date:  v.StatisticDate,
					Value: int(v.Total),
					Label: v.Spider,
				})
		}
	} else {
		//其他情况，按天展示，展示30天
		timeStamp := now.BeginningOfDay().AddDate(0, 0, -30).Unix()
		var tmpResult []*SpiderData
		dao.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", timeStamp).Where("`spider` != ''").
			Select("count(1) AS total, FROM_UNIXTIME(created_time, '%m-%d') AS statistic_date,spider").
			Group("statistic_date,spider").Order("statistic_date asc").Find(&tmpResult)

		for _, v := range tmpResult {
			result = append(result,
				response.ChartData{
					Date:  v.StatisticDate,
					Value: int(v.Total),
					Label: v.Spider,
				})
		}
	}

	return result
}

// StatisticTraffic 增加IP
func StatisticTraffic(separate string) []response.ChartData {
	//支持按天，按小时区分
	var result []response.ChartData

	if separate == "hour" {
		//按小时展示，展示24小时
		todayStamp := now.BeginningOfDay().Unix()
		var tmpResult []*SpiderData
		dao.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", todayStamp).Where("`spider` = ''").
			Select("count(1) AS total, count(distinct ip) as ips FROM_UNIXTIME(created_time, '%h:00') AS statistic_date").
			Group("statistic_date").Order("statistic_date asc").Find(&tmpResult)

		for _, v := range tmpResult {
			result = append(result,
				response.ChartData{
					Date:  v.StatisticDate,
					Value: int(v.Total),
					Label: "PV",
				}, response.ChartData{
					Date:  v.StatisticDate,
					Value: int(v.Ips),
					Label: "IP",
				})
		}
	} else {
		//其他情况，按天展示，展示30天
		timeStamp := now.BeginningOfDay().AddDate(0,0,-30).Unix()
		var tmpResult []*SpiderData
		dao.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", timeStamp).Where("`spider` = ''").
			Select("count(1) AS total, count(distinct ip) as ips, FROM_UNIXTIME(created_time, '%m-%d') AS statistic_date").
			Group("statistic_date").Order("statistic_date asc").Find(&tmpResult)

		for _, v := range tmpResult {
			result = append(result,
				response.ChartData{
					Date:  v.StatisticDate,
					Value: int(v.Total),
					Label: "PV",
				}, response.ChartData{
					Date:  v.StatisticDate,
					Value: int(v.Ips),
					Label: "IP",
				})
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

	builder := dao.DB.Model(&model.Statistic{})
	if isSpider {
		builder = builder.Where("`spider` != ''")
	}

	builder.Count(&total).Limit(limit).Offset(offset).Order("`id` desc").Find(&statistics)

	return statistics, total, nil
}
