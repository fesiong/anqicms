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

func CheckWithdrawToWechat() {
	if dao.DB == nil {
		return
	}
	var withdraws []model.UserWithdraw

	dao.DB.Where("status = ?", config.CommissionStatusWait).Find(&withdraws)
	nowStamp := time.Now().Unix()

	if len(withdraws) == 0 {
		return
	}

	client := wechat.NewClient(config.JsonData.PluginPay.WeixinAppId, config.JsonData.PluginPay.WeixinMchId, config.JsonData.PluginPay.WeixinApiKey, true)
	err := client.AddCertPemFilePath(config.ExecPath + config.JsonData.PluginPay.WeixinCertPath, config.ExecPath + config.JsonData.PluginPay.WeixinKeyPath)
	if err != nil {
		log.Println("微信证书错误：", err.Error())
		return
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
		userWeixin, err := GetUserWeixinByUserId(withdraw.UserId)
		if err != nil {
			// 这种情况一般不会出现
			withdraw.Status = -1
			withdraw.Remark = "用户不存在"
			dao.DB.Save(&withdraw)
			continue
		}
		user, err := GetUserInfoById(withdraw.UserId)
		if err != nil {
			// 这种情况一般不会出现
			withdraw.Status = -1
			withdraw.Remark = "用户不存在"
			dao.DB.Save(&withdraw)
			continue
		}
		if userWeixin.Openid == "" || user.RealName == "" {
			// 这种情况一般不会出现
			withdraw.ErrorTimes++
			withdraw.Remark = "用户未绑定微信或未实名认证"
			withdraw.LastTime = nowStamp
			dao.DB.Save(&withdraw)
			continue
		}

		bm := make(gopay.BodyMap)
		bm.Set("nonce_str", util.RandomString(32)).
			Set("partner_trade_no", fmt.Sprintf("%d", withdraw.Id)).
			Set("openid", userWeixin.Openid).
			Set("check_name", "FORCE_CHECK").
			Set("re_user_name", user.RealName).
			Set("amount", withdraw.Amount).
			Set("desc", "搜外内容管家佣金提现").
			Set("sign_type", wechat.SignType_HMAC_SHA256)

		wxRsp, err := client.Transfer(context.Background(), bm)
		if err != nil {
			withdraw.ErrorTimes++
			withdraw.Remark = err.Error()
			withdraw.LastTime = nowStamp
			dao.DB.Save(&withdraw)
			continue
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
