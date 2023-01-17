package provider

import (
	"context"
	"fmt"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/pkg/util"
	"github.com/go-pay/gopay/wechat"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"log"
	"time"
)

func GetWithdrawList(page, pageSize int) ([]*model.UserWithdraw, int64) {
	var withdraws []*model.UserWithdraw
	var total int64
	offset := (page - 1) * pageSize
	dao.DB.Model(&model.UserWithdraw{}).Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&withdraws)
	if len(withdraws) > 0 {
		var userIds = make([]uint, 0, len(withdraws))
		for i := range withdraws {
			userIds = append(userIds, withdraws[i].UserId)
		}
		users := GetUsersInfoByIds(userIds)
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

func GetWithdrawById(id uint) (*model.UserWithdraw, error) {
	var withdraw model.UserWithdraw
	err := dao.DB.Where(&model.UserWithdraw{}).Where("`id` = ?", id).Take(&withdraw).Error
	if err != nil {
		return nil, err
	}

	return &withdraw, nil
}

func SetUserWithdrawApproval(req *request.UserWithdrawRequest) error {
	withdraw, err := GetWithdrawById(req.Id)
	if err != nil {
		return err
	}

	// todo 打款给用户
	withdraw.Status = config.WithdrawStatusAgree
	dao.DB.Save(withdraw)

	//生成用户支付记录
	var userBalance int64
	err = dao.DB.Model(&model.User{}).Where("`id` = ?", withdraw.UserId).Pluck("balance", &userBalance).Error
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
	err = dao.DB.Create(&finance).Error
	if err != nil {
		//
	}

	return nil
}

func SetUserWithdrawFinished(req *request.UserWithdrawRequest) error {
	withdraw, err := GetWithdrawById(req.Id)
	if err != nil {
		return err
	}

	// todo 打款给用户
	withdraw.Status = config.WithdrawStatusFinished
	withdraw.SuccessTime = time.Now().Unix()
	dao.DB.Save(withdraw)

	//生成用户支付记录
	var userBalance int64
	err = dao.DB.Model(&model.User{}).Where("`id` = ?", withdraw.UserId).Pluck("balance", &userBalance).Error
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
	err = dao.DB.Create(&finance).Error
	if err != nil {
		//
	}

	return nil
}

var withdrawRunning = false

func CheckWithdrawToWechat() {
	if dao.DB == nil {
		return
	}
	if withdrawRunning {
		return
	}
	withdrawRunning = true
	defer func() {
		withdrawRunning = false
	}()
	var withdraws []model.UserWithdraw

	dao.DB.Where("status = ?", config.CommissionStatusWait).Find(&withdraws)
	nowStamp := time.Now().Unix()

	if len(withdraws) == 0 {
		return
	}
	var wechatClient *wechat.Client
	var weapp2Client *wechat.Client
	var err error
	if config.JsonData.PluginPay.WechatAppId != "" {
		wechatClient = wechat.NewClient(config.JsonData.PluginPay.WechatAppId, config.JsonData.PluginPay.WechatMchId, config.JsonData.PluginPay.WechatApiKey, true)
		err = wechatClient.AddCertPemFilePath(config.ExecPath+config.JsonData.PluginPay.WechatCertPath, config.ExecPath+config.JsonData.PluginPay.WechatKeyPath)
		if err != nil {
			log.Println("微信证书错误：", err.Error())
			return
		}
	}
	if config.JsonData.PluginPay.WeappAppId != "" {
		weapp2Client = wechat.NewClient(config.JsonData.PluginPay.WeappAppId, config.JsonData.PluginPay.WechatMchId, config.JsonData.PluginPay.WechatApiKey, true)
		err = weapp2Client.AddCertPemFilePath(config.ExecPath+config.JsonData.PluginPay.WechatCertPath, config.ExecPath+config.JsonData.PluginPay.WechatKeyPath)
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
		userWechat, err := GetUserWechatByUserId(withdraw.UserId)
		if err != nil {
			// 这种情况一般不会出现
			withdraw.Status = -1
			withdraw.Remark = config.Lang("用户不存在")
			dao.DB.Save(&withdraw)
			continue
		}
		user, err := GetUserInfoById(withdraw.UserId)
		if err != nil {
			// 这种情况一般不会出现
			withdraw.Status = -1
			withdraw.Remark = config.Lang("用户不存在")
			dao.DB.Save(&withdraw)
			continue
		}
		if userWechat.Openid == "" || user.RealName == "" {
			// 这种情况一般不会出现
			withdraw.ErrorTimes++
			withdraw.Remark = config.Lang("用户未绑定微信或未实名认证")
			withdraw.LastTime = nowStamp
			dao.DB.Save(&withdraw)
			continue
		}

		bm := make(gopay.BodyMap)
		bm.Set("nonce_str", util.RandomString(32)).
			Set("partner_trade_no", fmt.Sprintf("%d", withdraw.Id)).
			Set("openid", userWechat.Openid).
			Set("check_name", "FORCE_CHECK").
			Set("re_user_name", user.RealName).
			Set("amount", withdraw.Amount).
			Set("desc", config.Lang("佣金提现")).
			Set("sign_type", wechat.SignType_HMAC_SHA256)

		var wxRsp *wechat.TransfersResponse
		if userWechat.Platform == config.PlatformWeapp {
			if weapp2Client == nil {
				withdraw.ErrorTimes++
				withdraw.Remark = config.Lang("出错")
				withdraw.LastTime = nowStamp
				dao.DB.Save(&withdraw)
				continue
			}
			wxRsp, err = weapp2Client.Transfer(context.Background(), bm)
			if err != nil {
				withdraw.ErrorTimes++
				withdraw.Remark = err.Error()
				withdraw.LastTime = nowStamp
				dao.DB.Save(&withdraw)
				continue
			}
		} else {
			if wechatClient == nil {
				withdraw.ErrorTimes++
				withdraw.Remark = config.Lang("出错")
				withdraw.LastTime = nowStamp
				dao.DB.Save(&withdraw)
				continue
			}
			wxRsp, err = wechatClient.Transfer(context.Background(), bm)
			if err != nil {
				withdraw.ErrorTimes++
				withdraw.Remark = err.Error()
				withdraw.LastTime = nowStamp
				dao.DB.Save(&withdraw)
				continue
			}
		}

		if wxRsp.ReturnCode == gopay.FAIL {
			withdraw.ErrorTimes++
			withdraw.Remark = wxRsp.ReturnMsg
			withdraw.LastTime = nowStamp
			dao.DB.Save(&withdraw)
			continue
		}
		if wxRsp.ResultCode == gopay.FAIL {
			withdraw.ErrorTimes++
			withdraw.Remark = wxRsp.ErrCodeDes
			withdraw.LastTime = nowStamp
			dao.DB.Save(&withdraw)
			continue
		}

		withdraw.Status = config.CommissionStatusPaid
		withdraw.Remark = ""
		withdraw.LastTime = nowStamp
		dao.DB.Save(&withdraw)

	}
}
