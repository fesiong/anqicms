package provider

import (
    "crypto/tls"
    "errors"
    "fmt"
    "github.com/medivhzhan/weapp/v3"
    "io/ioutil"
    "kandaoni.com/anqicms/config"
    "kandaoni.com/anqicms/dao"
    "kandaoni.com/anqicms/library"
    "kandaoni.com/anqicms/model"
    "net/http"
    "time"
)

var weappClient *weapp.Client

func GetWeappClient(focus bool) *weapp.Client {
    if weappClient == nil || focus {
        httpCli := &http.Client{
            Timeout: 10 * time.Second,
            Transport: &http.Transport{
                // 跳过校验
                TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
            },
        }

        weappClient = weapp.NewClient(
            config.JsonData.PluginWeapp.AppID,
            config.JsonData.PluginWeapp.AppSecret,
            weapp.WithHttpClient(httpCli),
        )
    }

    return weappClient
}

func GetWeappQrcode(weappPath, scene string, userId uint) (string, error) {
    var qrcode model.WeappQrcode
    err := dao.DB.Where("`user_id` = ? and `path` = ?", userId, weappPath).Take(&qrcode).Error
    if err != nil {
        // 没有
        codeUrl, err := CreateWeappQrcode(weappPath, scene)
        if err != nil {
            return "", err
        }
        qrcode = model.WeappQrcode{
            UserId:  userId,
            Path:    weappPath,
            CodeUrl: codeUrl,
        }
        dao.DB.Save(&qrcode)
    }

    return config.JsonData.PluginStorage.StorageUrl + "/" + qrcode.CodeUrl, nil
}

func CreateWeappQrcode(weappPath, scene string) (string, error) {
    creator := weapp.QRCode{
        Path: weappPath,
    }
    resp, commonErr, err := GetWeappClient(false).GetQRCode(&creator)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if commonErr.ErrCode != 0 {
        return "", errors.New(commonErr.ErrMSG)
    }
    // 写入二维码到数据库
    bts, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    md5Str := library.Md5Bytes(bts)
    tmpName := md5Str + ".png"
    filePath := fmt.Sprintf("uploads/qrcode/%s/%s/%s", tmpName[:3], tmpName[3:6], tmpName[6:])

    _, err = Storage.UploadFile(filePath, bts)
    if err != nil {
        return "", err
    }
    //文件上传完成
    attachment := &model.Attachment{
        FileName:     tmpName,
        FileLocation: filePath,
        FileSize: int64(len(bts)),
        FileMd5:      md5Str,
        CategoryId:   0,
        IsImage:      0,
        Status:       1,
    }
    err = attachment.Save(dao.DB)
    attachment.GetThumb()

    return attachment.FileLocation, nil
}