package crond

import (
	"github.com/robfig/cron/v3"
	"kandaoni.com/anqicms/provider"
	"math/rand"
	"time"
)

func Crond() {
	crontab := cron.New(cron.WithSeconds())
	//每天执行一次，清理很久的statistic
	crontab.AddFunc("@daily", cleanStatistics)
	// 每天清理一次回收站内容
	crontab.AddFunc("@daily", CleanArchives)
	crontab.AddFunc("@hourly", startDigKeywords)
	crontab.AddFunc("1 */10 * * * *", CollectArticles)
	//每天检查一次收录量
	crontab.AddFunc("30 30 8 * * *", QuerySpiderInclude)
	// 每分钟检查一次需要发布的文章
	crontab.AddFunc("1 * * * * *", PublishPlanContents)
	// 每分钟提现
	crontab.AddFunc("1 * * * * *", CheckWithdrawToWechat)
	// 每分钟定期检查订单
	crontab.AddFunc("1 * * * * *", AutoCheckOrders)
	// 每天检查VIP
	crontab.AddFunc("@daily", CleanUserVip)
	// 每小时检查一次账号状态
	crontab.AddFunc("1 30 * * * *", CheckAuthValid)
	// 每小时检查一次时间因子
	crontab.AddFunc("1 10 * * * *", UpdateTimeFactor)
	// 每分钟检查一次 AI文章计划
	crontab.AddFunc("1 * * * * *", AiArticlePlan)
	crontab.Start()
}

func startDigKeywords() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.StartDigKeywords(false)
	}
}

func cleanStatistics() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.CleanStatistics()
	}
}

func PublishPlanContents() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.PublishPlanArchives()
	}
}

func CleanArchives() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.CleanArchives()
	}
}

func CollectArticles() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.CollectArticles()
	}
}

func QuerySpiderInclude() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.QuerySpiderInclude()
	}
}

func CheckWithdrawToWechat() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.CheckWithdrawToWechat()
	}
}

func AutoCheckOrders() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.AutoCheckOrders()
	}
}

func CleanUserVip() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.CleanUserVip()
	}
}

func CheckAuthValid() {
	rand.Seed(time.Now().UnixNano())
	time.Sleep(time.Duration(rand.Intn(600)+1) * time.Second)
	defaultSite := provider.CurrentSite(nil)
	defaultSite.AnqiCheckLogin(false)
}

func UpdateTimeFactor() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.TryToRunTimeFactor()
	}
}

func AiArticlePlan() {
	websites := provider.GetWebsites()
	for _, w := range websites {
		if !w.Initialed {
			continue
		}
		w.SyncAiArticlePlan()
	}
}
