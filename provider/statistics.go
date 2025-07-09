package provider

import (
	"encoding/json"
	"errors"
	"github.com/jinzhu/now"
	"gorm.io/gorm"
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

func (w *Website) StatisticSpider() []response.ChartData {
	var result []response.ChartData

	timeStamp := now.BeginningOfDay().AddDate(0, 0, -30).Unix()
	var tmpResult []*model.StatisticLog
	w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ?", timeStamp).Omit("visit_count").Order("created_time ASC").Find(&tmpResult)
	if len(tmpResult) == 0 {
		// 首次访问没有数据，则先尝试生成
		if w.StatisticLog != nil {
			w.StatisticLog.Calc(w.DB)
			// 再次查询
			w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ?", timeStamp).Omit("visit_count").Order("created_time ASC").Find(&tmpResult)
		}
	}

	for _, v := range tmpResult {
		vDate := time.Unix(v.CreatedTime, 0).Format("2006-01-02")
		for key, num := range v.SpiderCount {
			result = append(result,
				response.ChartData{
					Date:  vDate,
					Value: num,
					Label: key,
				})
		}
	}

	return result
}

// StatisticTraffic 增加IP
func (w *Website) StatisticTraffic() []response.ChartData {
	//支持按天，按小时区分
	var result []response.ChartData

	timeStamp := now.BeginningOfDay().AddDate(0, 0, -30).Unix()
	var tmpResult []*model.StatisticLog
	w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ?", timeStamp).Omit("spider_count").Order("created_time ASC").Find(&tmpResult)
	if len(tmpResult) == 0 {
		// 首次访问没有数据，则先尝试生成
		if w.StatisticLog != nil {
			w.StatisticLog.Calc(w.DB)
			// 再次查询
			w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ?", timeStamp).Omit("spider_count").Order("created_time ASC").Find(&tmpResult)
		}
	}
	for _, v := range tmpResult {
		vDate := time.Unix(v.CreatedTime, 0).Format("2006-01-02")
		result = append(result,
			response.ChartData{
				Date:  vDate,
				Value: v.VisitCount.PVCount,
				Label: "PV",
			}, response.ChartData{
				Date:  vDate,
				Value: v.VisitCount.IPCount,
				Label: "IP",
			})
	}

	return result
}

func (w *Website) GetStatisticDates() []string {
	if w.StatisticLog == nil {
		return nil
	}

	return w.StatisticLog.GetLogDates()
}

func (w *Website) StatisticDetail(filename string, currentPage, limit int) ([]*Statistic, int64, error) {
	if w.StatisticLog == nil {
		return nil, 0, errors.New("statistic log is not ready")
	}

	var statistics []*Statistic
	var total int64

	if limit < 1 {
		limit = 20
	}

	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * limit

	statistics, total = w.StatisticLog.Read(filename, offset, limit)

	return statistics, total, nil
}

func (w *Website) CleanStatistics() {
	//清理一个月前的记录
	if w.StatisticLog == nil {
		return
	}
	w.StatisticLog.Clear(false)
}

func (w *Website) GetStatisticsSummary(exact bool) *response.Statistics {
	var result = response.Statistics{}
	cacheKey := "cachedStatistics"
	err := w.Cache.Get(cacheKey, &result)
	if err != nil || exact {
		result = response.Statistics{}
		// 重新获取
		// 先检查文章总量是否超过10万
		explainCount := w.GetExplainCount("SELECT id FROM archives")
		if explainCount <= 100000 {
			exact = true
		}
		modules := w.GetCacheModules()
		for _, v := range modules {
			counter := response.ModuleCount{
				Id:   v.Id,
				Name: v.Title,
			}
			if exact {
				w.DB.Model(&model.Archive{}).Where("`module_id` = ?", v.Id).Count(&counter.Total)
			} else {
				toSql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
					return tx.Model(&model.Archive{}).Where("`module_id` = ?", v.Id).First(&model.Archive{})
				})
				counter.Total = w.GetExplainCount(toSql)
				if counter.Total <= 100000 {
					// 再次求取准确值
					w.DB.Model(&model.Archive{}).Where("`module_id` = ?", v.Id).Count(&counter.Total)
				}
			}
			result.ModuleCounts = append(result.ModuleCounts, counter)
			result.ArchiveCount.Total += counter.Total
		}
		lastWeek := now.BeginningOfWeek()
		today := now.BeginningOfDay()
		if exact {
			w.DB.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", lastWeek.AddDate(0, 0, -7).Unix(), lastWeek.Unix()).Count(&result.ArchiveCount.LastWeek)
		} else {
			toSql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", lastWeek.AddDate(0, 0, -7).Unix(), lastWeek.Unix()).First(&model.Archive{})
			})
			result.ArchiveCount.LastWeek = w.GetExplainCount(toSql)
			if result.ArchiveCount.LastWeek <= 100000 {
				// 再次求取准确值
				w.DB.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", lastWeek.AddDate(0, 0, -7).Unix(), lastWeek.Unix()).Count(&result.ArchiveCount.LastWeek)
			}
		}
		if exact {
			w.DB.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", today.Unix(), time.Now().Unix()).Count(&result.ArchiveCount.Today)
		} else {
			toSql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", today.Unix(), time.Now().Unix()).First(&model.Archive{})
			})
			result.ArchiveCount.Today = w.GetExplainCount(toSql)
			if result.ArchiveCount.Today <= 100000 {
				// 再次求取准确值
				w.DB.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", today.Unix(), time.Now().Unix()).Count(&result.ArchiveCount.Today)
			}
		}
		if exact {
			w.DB.Model(&model.ArchiveDraft{}).Where("created_time > ?", time.Now().Unix()).Count(&result.ArchiveCount.UnRelease)
		} else {
			toSql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.ArchiveDraft{}).Where("created_time > ?", time.Now().Unix()).First(&model.ArchiveDraft{})
			})
			result.ArchiveCount.UnRelease = w.GetExplainCount(toSql)
			if result.ArchiveCount.UnRelease <= 100000 {
				// 再次求取准确值
				w.DB.Model(&model.ArchiveDraft{}).Where("created_time > ?", time.Now().Unix()).Count(&result.ArchiveCount.UnRelease)
			}
		}
		if exact {
			w.DB.Model(&model.ArchiveDraft{}).Where("status = 0").Count(&result.ArchiveCount.Draft)
		} else {
			toSql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.ArchiveDraft{}).Where("status = 0").First(&model.ArchiveDraft{})
			})
			result.ArchiveCount.Draft = w.GetExplainCount(toSql)
			if result.ArchiveCount.Draft <= 100000 {
				// 再次求取准确值
				w.DB.Model(&model.ArchiveDraft{}).Where("status = 0").Count(&result.ArchiveCount.Draft)
			}
		}
		w.DB.Model(&model.Category{}).Where("`type` != ?", config.CategoryTypePage).Count(&result.CategoryCount)
		w.DB.Model(&model.Link{}).Count(&result.LinkCount)
		w.DB.Model(&model.Guestbook{}).Count(&result.GuestbookCount)
		designList := w.GetDesignList()
		result.TemplateCount = int64(len(designList))
		w.DB.Model(&model.Category{}).Where("`type` = ?", config.CategoryTypePage).Count(&result.PageCount)
		w.DB.Model(&model.Attachment{}).Count(&result.AttachmentCount)

		timeStamp := now.BeginningOfDay().AddDate(0, 0, -7).Unix()
		var tmpResult []*model.StatisticLog
		w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ?", timeStamp).Order("created_time ASC").Find(&tmpResult)

		for i, v := range tmpResult {
			result.TrafficCount.Total += int64(v.VisitCount.PVCount)
			result.SpiderCount.Total += calcSpider(v.SpiderCount)
			if i == len(tmpResult)-1 && v.CreatedTime == today.Unix() {
				result.TrafficCount.Today = int64(v.VisitCount.PVCount)
				result.SpiderCount.Total = calcSpider(v.SpiderCount)
			}
		}

		var lastInclude model.SpiderInclude
		w.DB.Model(&model.SpiderInclude{}).Order("id desc").Take(&lastInclude)
		result.IncludeCount = lastInclude

		result.CacheTime = time.Now().Unix()

		// 安装时间
		var installTime int64
		_ = json.Unmarshal([]byte(w.GetSettingValue(InstallTimeKey)), &installTime)
		// show guide 安装的第一天，还没设置站点名称，还没创建分类，没有发布文章，则show guide
		result.ShowGuide = (installTime+86400) > time.Now().Unix() || result.CategoryCount == 0 || result.ArchiveCount.Total == 0 || len(w.System.SiteName) == 0
		result.Exact = exact
		// 写入缓存，并缓存60秒
		w.Cache.Set(cacheKey, result, 60)
	}

	return &result
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
	var statisticResult model.StatisticLog
	w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ? and `created_time` < ?", todayStamp-86400, todayStamp).Take(&statisticResult)
	var totalSpider int64
	var spiderResult []*SpiderData
	if statisticResult.SpiderCount != nil {
		totalSpider = calcSpider(statisticResult.SpiderCount)
		for key, num := range statisticResult.SpiderCount {
			spiderResult = append(spiderResult, &SpiderData{
				Spider: key,
				Total:  int64(num),
			})
		}
	}

	// 访问量
	var totalVisit = statisticResult.VisitCount.PVCount
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
		subject := time.Now().Add(-86400*time.Second).Format("2006-01-02 ") + w.Tr("s(s)SiteData", w.System.SiteName, w.System.BaseUrl)
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

func calcSpider(data map[string]int) int64 {
	var count int
	for _, v := range data {
		count += v
	}

	return int64(count)
}
