package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/esap/wechat/util"
	"log"
	"time"
)

// 内容安全检查 使用QQ的安全检查接口
const QQAppid = "1112215177"
const QQSecret = "1skIGTJgKM2ZcILg"

func QQMsgSecCheck(content string) bool {
	token, err := QQGetAccessToken()
	if err != nil {
		return false
	}
	api := fmt.Sprintf("https://api.q.qq.com/api/json/security/MsgSecCheck?access_token=%s", token)
	postData := map[string]string{
		"access_token": token,
		"appid":        QQAppid,
		"content":      content,
	}
	buf, err := util.PostJson(api, postData)
	if err != nil {
		return false
	}
	log.Println(string(buf))
	type SecResult struct {
		ErrCode int    `json:"errCode"`
		ErrMsg  string `json:"errMsg"`
	}
	var resResult SecResult
	err = json.Unmarshal(buf, &resResult)
	if err != nil {
		return false
	}

	return resResult.ErrCode == 87014
}

type QQAccessTokenResult struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	Errcode     int    `json:"errcode"`
	Errmsg      string `json:"errmsg"`
	LastTime    int64
}

var QQAccessToken = QQAccessTokenResult{}

func QQGetAccessToken() (string, error) {
	if QQAccessToken.ExpiresIn > time.Now().Unix() {
		return QQAccessToken.AccessToken, nil
	}
	if QQAccessToken.Errcode > 0 && QQAccessToken.LastTime > time.Now().Add(-7000*time.Second).Unix() {
		return "", errors.New(QQAccessToken.Errmsg)
	}
	api := fmt.Sprintf("https://api.q.qq.com/api/getToken?grant_type=client_credential&appid=%s&secret=%s", QQAppid, QQSecret)
	err := util.GetJson(api, &QQAccessToken)
	if err != nil {
		return "", err
	}
	QQAccessToken.LastTime = time.Now().Unix()
	if QQAccessToken.Errcode != 0 {
		return "", errors.New(QQAccessToken.Errmsg)
	}
	QQAccessToken.ExpiresIn = time.Now().Unix() + QQAccessToken.ExpiresIn - 200

	return QQAccessToken.AccessToken, nil
}
