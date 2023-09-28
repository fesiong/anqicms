package provider

import (
	"encoding/json"
	"github.com/jinzhu/now"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
	"time"
)

type SpiderData struct {
	Total         int64  `json:"total"`
	Ips           int64  `json:"ips"`
	StatisticDate string `json:"statistic_date"`
	Spider        string
}

func (w *Website) StatisticSpider(separate string) []response.ChartData {
	//支持按天，按小时区分
	var result []response.ChartData

	if separate == "hour" {
		//按小时展示，展示24小时
		todayStamp := now.BeginningOfDay().Unix()
		var tmpResult []*SpiderData
		w.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", todayStamp).Where("`spider` != ''").
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
		w.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", timeStamp).Where("`spider` != ''").
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
func (w *Website) StatisticTraffic(separate string) []response.ChartData {
	//支持按天，按小时区分
	var result []response.ChartData

	if separate == "hour" {
		//按小时展示，展示24小时
		todayStamp := now.BeginningOfDay().Unix()
		var tmpResult []*SpiderData
		w.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", todayStamp).Where("`spider` = ''").
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
		timeStamp := now.BeginningOfDay().AddDate(0, 0, -30).Unix()
		var tmpResult []*SpiderData
		w.DB.Model(&model.Statistic{}).Where("`created_time` >= ?", timeStamp).Where("`spider` = ''").
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

func (w *Website) StatisticDetail(isSpider bool, currentPage, limit int) ([]*model.Statistic, int64, error) {
	var statistics []*model.Statistic
	var total int64

	if limit < 1 {
		limit = 20
	}

	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * limit

	builder := w.DB.Model(&model.Statistic{})
	if isSpider {
		builder = builder.Where("`spider` != ''")
	}

	builder.Count(&total).Limit(limit).Offset(offset).Order("`id` desc").Find(&statistics)

	return statistics, total, nil
}

func (w *Website) CleanStatistics() {
	//清理一个月前的记录
	agoStamp := time.Now().AddDate(0, 0, -30).Unix()
	w.DB.Unscoped().Where("`created_time` < ?", agoStamp).Delete(model.Statistic{})
}

func (w *Website) GetStatisticsSummary() *response.Statistics {
	var result = &response.Statistics{}
	if w.CachedStatistics == nil || w.CachedStatistics.CacheTime < time.Now().Add(-60*time.Second).Unix() {
		modules := w.GetCacheModules()
		for _, v := range modules {
			counter := response.ModuleCount{
				Id:   v.Id,
				Name: v.Title,
			}
			w.DB.Model(&model.Archive{}).Where("`module_id` = ?", v.Id).Count(&counter.Total)
			result.ModuleCounts = append(result.ModuleCounts, counter)
			result.ArchiveCount.Total += counter.Total
		}
		lastWeek := now.BeginningOfWeek()
		today := now.BeginningOfDay()
		w.DB.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", lastWeek.AddDate(0, 0, -7).Unix(), lastWeek.Unix()).Count(&result.ArchiveCount.LastWeek)
		w.DB.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", today.Unix(), time.Now().Unix()).Count(&result.ArchiveCount.Today)
		w.DB.Model(&model.Archive{}).Where("created_time > ?", time.Now().Unix()).Count(&result.ArchiveCount.UnRelease)

		w.DB.Model(&model.Category{}).Where("`type` != ?", config.CategoryTypePage).Count(&result.CategoryCount)
		w.DB.Model(&model.Link{}).Count(&result.LinkCount)
		w.DB.Model(&model.Guestbook{}).Count(&result.GuestbookCount)
		designList := w.GetDesignList()
		result.TemplateCount = int64(len(designList))
		w.DB.Model(&model.Category{}).Where("`type` = ?", config.CategoryTypePage).Count(&result.PageCount)
		w.DB.Model(&model.Attachment{}).Count(&result.AttachmentCount)

		w.DB.Model(&model.Statistic{}).Where("`spider` = '' and `created_time` >= ?", time.Now().AddDate(0, 0, -7).Unix()).Count(&result.TrafficCount.Total)
		w.DB.Model(&model.Statistic{}).Where("`spider` = '' and `created_time` >= ?", today.Unix()).Count(&result.TrafficCount.Today)

		w.DB.Model(&model.Statistic{}).Where("`spider`!= '' and `created_time` >= ?", time.Now().AddDate(0, 0, -7).Unix()).Count(&result.SpiderCount.Total)
		w.DB.Model(&model.Statistic{}).Where("`spider` != '' and `created_time` >= ?", today.Unix()).Count(&result.SpiderCount.Today)

		var lastInclude model.SpiderInclude
		w.DB.Model(&model.SpiderInclude{}).Order("id desc").Take(&lastInclude)
		result.IncludeCount = lastInclude

		result.CacheTime = time.Now().Unix()

		// 安装时间
		var installTime int64
		_ = json.Unmarshal([]byte(w.GetSettingValue(InstallTimeKey)), &installTime)
		// show guide 安装的第一天，还没设置站点名称，还没创建分类，没有发布文章，则show guide
		result.ShowGuide = (installTime+86400) > time.Now().Unix() || result.CategoryCount == 0 || result.ArchiveCount.Total == 0 || len(w.System.SiteName) == 0

		w.CachedStatistics = result
	}

	return w.CachedStatistics
}
