package provider

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/medivhzhan/weapp/v3"
	"io"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"net/http"
	"time"
)

func (w *Website) GetWeappClient(focus bool) *weapp.Client {
	if w.weappClient == nil || focus {
		httpCli := &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				// 跳过校验
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		w.weappClient = weapp.NewClient(
			w.PluginWeapp.AppID,
			w.PluginWeapp.AppSecret,
			weapp.WithHttpClient(httpCli),
		)
		w2 := GetWebsite(w.Id)
		w2.weappClient = w.weappClient
	}

	return w.weappClient
}

func (w *Website) GetWeappQrcode(weappPath, scene string, userId uint) (string, error) {
	var qrcode model.WeappQrcode
	err := w.DB.Where("`user_id` = ? and `path` = ?", userId, weappPath).Take(&qrcode).Error
	if err != nil {
		// 没有
		codeUrl, err := w.CreateWeappQrcode(weappPath, scene)
		if err != nil {
			return "", err
		}
		qrcode = model.WeappQrcode{
			UserId:  userId,
			Path:    weappPath,
			CodeUrl: codeUrl,
		}
		w.DB.Save(&qrcode)
	}

	return w.PluginStorage.StorageUrl + "/" + qrcode.CodeUrl, nil
}

func (w *Website) CreateWeappQrcode(weappPath, scene string) (string, error) {
	creator := weapp.QRCode{
		Path: weappPath,
	}
	resp, commonErr, err := w.GetWeappClient(false).GetQRCode(&creator)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if commonErr.ErrCode != 0 {
		return "", errors.New(commonErr.ErrMSG)
	}
	// 写入二维码到数据库
	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	md5Str := library.Md5Bytes(bts)
	tmpName := md5Str + ".png"
	filePath := fmt.Sprintf("uploads/qrcode/%s/%s/%s", tmpName[:3], tmpName[3:6], tmpName[6:])

	_, err = w.Storage.UploadFile(filePath, bts)
	if err != nil {
		return "", err
	}
	//文件上传完成
	attachment := &model.Attachment{
		FileName:     tmpName,
		FileLocation: filePath,
		FileSize:     int64(len(bts)),
		FileMd5:      md5Str,
		CategoryId:   0,
		IsImage:      0,
		Status:       1,
	}
	err = attachment.Save(w.DB)
	attachment.GetThumb(w.PluginStorage.StorageUrl)

	return attachment.FileLocation, nil
}
