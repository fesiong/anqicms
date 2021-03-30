package crond

import (
	"github.com/robfig/cron/v3"
	"irisweb/config"
	"irisweb/model"
	"time"
)

func Crond(){
	crontab := cron.New(cron.WithSeconds())
	//每天执行一次，清理很久的statistic
	crontab.AddFunc("@daily", cleanStatistics)
	crontab.Start()
}

func cleanStatistics() {
	//清理一个月前的记录
	agoStamp := time.Now().AddDate(0, 0, -30).Unix()
	config.DB.Unscoped().Where("`created_time` < ?", agoStamp).Delete(model.Statistic{})
}