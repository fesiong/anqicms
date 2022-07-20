package crond

import (
	"github.com/robfig/cron/v3"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"time"
)

func Crond() {
	crontab := cron.New(cron.WithSeconds())
	//每天执行一次，清理很久的statistic
	crontab.AddFunc("@daily", cleanStatistics)
	// 每天清理一次回收站内容
	crontab.AddFunc("@daily", provider.CleanArchives)
	crontab.AddFunc("@hourly", startDigKeywords)
	crontab.AddFunc("1 */10 * * * *", provider.CollectArticles)
	//每天检查一次收录量
	crontab.AddFunc("30 30 8 * * *", provider.QuerySpiderInclude)
	// 每分钟检查一次需要发布的文章
	crontab.AddFunc("1 * * * * *", PublishPlanContents)
	crontab.Start()
}

func startDigKeywords() {
	if dao.DB == nil {
		return
	}
	provider.StartDigKeywords(false)
}

func cleanStatistics() {
	if dao.DB == nil {
		return
	}
	//清理一个月前的记录
	agoStamp := time.Now().AddDate(0, 0, -30).Unix()
	dao.DB.Unscoped().Where("`created_time` < ?", agoStamp).Delete(model.Statistic{})

}

func PublishPlanContents() {
	if dao.DB == nil {
		return
	}
	go provider.PublishPlanArchives()
}
