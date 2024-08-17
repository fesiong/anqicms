package manageController

import (
	"github.com/jinzhu/now"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"time"
)

// StatisticSpider 蜘蛛爬行情况
func StatisticSpider(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	//支持按天，按小时区分
	separate := ctx.URLParam("separate")

	result := currentSite.StatisticSpider(separate)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

func StatisticTraffic(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	//支持按天，按小时区分
	separate := ctx.URLParam("separate")

	result := currentSite.StatisticTraffic(separate)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

func StatisticDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	isSpider, _ := ctx.URLParamBool("is_spider")

	list, total, _ := currentSite.StatisticDetail(isSpider, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  list,
	})
}

func GetSpiderIncludeDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

func GetSpiderInclude(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
