package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"log"
)

func (w *Website) GetAiArticlePlanByReqId(reqId int64) (*model.AiArticlePlan, error) {
	var plan model.AiArticlePlan
	err := w.DB.Where("`req_id` = ?", reqId).Take(&plan).Error

	if err != nil {
		return nil, err
	}

	return &plan, nil
}

func (w *Website) GetAiArticlePlanById(id uint) (*model.AiArticlePlan, error) {
	var plan model.AiArticlePlan
	err := w.DB.Where("`id` = ?", id).Take(&plan).Error

	if err != nil {
		return nil, err
	}

	return &plan, nil
}

func (w *Website) GetAiArticlePlanByKeyword(planType int, keyword string) (*model.AiArticlePlan, error) {
	var plan model.AiArticlePlan
	err := w.DB.Where("`type` = ? and `keyword` = ?", planType, keyword).Take(&plan).Error

	if err != nil {
		return nil, err
	}

	return &plan, nil
}

func (w *Website) SaveAiArticlePlan(resp *AnqiAiResult, useSelf bool) (*model.AiArticlePlan, error) {
	if resp.ReqId > 0 {
		plan, err := w.GetAiArticlePlanByReqId(resp.ReqId)
		if err == nil {
			// 已存在
			return plan, nil
		}
	}

	plan := &model.AiArticlePlan{
		Type:       resp.Type,
		ReqId:      resp.ReqId,
		Language:   resp.Language,
		ToLanguage: resp.ToLanguage,
		Keyword:    resp.Keyword,
		Demand:     resp.Demand,
		ArticleId:  resp.ArticleId,
		PayCount:   resp.PayCount,
		UseSelf:    useSelf,
		Status:     resp.Status,
	}

	err := w.DB.Save(plan).Error
	if err != nil {
		return nil, err
	}

	return plan, nil
}

// SyncAiArticlePlan 从服务器中拉取进行中的任务
func (w *Website) SyncAiArticlePlan() {
	//if !w.AiGenerateConfig.Open {
	//	return
	//}
	var plans []*model.AiArticlePlan
	w.DB.Model(&model.AiArticlePlan{}).Where("`status` = ? and `use_self` = 0", config.AiArticleStatusDoing).Find(&plans)

	if len(plans) == 0 {
		return
	}

	for _, plan := range plans {
		err := w.AnqiSyncAiPlanResult(plan)
		if err != nil {
			log.Println("plan text", plan.Id, err)
		}
	}
}
