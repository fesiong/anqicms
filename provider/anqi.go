package provider

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/parnurzeal/gorequest"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/request"
	"mime/multipart"
	"path/filepath"
	"time"
)

const AnqiApi = "https://www.anqicms.com/auth"

type AnqiLoginResult struct {
	Code int                   `json:"code"`
	Msg  string                `json:"msg"`
	Data config.AnqiUserConfig `json:"data"`
}

type AnqiTemplateResult struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type AnqiAttachment struct {
	Id           uint   `json:"id"`
	FileName     string `json:"file_name"`
	FileLocation string `json:"file_location"`
	FileSize     int64  `json:"file_size"`
	FileMd5      string `json:"file_md5"`
	IsImage      int    `json:"is_image"`
	Logo         string `json:"logo"`
	Thumb        string `json:"thumb"`
}

type AnqiAttachmentResult struct {
	Code int            `json:"code"`
	Msg  string         `json:"msg"`
	Data AnqiAttachment `json:"data"`
}

func AnqiLogin(req *request.AnqiLoginRequest) error {
	// 重置
	config.AnqiUser = config.AnqiUserConfig{}
	_ = SaveSettingValue(AnqiSettingKey, config.AnqiUser)
	var result AnqiLoginResult
	_, body, errs := NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/login").Send(req).EndStruct(&result)

	if len(errs) > 0 {
		library.DebugLog("error", string(body))
		return errs[0]
	}

	if result.Code != 0 {
		return errors.New(result.Msg)
	}

	// login success
	config.AnqiUser = result.Data
	config.AnqiUser.LoginTime = time.Now().Unix()
	config.AnqiUser.CheckTime = config.AnqiUser.LoginTime
	err := SaveSettingValue(AnqiSettingKey, config.AnqiUser)
	if err != nil {
		return err
	}

	return nil
}

func AnqiCheckLogin() {
	if config.AnqiUser.AuthId == 0 {
		return
	}
	if config.AnqiUser.CheckTime > time.Now().Add(-86400*time.Second).Unix() {
		return
	}
	var result AnqiLoginResult
	_, body, errs := NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/check").Send(config.AnqiUser).EndStruct(&result)

	if len(errs) > 0 {
		library.DebugLog("error", string(body))
		config.AnqiUser.CheckTime = time.Now().Unix()
		return
	}

	if result.Code != 0 {
		// 重置
		config.AnqiUser = config.AnqiUserConfig{}
		_ = SaveSettingValue(AnqiSettingKey, config.AnqiUser)
		return
	}

	// login success
	config.AnqiUser.CheckTime = time.Now().Unix()
	_ = SaveSettingValue(AnqiSettingKey, config.AnqiUser)
}

func AnqiShareTemplate(req *request.AnqiTemplateRequest) error {
	if config.AnqiUser.AuthId == 0 {
		return errors.New("请先登录 AnqiCMS 账号")
	}
	design, err := GetDesignInfo(req.Package, false)
	if err != nil {
		return err
	}
	if req.AutoBackup {
		// 先自动备份
		err = BackupDesignData(req.Package)
		if err != nil {
			return err
		}
	}
	// 需要先推送design
	var result AnqiTemplateResult
	designData, err := CreateDesignZip(design.Package)
	if err != nil {
		return err
	}
	attach, err := AnqiUploadAttachment(designData.Bytes(), design.Package+".zip")
	if err != nil {
		return err
	}
	req.TemplatePath = attach.FileLocation
	req.TemplateType = design.TemplateType
	req.TemplateId = design.TemplateId
	// 开始提交数据
	_, body, errs := NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/template/share").Send(req).EndStruct(&result)
	if len(errs) > 0 {
		library.DebugLog("error", string(body))
		return errs[0]
	}

	if result.Code != 0 {
		return errors.New(result.Msg)
	}

	return nil
}

func AnqiUploadAttachment(data []byte, name string) (*AnqiAttachment, error) {
	if config.AnqiUser.AuthId == 0 {
		return nil, errors.New("请先登录 AnqiCMS 账号")
	}

	var result AnqiAttachmentResult
	_, body, errs := NewAuthReq(gorequest.TypeMultipart).Post(AnqiApi+"/template/upload").SendFile(data, name, "attach").EndStruct(&result)
	if len(errs) > 0 {
		library.DebugLog("error", string(body))
		return nil, errs[0]
	}
	if result.Code != 0 {
		return nil, errors.New(result.Msg)
	}

	return &result.Data, nil
}

func AnqiDownloadTemplate(req *request.AnqiTemplateRequest) error {
	var result AnqiTemplateResult

	_, body, errs := NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/template/download").Send(req).EndStruct(&result)
	if len(errs) > 0 {
		library.DebugLog("error", string(body))
		return errs[0]
	}
	if result.Code != 0 {
		return errors.New(result.Msg)
	}

	downloadUrl, ok := result.Data.(string)
	if !ok {
		return errors.New("读取下载地址错误")
	}

	_, body, errs = NewAuthReq(gorequest.TypeHTML).Get(downloadUrl).EndBytes()
	if errs != nil {
		return errs[0]
	}

	info := &multipart.FileHeader{
		Filename: filepath.Base(downloadUrl),
		Header:   nil,
		Size:     int64(len(body)),
	}
	file := bytes.NewReader(body)

	// 将模板写入到本地
	err := UploadDesignZip(file, info)
	if err != nil {
		return err
	}

	return nil
}

func NewAuthReq(contentType string) *gorequest.SuperAgent {
	req := gorequest.New().
		SetDoNotClearSuperAgent(true).
		TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
		Timeout(30*time.Second).
		Type(contentType).
		Set("token", config.AnqiUser.Token).
		//set key header
		Set("domain", config.JsonData.System.BaseUrl).
		//set oem header
		Set("User-Agent", fmt.Sprintf("anqicms/%s", config.Version))

	return req
}
