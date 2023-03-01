package provider

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/parnurzeal/gorequest"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

const AnqiApi = "https://www.anqicms.com/auth"

type AnqiLoginResult struct {
	Code int                    `json:"code"`
	Msg  string                 `json:"msg"`
	Data *config.AnqiUserConfig `json:"data"`
}

type AnqiTemplateResult struct {
	Code int                    `json:"code"`
	Msg  string                 `json:"msg"`
	Data response.DesignPackage `json:"data"`
}

type AnqiDownloadTemplateResult struct {
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

type AnqiPseudoRequest struct {
	Title        string `json:"title"`
	Content      string `json:"content"`
	TextLength   int64  `json:"text_length"`
	PseudoRemain int64  `json:"pseudo_remain"`
}

type AnqiPseudoResult struct {
	Code int               `json:"code"`
	Msg  string            `json:"msg"`
	Data AnqiPseudoRequest `json:"data"`
}

type AnqiTranslateRequest struct {
	Title      string `json:"title"`
	Content    string `json:"content"`
	From       string `json:"from"`
	To         string `json:"to"`
	TextLength int64  `json:"text_length"`
	TextRemain int64  `json:"text_remain"`
	Cost       bool   `json:"cost"`
}

type AnqiTranslateResult struct {
	Code int                  `json:"code"`
	Msg  string               `json:"msg"`
	Data AnqiTranslateRequest `json:"data"`
}

// AnqiLogin
// anqi 账号只需要登录一次，全部站点通用，信息记录在
func (w *Website) AnqiLogin(req *request.AnqiLoginRequest) error {
	defaultSite := CurrentSite(nil)
	if w.Id == 1 {
		defaultSite = w
	}
	// 重置
	config.AnqiUser = config.AnqiUserConfig{}
	_ = defaultSite.SaveSettingValue(AnqiSettingKey, config.AnqiUser)
	var result AnqiLoginResult
	_, body, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/login").Send(req).EndStruct(&result)

	if len(errs) > 0 {
		library.DebugLog(config.ExecPath+"cache/", "error.log", string(body))
		return errs[0]
	}

	if result.Code != 0 {
		return errors.New(result.Msg)
	}

	// login success
	config.AnqiUser = *result.Data
	config.AnqiUser.LoginTime = time.Now().Unix()
	config.AnqiUser.CheckTime = config.AnqiUser.LoginTime
	err := defaultSite.SaveSettingValue(AnqiSettingKey, config.AnqiUser)
	if err != nil {
		return err
	}

	return nil
}

func (w *Website) AnqiCheckLogin(force bool) {
	if config.AnqiUser.AuthId == 0 {
		return
	}
	if !force && config.AnqiUser.CheckTime > time.Now().Add(-3600*time.Second).Unix() {
		return
	}
	defaultSite := CurrentSite(nil)
	if w.Id == 1 {
		defaultSite = w
	}
	var result AnqiLoginResult
	_, body, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/check").Send(config.AnqiUser).EndStruct(&result)

	if len(errs) > 0 {
		library.DebugLog(config.ExecPath+"cache/", "error.log", string(body))
		config.AnqiUser.CheckTime = time.Now().Unix()
		return
	}

	if result.Code != 0 {
		// 重置
		config.AnqiUser = config.AnqiUserConfig{}
		_ = defaultSite.SaveSettingValue(AnqiSettingKey, config.AnqiUser)
		return
	}

	// login success
	if result.Data != nil {
		config.AnqiUser.AuthId = result.Data.AuthId
		config.AnqiUser.UserName = result.Data.UserName
		config.AnqiUser.ExpireTime = result.Data.ExpireTime
		config.AnqiUser.PseudoRemain = result.Data.PseudoRemain
		config.AnqiUser.TranslateRemain = result.Data.TranslateRemain
		config.AnqiUser.FreeTransRemain = result.Data.FreeTransRemain
		config.AnqiUser.Status = result.Data.Status
	}
	config.AnqiUser.CheckTime = time.Now().Unix()
	_ = defaultSite.SaveSettingValue(AnqiSettingKey, config.AnqiUser)
}

func GetAuthInfo() *config.AnqiUserConfig {
	config.AnqiUser.Valid = config.AnqiUser.ExpireTime > time.Now().Unix()

	return &config.AnqiUser
}

func (w *Website) AnqiShareTemplate(req *request.AnqiTemplateRequest) error {
	if config.AnqiUser.AuthId == 0 {
		return errors.New("请先登录 AnqiCMS 账号")
	}
	design, err := w.GetDesignInfo(req.Package, false)
	if err != nil {
		return err
	}
	if req.AutoBackup {
		// 先自动备份
		err = w.BackupDesignData(req.Package)
		if err != nil {
			return err
		}
	}
	// 需要先推送design
	var result AnqiTemplateResult
	designData, err := w.CreateDesignZip(design.Package)
	if err != nil {
		return err
	}
	attach, err := w.AnqiUploadAttachment(designData.Bytes(), design.Package+".zip")
	if err != nil {
		return err
	}
	req.TemplatePath = attach.FileLocation
	req.TemplateType = design.TemplateType
	req.TemplateId = design.TemplateId
	// 开始提交数据
	_, body, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/template/share").Send(req).EndStruct(&result)
	if len(errs) > 0 {
		library.DebugLog(config.ExecPath+"cache/", "error.log", string(body))
		return errs[0]
	}

	if result.Code != 0 {
		return errors.New(result.Msg)
	}
	design.TemplateId = result.Data.TemplateId
	design.AuthId = result.Data.AuthId
	design.Name = result.Data.Name
	design.Version = result.Data.Version
	design.Description = result.Data.Description
	design.Author = result.Data.Author
	design.Homepage = result.Data.Homepage
	err = w.writeDesignInfo(design)

	return err
}

func (w *Website) AnqiSendFeedback(req *request.AnqiFeedbackRequest) error {
	if config.AnqiUser.AuthId == 0 {
		return errors.New("请先登录 AnqiCMS 账号")
	}
	req.Version = config.Version
	req.Platform = runtime.GOOS
	req.Domain = w.System.BaseUrl
	// 开始提交数据
	var result AnqiDownloadTemplateResult
	_, body, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/feedback").Send(req).EndStruct(&result)
	if len(errs) > 0 {
		library.DebugLog(config.ExecPath+"cache/", "error.log", string(body))
		return errs[0]
	}

	if result.Code != 0 {
		return errors.New(result.Msg)
	}

	return nil
}

func (w *Website) AnqiUploadAttachment(data []byte, name string) (*AnqiAttachment, error) {
	if config.AnqiUser.AuthId == 0 {
		return nil, errors.New("请先登录 AnqiCMS 账号")
	}

	var result AnqiAttachmentResult
	_, body, errs := w.NewAuthReq(gorequest.TypeMultipart).Post(AnqiApi+"/template/upload").SendFile(data, name, "attach").EndStruct(&result)
	if len(errs) > 0 {
		library.DebugLog(config.ExecPath+"cache/", "error.log", string(body))
		return nil, errs[0]
	}
	if result.Code != 0 {
		return nil, errors.New(result.Msg)
	}

	return &result.Data, nil
}

func (w *Website) AnqiDownloadTemplate(req *request.AnqiTemplateRequest) error {
	var result AnqiDownloadTemplateResult

	_, body, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/template/download").Send(req).EndStruct(&result)
	if len(errs) > 0 {
		library.DebugLog(config.ExecPath+"cache/", "error.log", string(body))
		return errs[0]
	}
	if result.Code != 0 {
		return errors.New(result.Msg)
	}

	downloadUrl, ok := result.Data.(string)
	if !ok {
		return errors.New("读取下载地址错误")
	}

	_, body, errs = w.NewAuthReq(gorequest.TypeHTML).Get(downloadUrl).EndBytes()
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
	err := w.UploadDesignZip(file, info)
	if err != nil {
		return err
	}

	return nil
}

func (w *Website) AnqiPseudoArticle(archive *model.Archive) error {
	archiveData, err := w.GetArchiveDataById(archive.Id)
	if err != nil {
		return err
	}
	req := &AnqiPseudoRequest{
		Title:   archive.Title,
		Content: archiveData.Content,
	}
	var result AnqiPseudoResult
	_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/pseudo").Send(req).EndStruct(&result)
	if len(errs) > 0 {
		return errs[0]
	}
	if result.Code != 0 {
		return errors.New(result.Msg)
	}
	archive.Title = result.Data.Title
	if len(result.Data.Content) > 1000 {
		archive.Description = string([]rune(result.Data.Content)[:1000])
	} else if len(result.Data.Content) > 0 {
		archive.Description = result.Data.Content
	}
	archive.HasPseudo = 1
	err = w.DB.Save(archive).Error
	// 再保存内容
	archiveData.Content = result.Data.Content
	w.DB.Save(archiveData)

	return nil
}

func (w *Website) AnqiTranslateArticle(archive *model.Archive) error {
	archiveData, err := w.GetArchiveDataById(archive.Id)
	if err != nil {
		return err
	}
	req := &AnqiTranslateRequest{
		Title:   archive.Title,
		Content: archiveData.Content,
	}
	var result AnqiTranslateResult
	_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/translate").Send(req).EndStruct(&result)
	if len(errs) > 0 {
		return errs[0]
	}
	if result.Code != 0 {
		return errors.New(result.Msg)
	}
	archive.Title = result.Data.Title
	if len(result.Data.Content) > 1000 {
		archive.Description = string([]rune(result.Data.Content)[:1000])
	} else if len(result.Data.Content) > 0 {
		archive.Description = result.Data.Content
	}
	err = w.DB.Save(archive).Error
	// 再保存内容
	archiveData.Content = result.Data.Content
	w.DB.Save(archiveData)

	return nil
}

func (w *Website) NewAuthReq(contentType string) *gorequest.SuperAgent {
	req := gorequest.New().
		SetDoNotClearSuperAgent(true).
		TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
		Timeout(30*time.Second).
		Type(contentType).
		Set("token", config.AnqiUser.Token).
		//set key header
		Set("domain", w.System.BaseUrl).
		//set oem header
		Set("User-Agent", fmt.Sprintf("anqicms/%s", config.Version))

	return req
}

// Restart first need to stop iris. so it will call after iris shutdown complete.
func Restart() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	args := os.Args
	env := os.Environ()
	// Windows does not support exec syscall.
	if runtime.GOOS == "windows" {
		cmd := exec.Command(self, args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = env
		err = cmd.Run()
		if err == nil {
			os.Exit(0)
		}
		return err
	}
	return syscall.Exec(self, args, env)
}
