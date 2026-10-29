package provider

import (
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/jinzhu/now"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
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
	w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ?", timeStamp).Omit("spider_count", "shop_count").Order("created_time ASC").Find(&tmpResult)
	if len(tmpResult) == 0 {
		// 首次访问没有数据，则先尝试生成
		if w.StatisticLog != nil {
			w.StatisticLog.Calc(w.DB)
			// 再次查询
			w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ?", timeStamp).Omit("spider_count", "shop_count").Order("created_time ASC").Find(&tmpResult)
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

func (w *Website) StatisticDetail(filename string, searchType string, currentPage, limit int) ([]*Statistic, int64, error) {
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

	statistics, total = w.StatisticLog.Read(filename, searchType, offset, limit)

	return statistics, total, nil
}

func (w *Website) CleanStatistics() {
	// 删除 searchLog
	w.DB.Where("created_time < ?", now.BeginningOfDay().Add(-30*24*time.Hour).Unix()).Delete(&model.SearchLog{})

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
		module := w.GetModuleFromCache(2)
		counter := response.ModuleCount{
			Id:   module.Id,
			Name: module.Title,
		}
		if exact {
			w.DB.Model(&model.Archive{}).Where("`module_id` = ?", module.Id).Count(&counter.Total)
		} else {
			toSql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.Archive{}).Where("`module_id` = ?", module.Id).First(&model.Archive{})
			})
			counter.Total = w.GetExplainCount(toSql)
			if counter.Total <= 100000 {
				// 再次求取准确值
				w.DB.Model(&model.Archive{}).Where("`module_id` = ?", module.Id).Count(&counter.Total)
			}
		}
		result.ModuleCounts = append(result.ModuleCounts, counter)
		result.ArchiveCount.Total += counter.Total
		lastWeek := now.BeginningOfWeek()
		today := now.BeginningOfDay()
		if exact {
			w.DB.Model(&model.Archive{}).Where("module_id = ? and created_time >= ? and created_time < ?", module.Id, lastWeek.AddDate(0, 0, -7).Unix(), lastWeek.Unix()).Count(&result.ArchiveCount.LastWeek)
		} else {
			toSql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.Archive{}).Where("module_id = ? and created_time >= ? and created_time < ?", module.Id, lastWeek.AddDate(0, 0, -7).Unix(), lastWeek.Unix()).First(&model.Archive{})
			})
			result.ArchiveCount.LastWeek = w.GetExplainCount(toSql)
			if result.ArchiveCount.LastWeek <= 100000 {
				// 再次求取准确值
				w.DB.Model(&model.Archive{}).Where("module_id = ? and created_time >= ? and created_time < ?", module.Id, lastWeek.AddDate(0, 0, -7).Unix(), lastWeek.Unix()).Count(&result.ArchiveCount.LastWeek)
			}
		}
		if exact {
			w.DB.Model(&model.Archive{}).Where("module_id = ? and created_time >= ? and created_time < ?", module.Id, today.Unix(), time.Now().Unix()).Count(&result.ArchiveCount.Today)
		} else {
			toSql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.Archive{}).Where("module_id = ? and created_time >= ? and created_time < ?", module.Id, today.Unix(), time.Now().Unix()).First(&model.Archive{})
			})
			result.ArchiveCount.Today = w.GetExplainCount(toSql)
			if result.ArchiveCount.Today <= 100000 {
				// 再次求取准确值
				w.DB.Model(&model.Archive{}).Where("module_id = ? and created_time >= ? and created_time < ?", module.Id, today.Unix(), time.Now().Unix()).Count(&result.ArchiveCount.Today)
			}
		}
		if exact {
			w.DB.Model(&model.ArchiveDraft{}).Where("module_id = ? and created_time > ?", module.Id, time.Now().Unix()).Count(&result.ArchiveCount.UnRelease)
		} else {
			toSql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.ArchiveDraft{}).Where("module_id = ? and created_time > ?", module.Id, time.Now().Unix()).First(&model.ArchiveDraft{})
			})
			result.ArchiveCount.UnRelease = w.GetExplainCount(toSql)
			if result.ArchiveCount.UnRelease <= 100000 {
				// 再次求取准确值
				w.DB.Model(&model.ArchiveDraft{}).Where("module_id = ? and created_time > ?", module.Id, time.Now().Unix()).Count(&result.ArchiveCount.UnRelease)
			}
		}
		if exact {
			w.DB.Model(&model.ArchiveDraft{}).Where("module_id = ? and status = 0", module.Id).Count(&result.ArchiveCount.Draft)
		} else {
			toSql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&model.ArchiveDraft{}).Where("module_id = ? and status = 0", module.Id).First(&model.ArchiveDraft{})
			})
			result.ArchiveCount.Draft = w.GetExplainCount(toSql)
			if result.ArchiveCount.Draft <= 100000 {
				// 再次求取准确值
				w.DB.Model(&model.ArchiveDraft{}).Where("module_id = ? and status = 0", module.Id).Count(&result.ArchiveCount.Draft)
			}
		}
		w.DB.Model(&model.Category{}).Where("module_id = ? and `type` != ?", module.Id, config.CategoryTypePage).Count(&result.CategoryCount)
		w.DB.Model(&model.Link{}).Count(&result.LinkCount)
		w.DB.Model(&model.Guestbook{}).Count(&result.GuestbookCount)
		designList := w.GetDesignList()
		result.TemplateCount = int64(len(designList))
		w.DB.Model(&model.Category{}).Where("module_id = ? and `type` = ?", module.Id, config.CategoryTypePage).Count(&result.PageCount)
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

func (w *Website) StatisticCoupon(startTime int64, endTime int64) []response.ChartData {
	// 优惠券领取数量
	// 优惠券使用数量
	var tmpResult []response.ChartData
	w.DB.Model(&model.CouponCode{}).Where("get_time >= ? and get_time < ?", startTime, endTime).Select("DATE(FROM_UNIXTIME(get_time)) AS `date`, count(*) as `value`, 'gain' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult)

	var tmpResult2 []response.ChartData
	w.DB.Model(&model.CouponCode{}).Where("use_time >= ? and use_time < ?", startTime, endTime).Select("DATE(FROM_UNIXTIME(use_time)) AS `date`, count(*) as `value`, 'used' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult2)

	return append(tmpResult, tmpResult2...)
}

func (w *Website) StatisticSearch(startTime int64, endTime int64) []response.ChartData {
	var tmpResult []response.ChartData
	w.DB.Model(&model.SearchLog{}).Where("created_time >= ? and created_time < ?", startTime, endTime).Select("DATE(FROM_UNIXTIME(created_time)) AS `date`, count(*) as `value`, 'search' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult)

	return tmpResult
}

func (w *Website) StatisticCustomer(startTime int64, endTime int64) []response.ChartData {
	// 新注册用户数量
	// 活跃用户数量
	var tmpResult []response.ChartData
	w.DB.Model(&model.User{}).Where("created_time >= ? and created_time < ?", startTime, endTime).Select("DATE(FROM_UNIXTIME(created_time)) AS `date`, count(*) as `value`, 'register' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult)

	var tmpResult2 []response.ChartData
	w.DB.Model(&model.LoginLog{}).Where("created_time >= ? and created_time < ? and status = 1", startTime, endTime).Select("DATE(FROM_UNIXTIME(created_time)) AS `date`, count(*) as `value`, 'login' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult2)

	return append(tmpResult, tmpResult2...)
}

func (w *Website) StatisticProduct(startTime int64, endTime int64) []response.ChartData {
	// 上新
	// 访问
	var result []response.ChartData
	// 上新。产品判定走 modules.kind，不再硬编码 module_id = 2：
	// 硬编码时如果用户重建过模型（id 变化），这里会恒为 0，图表看着就是「没有上新」。
	w.DB.Model(&model.Archive{}).
		Where("module_id IN (SELECT `id` FROM `modules` WHERE `kind` = ?)", model.ModuleKindProduct).
		Where("created_time >= ? and created_time < ?", startTime, endTime).
		Select("DATE(FROM_UNIXTIME(created_time)) AS `date`, count(*) as `value`, 'new' as `label`").
		Group("`date`").Order("`date`").Scan(&result)
	// 如果结束时间是今天，则今天的数据需要获取最新的数据
	today := now.EndOfDay().Unix()
	if endTime == today {
		if w.StatisticLog != nil {
			w.StatisticLog.Calc(w.DB)
		}
	}
	var tmpResult []*model.StatisticLog
	w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ? and created_time < ?", startTime, endTime).Omit("spider_count", "visit_count").Order("created_time ASC").Find(&tmpResult)
	if len(tmpResult) == 0 {
		// 首次访问没有数据，则先尝试生成
		if w.StatisticLog != nil {
			w.StatisticLog.Calc(w.DB)
			// 再次查询
			w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ? and created_time < ?", startTime, endTime).Omit("spider_count", "visit_count").Order("created_time ASC").Find(&tmpResult)
		}
	}
	for _, v := range tmpResult {
		vDate := time.Unix(v.CreatedTime, 0).Format("2006-01-02")
		result = append(result,
			response.ChartData{
				Date:  vDate,
				Value: v.ShopCount.ProductView,
				Label: "views",
			})
	}

	return result
}

func (w *Website) StatisticOrder(startTime int64, endTime int64) []response.ChartData {
	// 下单
	// 支付
	var tmpResult []response.ChartData
	w.DB.Model(&model.Order{}).Where("created_time >= ? and created_time < ?", startTime, endTime).Select("DATE(FROM_UNIXTIME(created_time)) AS `date`, count(*) as `value`, 'create' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult)

	var tmpResult2 []response.ChartData
	w.DB.Model(&model.Order{}).Where("paid_time >= ? and paid_time < ?", startTime, endTime).Select("DATE(FROM_UNIXTIME(paid_time)) AS `date`, count(*) as `value`, 'pay' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult2)
	var tmpResult3 []response.ChartData
	w.DB.Model(&model.Order{}).Where("paid_time >= ? and paid_time < ?", startTime, endTime).Select("DATE(FROM_UNIXTIME(paid_time)) AS `date`, sum(amount) as `value`, 'pay' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult3)

	tmpResult = append(tmpResult, tmpResult2...)
	tmpResult = append(tmpResult, tmpResult3...)
	return tmpResult
}

func (w *Website) StatisticConversion(startTime int64, endTime int64) []response.ChartData {
	// 转化率
	// 点击
	// 加购
	// checkout
	// confirm
	// pay
	var result []response.ChartData

	// 如果结束时间是今天，则今天的数据需要获取最新的数据
	today := now.EndOfDay().Unix()
	if endTime == today {
		if w.StatisticLog != nil {
			w.StatisticLog.Calc(w.DB)
		}
	}
	var tmpResult []*model.StatisticLog
	w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ? and created_time < ?", startTime, endTime).Omit("spider_count", "visit_count").Order("created_time ASC").Find(&tmpResult)
	if len(tmpResult) == 0 {
		// 首次访问没有数据，则先尝试生成
		if w.StatisticLog != nil {
			w.StatisticLog.Calc(w.DB)
			// 再次查询
			w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ? and created_time < ?", startTime, endTime).Omit("spider_count", "visit_count").Order("created_time ASC").Find(&tmpResult)
		}
	}
	for _, v := range tmpResult {
		vDate := time.Unix(v.CreatedTime, 0).Format("2006-01-02")
		result = append(result,
			response.ChartData{
				Date:  vDate,
				Value: v.ShopCount.ProductView,
				Label: "views",
			}, response.ChartData{
				Date:  vDate,
				Value: v.ShopCount.Cart,
				Label: "cart",
			}, response.ChartData{
				Date:  vDate,
				Value: v.ShopCount.Checkout,
				Label: "checkout",
			}, response.ChartData{
				Date:  vDate,
				Value: v.ShopCount.OrderCreate,
				Label: "order_create",
			}, response.ChartData{
				Date:  vDate,
				Value: v.ShopCount.PayCreate,
				Label: "pay_create",
			}, response.ChartData{
				Date:  vDate,
				Value: v.ShopCount.Payment,
				Label: "payment",
			})
	}

	return result
}

func (w *Website) StatisticLive(startTime int64, endTime int64) map[string][]response.ChartData {
	// 销售额，按天
	var results = map[string][]response.ChartData{}
	// 销售额
	var tmpResult1 []response.ChartData
	w.DB.Model(&model.Order{}).Where("paid_time >= ? and paid_time < ?", startTime, endTime).Select("DATE(FROM_UNIXTIME(paid_time)) AS `date`, sum(amount) as `value`, 'pay' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult1)
	results["amount"] = tmpResult1
	// 平均订单金额
	var tmpResult2 []response.ChartData
	w.DB.Model(&model.Order{}).Where("paid_time >= ? and paid_time < ?", startTime, endTime).Select("DATE(FROM_UNIXTIME(paid_time)) AS `date`, ROUND(sum(amount)/count(*)) as `value`, 'average' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult2)
	results["average_amount"] = tmpResult2
	// 订单数量
	var tmpResult3 []response.ChartData
	w.DB.Model(&model.Order{}).Where("paid_time >= ? and paid_time < ?", startTime, endTime).Select("DATE(FROM_UNIXTIME(paid_time)) AS `date`, count(*) as `value`, 'count' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult3)
	results["order_count"] = tmpResult3

	// 如果结束时间是今天，则今天的数据需要获取最新的数据
	today := now.EndOfDay().Unix()
	if endTime == today {
		if w.StatisticLog != nil {
			w.StatisticLog.Calc(w.DB)
		}
	}
	var tmpResult []*model.StatisticLog
	w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ? and created_time < ?", startTime, endTime).Order("created_time ASC").Find(&tmpResult)
	if len(tmpResult) == 0 {
		// 首次访问没有数据，则先尝试生成
		if w.StatisticLog != nil {
			w.StatisticLog.Calc(w.DB)
			// 再次查询
			w.DB.Model(&model.StatisticLog{}).Where("`created_time` >= ? and created_time < ?", startTime, endTime).Omit("spider_count", "visit_count").Order("created_time ASC").Find(&tmpResult)
		}
	}
	// 访问量 等
	var productView []response.ChartData
	// 复购，新客
	var order []response.ChartData
	// coupon
	var coupon []response.ChartData
	// 转化率
	var conversion []response.ChartData
	// customer
	var customer []response.ChartData

	for _, v := range tmpResult {
		vDate := time.Unix(v.CreatedTime, 0).Format("2006-01-02")
		// 访问量
		productView = append(productView, response.ChartData{
			Date:  vDate,
			Label: "view",
			Value: v.ShopCount.ProductView,
		})
		// 复购，新客
		order = append(order, response.ChartData{
			Date:  vDate,
			Label: "first_order",
			Value: v.ShopCount.FirstOrder,
		}, response.ChartData{
			Date:  vDate,
			Label: "repurchase",
			Value: v.ShopCount.Repurchase,
		})
		// coupon 使用量
		coupon = append(coupon, response.ChartData{
			Date:  vDate,
			Label: "coupon_get",
			Value: v.ShopCount.CouponGet,
		}, response.ChartData{
			Date:  vDate,
			Label: "coupon_use",
			Value: v.ShopCount.CouponUse,
		})
		// 转化率
		conversion = append(conversion, response.ChartData{
			Date:  vDate,
			Value: v.ShopCount.ProductView,
			Label: "views",
		}, response.ChartData{
			Date:  vDate,
			Value: v.ShopCount.Cart,
			Label: "cart",
		}, response.ChartData{
			Date:  vDate,
			Value: v.ShopCount.Checkout,
			Label: "checkout",
		}, response.ChartData{
			Date:  vDate,
			Value: v.ShopCount.OrderCreate,
			Label: "order_create",
		}, response.ChartData{
			Date:  vDate,
			Value: v.ShopCount.ProductView,
			Label: "pay_create",
		}, response.ChartData{
			Date:  vDate,
			Value: v.ShopCount.ProductView,
			Label: "payment",
		})
		//  客户
		customer = append(customer, response.ChartData{
			Date:  vDate,
			Label: "new_user",
			Value: v.ShopCount.UserCreate,
		}, response.ChartData{
			Date:  vDate,
			Label: "active",
			Value: v.ShopCount.UserLogin,
		})
	}

	results["product_view"] = productView
	results["order"] = order
	results["coupon"] = coupon
	results["conversion"] = conversion
	results["customer"] = customer

	return results
}

func (w *Website) StatisticSearchDetail(currentPage, limit int) ([]*model.SearchLog, int64) {
	var total int64
	var tmpResult []*model.SearchLog
	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * limit

	w.DB.Model(&model.SearchLog{}).Count(&total)
	w.DB.Offset(offset).Limit(limit).Order("id desc").Find(&tmpResult)

	return tmpResult, total
}

func (w *Website) SendStatisticsMail() {
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

	setting := w.PluginSendmail
	if setting == nil || setting.Account == "" {
		//成功配置，则跳过
		return
	}
	template, exist := w.GetEmailTemplateInfo("report")
	if !exist || template.Open == false {
		return
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

	// 开始写邮件内容
	frontUrl := w.System.BaseUrl
	if w.System.FrontUrl != "" {
		frontUrl = w.System.FrontUrl
	}
	subject := time.Now().Add(-86400*time.Second).Format("2006-01-02 ") + w.Tr("s(s)SiteData", w.System.SiteName, frontUrl)
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
	_ = w.SendMail(subject, content, nil)
}

func calcSpider(data map[string]int) int64 {
	var count int
	for _, v := range data {
		count += v
	}

	return int64(count)
}

// shop count cache
type CachedShopCount struct {
	mu             sync.Mutex
	Counts         model.ShopCount
	lastStoredTime int64
}
