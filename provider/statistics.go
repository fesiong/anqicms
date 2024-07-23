package provider

import (
	"encoding/json"
	"github.com/jinzhu/now"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
	"strconv"
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
			Select("count(1) AS total, FROM_UNIXTIME(created_time, '%H:00') AS statistic_date,spider").
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
			Select("count(1) AS total, count(distinct ip) as ips FROM_UNIXTIME(created_time, '%H:00') AS statistic_date").
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
		w.DB.Model(&model.ArchiveDraft{}).Where("created_time > ?", time.Now().Unix()).Count(&result.ArchiveCount.UnRelease)
		w.DB.Model(&model.ArchiveDraft{}).Where("status = 0").Count(&result.ArchiveCount.Draft)

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

func (w *Website) SendStatisticsMail() {
	setting := w.PluginSendmail
	if setting.Account == "" {
		//成功配置，则跳过
		return
	}
	// 开始统计数据
	// 需要发送以下数据
	// 各个搜索引擎收录数据
	// 访问数据
	todayStamp := now.BeginningOfDay().Unix()

	// 收录量
	var engineIndex model.SpiderInclude
	err := w.DB.Where("`created_time` >= ?", todayStamp-86400).Order("id desc").Take(&engineIndex).Error
	if err != nil {
		w.QuerySpiderInclude()
		// 重新查询
		w.DB.Where("`created_time` >= ?", todayStamp-86400).Order("id desc").Take(&engineIndex)
	}
	// 蜘蛛
	var spiderResult []*SpiderData
	var totalSpider int64
	w.DB.Model(&model.Statistic{}).Where("`created_time` >= ? and `created_time` < ?", todayStamp-86400, todayStamp).Where("`spider` != ''").
		Select("count(1) AS total, spider").
		Group("spider").Find(&spiderResult)
	for _, v := range spiderResult {
		totalSpider += v.Total
	}
	// 访问量
	var visitResult []*SpiderData
	var totalVisit int64
	w.DB.Model(&model.Statistic{}).Where("`created_time` >= ? and `created_time` < ?", todayStamp-86400, todayStamp).Where("`spider` = ''").
		Select("count(1) AS total, FROM_UNIXTIME(created_time, '%H:00') AS statistic_date").
		Group("statistic_date").Order("statistic_date asc").Find(&visitResult)
	for _, v := range visitResult {
		totalVisit += v.Total
	}
	// 文档等数据
	var archiveCount int64
	var allArchiveCount int64
	w.DB.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", todayStamp-86400, todayStamp).Count(&archiveCount)
	w.DB.Model(&model.Archive{}).Count(&allArchiveCount)
	var categoryCount int64
	w.DB.Model(&model.Category{}).Where("`type` != ?", config.CategoryTypePage).Count(&categoryCount)
	var pageCount int64
	w.DB.Model(&model.Category{}).Where("`type` = ?", config.CategoryTypePage).Count(&pageCount)
	var linkCount int64
	w.DB.Model(&model.Link{}).Count(&linkCount)
	var guestbookCount int64
	w.DB.Model(&model.Guestbook{}).Where("`created_time` >= ? and `created_time` < ?", todayStamp-86400, todayStamp).Count(&guestbookCount)
	var commentCount int64
	w.DB.Model(&model.Comment{}).Where("`created_time` >= ? and `created_time` < ?", todayStamp-86400, todayStamp).Count(&commentCount)
	var userCount int64
	w.DB.Model(&model.User{}).Where("`created_time` >= ? and `created_time` < ?", todayStamp-86400, todayStamp).Count(&userCount)
	// 后台操作记录
	var loginCount int64
	w.DB.Model(&model.AdminLoginLog{}).Where("`created_time` >= ? and `created_time` < ?", todayStamp-86400, todayStamp).Count(&loginCount)
	var adminLogCount int64
	w.DB.Model(&model.AdminLog{}).Where("`created_time` >= ? and `created_time` < ?", todayStamp-86400, todayStamp).Count(&adminLogCount)

	if w.SendTypeValid(SendTypeDaily) {
		// 开始写邮件内容
		subject := time.Now().Add(-86400*time.Second).Format("2006-01-02 ") + w.Tr("s(s)SiteData")
		content := `<html>
<head>
  <style>
    body {
      text-align: center;
      width: 90%;
      margin: 30px auto;
    }
    table {
      border-collapse: collapse;
      border-spacing: 0;
      width: 100%;
      background-color: #fff;
      color: #333
    }
    table tr {
      transition: all .3s;
      -webkit-transition: all .3s
    }
    table th {
      text-align: left;
      font-weight: 400
    }
    table tbody tr:hover,
    table thead tr {
      background-color: #FAFAFA
    }
    table td,
    table th {
      border-width: 1px;
      border-style: solid;
      border-color: #ccc
    }
    table td,
    table th {
      position: relative;
      padding: 9px 15px;
      min-height: 20px;
      line-height: 20px;
      font-size: 14px
    }
  </style>
</head>
<body>`
		content += "<h1>" + subject + "</h1>\n"
		content += "<h2>" + w.Tr("Inclusion") + "</h2>\n"
		content += `<table>
    <thead>
      <tr>`
		content += "<th>" + w.Tr("Baidu") + "</th>\n"
		content += "<th>" + w.Tr("Sogou") + "</th>\n"
		content += "<th>" + w.Tr("360") + "</th>\n"
		content += "<th>" + w.Tr("Bing") + "</th>\n"
		content += "<th>" + w.Tr("Google") + "</th>\n"
		content += `</tr>
    </thead>
    <tbody>
      <tr>`
		content += "<td>" + strconv.Itoa(engineIndex.BaiduCount) + "</td>\n" +
			"<td>" + strconv.Itoa(engineIndex.SogouCount) + "</td>\n" +
			"<td>" + strconv.Itoa(engineIndex.SoCount) + "</td>\n" +
			"<td>" + strconv.Itoa(engineIndex.BingCount) + "</td>\n" +
			"<td>" + strconv.Itoa(engineIndex.GoogleCount) + "</td>\n"
		content += `</tr>
    </tbody>
  </table>`
		content += "<h2>" + w.Tr("SpiderCrawling") + "</h2>\n"
		content += `<table>
    <thead>
      <tr>`
		for _, v := range spiderResult {
			content += "<th>" + v.Spider + "</th>"
		}
		content += `
      </tr>
    </thead>
    <tfoot>
      <tr>`
		content += "<td>" + w.Tr("Total") + "</td>\n"
		content += "<td colspan='" + strconv.Itoa(len(spiderResult)-1) + "'>" + strconv.Itoa(int(totalSpider)) + "</td>"
		content += `</tr>
    </tfoot>
    <tbody>
      <tr>`
		for _, v := range spiderResult {
			content += "<td>" + strconv.Itoa(int(v.Total)) + "</td>"
		}
		content += `
      </tr>
    </tbody>
  </table>`
		content += "<h2>" + w.Tr("Visits") + "</h2>"
		content += `<table>
    <thead>
      <tr>`
		content += "<th>" + w.Tr("Time") + "</th>"
		content += "<th>" + w.Tr("Visit") + "</th>"
		content += `</tr>
    </thead>
    <tfoot>
      <tr>`
		content += "<td>" + w.Tr("Total") + "</td>"
		content += "<td>" + strconv.Itoa(int(totalVisit)) + "</td>"
		content += `
      </tr>
    </tfoot>
    <tbody>`
		for i := 0; i < len(visitResult); i++ {
			content += "<tr>\n        <td>" + visitResult[i].StatisticDate + "</td>\n        <td>" + strconv.Itoa(int(visitResult[i].Total)) + "</td>\n</tr>"
		}
		content += `
    </tbody>
  </table>`
		content += "<h2>" + w.Tr("SiteClickData") + "</h2>"
		content += `<table>
    <thead>
      <tr>`
		content += "<th>" + w.Tr("Entry") + "</th>"
		content += "<th>" + w.Tr("Quantity") + "</th>"
		content += `</tr>
    </thead>
    <tbody>`
		content += "<tr>\n        <td>" + w.Tr("Document") + "</td>\n        <td>" + strconv.Itoa(int(allArchiveCount)) + "</td>\n      </tr>"
		content += "<tr>\n        <td>" + w.Tr("AddDocument") + "</td>\n        <td>" + strconv.Itoa(int(archiveCount)) + "</td>\n      </tr>"
		content += "<tr>\n        <td>" + w.Tr("Category") + "</td>\n        <td>" + strconv.Itoa(int(categoryCount)) + "</td>\n      </tr>"
		content += "<tr>\n        <td>" + w.Tr("SinglePage") + "</td>\n        <td>" + strconv.Itoa(int(pageCount)) + "</td>\n      </tr>"
		content += "<tr>\n        <td>" + w.Tr("FriendlyLink") + "</td>\n        <td>" + strconv.Itoa(int(linkCount)) + "</td>\n      </tr>"
		content += "<tr>\n        <td>" + w.Tr("AddMessage") + "</td>\n        <td>" + strconv.Itoa(int(guestbookCount)) + "</td>\n      </tr>"
		content += "<tr>\n        <td>" + w.Tr("AddComment") + "</td>\n        <td>" + strconv.Itoa(int(commentCount)) + "</td>\n      </tr>"
		content += "<tr>\n        <td>" + w.Tr("AddUser") + "</td>\n        <td>" + strconv.Itoa(int(userCount)) + "</td>\n      </tr>"
		content += "<tr>\n        <td>" + w.Tr("BackstageLogin") + "</td>\n        <td>" + strconv.Itoa(int(loginCount)) + "</td>\n      </tr>"
		content += "<tr>\n        <td>" + w.Tr("BackstageOperation") + "</td>\n        <td>" + strconv.Itoa(int(adminLogCount)) + "</td>\n      </tr>"
		content += `
    </tbody>
  </table>
</body>

</html>`

		// 不记录错误
		_ = w.sendMail(subject, content, nil, true, false)
	}
}
