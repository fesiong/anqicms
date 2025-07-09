package provider

import (
	"context"
	"fmt"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/wechat"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"log"
	"time"
)

func (w *Website) GetWithdrawList(page, pageSize int) ([]*model.UserWithdraw, int64) {
	var withdraws []*model.UserWithdraw
	var total int64
	offset := (page - 1) * pageSize
	w.DB.Model(&model.UserWithdraw{}).Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&withdraws)
	if len(withdraws) > 0 {
		var userIds = make([]uint, 0, len(withdraws))
		for i := range withdraws {
			userIds = append(userIds, withdraws[i].UserId)
		}
		users := w.GetUsersInfoByIds(userIds)
		for i := range withdraws {
			for u := range users {
				if withdraws[i].UserId == users[u].Id {
					withdraws[i].UserName = users[u].UserName
				}
			}
		}
	}
	return withdraws, total
}

func (w *Website) GetWithdrawById(id uint) (*model.UserWithdraw, error) {
	var withdraw model.UserWithdraw
	err := w.DB.Where(&model.UserWithdraw{}).Where("`id` = ?", id).Take(&withdraw).Error
	if err != nil {
		return nil, err
	}

	return &withdraw, nil
}

func (w *Website) SetUserWithdrawApproval(req *request.UserWithdrawRequest) error {
	withdraw, err := w.GetWithdrawById(req.Id)
	if err != nil {
		return err
	}

	// todo 打款给用户
	withdraw.Status = config.WithdrawStatusAgree
	w.DB.Save(withdraw)

	//生成用户支付记录
	var userBalance int64
	err = w.DB.Model(&model.User{}).Where("`id` = ?", withdraw.UserId).Pluck("balance", &userBalance).Error
	//状态更改了，增加一条记录到用户
	finance := model.Finance{
		UserId:      withdraw.UserId,
		Direction:   config.FinanceOutput,
		Amount:      withdraw.Amount,
		AfterAmount: userBalance,
		Action:      config.FinanceActionWithdraw,
		OrderId:     "",
		Status:      1,
	}
	err = w.DB.Create(&finance).Error
	if err != nil {
		//
	}

	return nil
}

func (w *Website) SetUserWithdrawFinished(req *request.UserWithdrawRequest) error {
	withdraw, err := w.GetWithdrawById(req.Id)
	if err != nil {
		return err
	}

	// todo 打款给用户
	withdraw.Status = config.WithdrawStatusFinished
	withdraw.SuccessTime = time.Now().Unix()
	w.DB.Save(withdraw)

	//生成用户支付记录
	var userBalance int64
	err = w.DB.Model(&model.User{}).Where("`id` = ?", withdraw.UserId).Pluck("balance", &userBalance).Error
	//状态更改了，增加一条记录到用户
	finance := model.Finance{
		UserId:      withdraw.UserId,
		Direction:   config.FinanceOutput,
		Amount:      withdraw.Amount,
		AfterAmount: userBalance,
		Action:      config.FinanceActionWithdraw,
		OrderId:     "",
		Status:      1,
	}
	err = w.DB.Create(&finance).Error
	if err != nil {
		//
	}

	return nil
}

var withdrawRunning = false

func (w *Website) CheckWithdrawToWechat() {
	if w.DB == nil {
		return
	}
	if withdrawRunning {
		return
	}
	withdrawRunning = true
	defer func() {
		withdrawRunning = false
	}()
	if w.PluginPay.WechatKeyPath == "" || (w.PluginPay.WechatAppId == "" && w.PluginPay.WeappAppId == "") {
		return
	}
	var withdraws []model.UserWithdraw

	w.DB.Where("status = ?", config.CommissionStatusWait).Find(&withdraws)
	nowStamp := time.Now().Unix()

	if len(withdraws) == 0 {
		return
	}
	var wechatClient *wechat.Client
	var weapp2Client *wechat.Client
	var err error
	if w.PluginPay.WechatAppId != "" {
		wechatClient = wechat.NewClient(w.PluginPay.WechatAppId, w.PluginPay.WechatMchId, w.PluginPay.WechatApiKey, true)
		err = wechatClient.AddCertPemFilePath(w.DataPath+"cert/"+w.PluginPay.WechatCertPath, w.DataPath+"cert/"+w.PluginPay.WechatKeyPath)
		if err != nil {
			log.Println("微信证书错误：", err.Error())
			return
		}
	}
	if w.PluginPay.WeappAppId != "" {
		weapp2Client = wechat.NewClient(w.PluginPay.WeappAppId, w.PluginPay.WechatMchId, w.PluginPay.WechatApiKey, true)
		err = weapp2Client.AddCertPemFilePath(w.DataPath+"cert/"+w.PluginPay.WechatCertPath, w.DataPath+"cert/"+w.PluginPay.WechatKeyPath)
		if err != nil {
			log.Println("微信证书错误：", err.Error())
			return
		}
	}

	for _, withdraw := range withdraws {
		if withdraw.ErrorTimes > 0 {
			// 判断是否需要执行，1分，10分钟，1小时，1天
			if withdraw.ErrorTimes > 3 {
				if withdraw.LastTime > nowStamp-86400 {
					continue
				}
			} else if withdraw.ErrorTimes > 2 {
				if withdraw.LastTime > nowStamp-3600 {
					continue
				}
			} else if withdraw.ErrorTimes > 1 {
				if withdraw.LastTime > nowStamp-600 {
					continue
				}
			}
		}

		// 请求提现
		userWechat, err := w.GetUserWechatByUserId(withdraw.UserId)
		if err != nil {
			// 这种情况一般不会出现
			withdraw.Status = -1
			withdraw.Remark = w.Tr("UserDoesNotExist")
			w.DB.Save(&withdraw)
			continue
		}
		user, err := w.GetUserInfoById(withdraw.UserId)
		if err != nil {
			// 这种情况一般不会出现
			withdraw.Status = -1
			withdraw.Remark = w.Tr("UserDoesNotExist")
			w.DB.Save(&withdraw)
			continue
		}
		if userWechat.Openid == "" || user.RealName == "" {
			// 这种情况一般不会出现
			withdraw.ErrorTimes++
			withdraw.Remark = w.Tr("UserIsNotBoundToWechatOrReal-NameAuthenticationIsNotAvailable")
			withdraw.LastTime = nowStamp
			w.DB.Save(&withdraw)
			continue
		}

		bm := make(gopay.BodyMap)
		bm.Set("nonce_str", library.GenerateRandString(32)).
			Set("partner_trade_no", fmt.Sprintf("%d", withdraw.Id)).
			Set("openid", userWechat.Openid).
			Set("check_name", "FORCE_CHECK").
			Set("re_user_name", user.RealName).
			Set("amount", withdraw.Amount).
			Set("desc", w.Tr("CommissionWithdrawal")).
			Set("sign_type", wechat.SignType_HMAC_SHA256)

		var wxRsp *wechat.TransfersResponse
		if userWechat.Platform == config.PlatformWeapp {
			if weapp2Client == nil {
				withdraw.ErrorTimes++
				withdraw.Remark = w.Tr("Error")
				withdraw.LastTime = nowStamp
				w.DB.Save(&withdraw)
				continue
			}
			wxRsp, err = weapp2Client.Transfer(context.Background(), bm)
			if err != nil {
				withdraw.ErrorTimes++
				withdraw.Remark = err.Error()
				withdraw.LastTime = nowStamp
				w.DB.Save(&withdraw)
				continue
			}
		} else {
			if wechatClient == nil {
				withdraw.ErrorTimes++
				withdraw.Remark = w.Tr("Error")
				withdraw.LastTime = nowStamp
				w.DB.Save(&withdraw)
				continue
			}
			wxRsp, err = wechatClient.Transfer(context.Background(), bm)
			if err != nil {
				withdraw.ErrorTimes++
				withdraw.Remark = err.Error()
				withdraw.LastTime = nowStamp
				w.DB.Save(&withdraw)
				continue
			}
		}

		if wxRsp.ReturnCode == gopay.FAIL {
			withdraw.ErrorTimes++
			withdraw.Remark = wxRsp.ReturnMsg
			withdraw.LastTime = nowStamp
			w.DB.Save(&withdraw)
			continue
		}
		if wxRsp.ResultCode == gopay.FAIL {
			withdraw.ErrorTimes++
			withdraw.Remark = wxRsp.ErrCodeDes
			withdraw.LastTime = nowStamp
			w.DB.Save(&withdraw)
			continue
		}

		withdraw.Status = config.CommissionStatusPaid
		withdraw.Remark = ""
		withdraw.LastTime = nowStamp
		w.DB.Save(&withdraw)

	}
}
