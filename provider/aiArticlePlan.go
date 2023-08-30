package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

func (w *Website) GetAiArticlePlanByReqId(reqId uint) (*model.AiArticlePlan, error) {
	var plan model.AiArticlePlan
	err := w.DB.Where("`req_id` = ?", reqId).Take(&plan).Error

	if err != nil {
		return nil, err
	}

	return &plan, nil
}

func (w *Website) SaveAiArticlePlan(resp *AnqiAiResult) (*model.AiArticlePlan, error) {
	plan, err := w.GetAiArticlePlanByReqId(resp.ReqId)
	if err == nil {
		// 已存在
		return plan, nil
	}

	plan = &model.AiArticlePlan{
		Type:      resp.Type,
		ReqId:     resp.ReqId,
		Language:  resp.Language,
		Keyword:   resp.Keyword,
		Demand:    resp.Demand,
		ArticleId: resp.ArticleId,
		PayCount:  resp.PayCount,
		Status:    config.AiArticleStatusDoing,
	}

	err = w.DB.Save(plan).Error
	if err != nil {
		return nil, err
	}

	return plan, nil
}

// SyncAiArticlePlan 从服务器中拉取进行中的任务
func (w *Website) SyncAiArticlePlan() {
	var plans []*model.AiArticlePlan
	w.DB.Model(&model.AiArticlePlan{}).Where("`status` = ? and `use_self` = 0", config.AiArticleStatusDoing).Find(&plans)

	if len(plans) == 0 {
		return
	}

	for _, plan := range plans {
		w.AnqiSyncAiPlanResult(plan)
	}
}
