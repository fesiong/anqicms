package manageController

import (
	"time"

	"github.com/jinzhu/now"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

// StatisticSpider 获取蜘蛛爬行情况统计图表数据。
func StatisticSpider(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	result := currentSite.StatisticSpider()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// StatisticTraffic 获取当前站点的流量统计概览图表数据。
func StatisticTraffic(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	result := currentSite.StatisticTraffic()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// StatisticDates 获取当前站点可用的流量统计日期列表。
func StatisticDates(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	result := currentSite.GetStatisticDates()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// StatisticDetail 获取指定日期和类型的流量统计明细列表，支持分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "date": 统计日期。
//   - 查询参数 "type": 统计类型：spider=蜘蛛爬行情况，traffic=流量统计。
func StatisticDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	date := ctx.URLParam("date")
	searchType := ctx.URLParam("type")

	list, total, _ := currentSite.StatisticDetail(date, searchType, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  list,
	})
}

// GetSpiderIncludeDetail 获取搜索引擎蜘蛛收录明细列表，支持分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
func GetSpiderIncludeDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	var list []*model.SpiderInclude
	var total int64

	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize

	builder := currentSite.DB.Model(&model.SpiderInclude{})

	builder.Count(&total).Limit(pageSize).Offset(offset).Order("`id` desc").Find(&list)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  list,
	})
}

// GetSpiderInclude 获取搜索引擎蜘蛛收录统计图表数据。
func GetSpiderInclude(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var result = make([]response.ChartData, 0, 30*5)

	timeStamp := now.BeginningOfDay().AddDate(0, 0, -30).Unix()

	var includeLogs []model.SpiderInclude
	currentSite.DB.Model(&model.SpiderInclude{}).Where("`created_time` >= ?", timeStamp).
		Order("created_time asc").
		Scan(&includeLogs)

	lastDate := ""
	for _, v := range includeLogs {
		date := time.Unix(v.CreatedTime, 0).Format("01-02")
		if date == lastDate {
			continue
		}
		lastDate = date
		result = append(result, response.ChartData{
			Date:  date,
			Label: ctx.Tr("Baidu"),
			Value: v.BaiduCount,
		}, response.ChartData{
			Date:  date,
			Label: ctx.Tr("Sogou"),
			Value: v.SogouCount,
		}, response.ChartData{
			Date:  date,
			Label: ctx.Tr("Soso"),
			Value: v.SoCount,
		}, response.ChartData{
			Date:  date,
			Label: ctx.Tr("Bing"),
			Value: v.BingCount,
		}, response.ChartData{
			Date:  date,
			Label: ctx.Tr("Google"),
			Value: v.GoogleCount,
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}
