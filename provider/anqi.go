package provider

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/parnurzeal/gorequest"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
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

type AnqiAiRequest struct {
	Keyword    string `json:"keyword"`
	Language   string `json:"language"`
	Title      string `json:"title"`
	Demand     string `json:"demand,omitempty"`
	Content    string `json:"content"`
	TextLength int64  `json:"text_length"`
	AiRemain   int64  `json:"ai_remain"`
}

type AnqiAiResult struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data AnqiAiRequest `json:"data"`
}

type AnqiAiStreamResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data string `json:"data"`
}

type AnqiSensitiveResult struct {
	Code int      `json:"code"`
	Msg  string   `json:"msg"`
	Data []string `json:"data"`
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
		config.AnqiUser.TranslateRemain = result.Data.TranslateRemain
		config.AnqiUser.AiRemain = result.Data.AiRemain
		config.AnqiUser.Integral = result.Data.Integral
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
	archive.Description = library.ParseDescription(strings.ReplaceAll(library.StripTags(result.Data.Content), "\n", " "))
	err = w.DB.Save(archive).Error
	// 再保存内容
	archiveData.Content = result.Data.Content
	w.DB.Save(archiveData)

	return nil
}

func (w *Website) AnqiAiPseudoArticle(archive *model.Archive) error {
	archiveData, err := w.GetArchiveDataById(archive.Id)
	if err != nil {
		return err
	}
	req := &AnqiAiRequest{
		Title:    archive.Title,
		Content:  archiveData.Content,
		Language: w.System.Language, // 以系统语言为标准
	}
	if w.AiGenerateConfig.Open {
		req, err = w.SelfAiPseudoResult(req)
		if err != nil {
			return err
		}
	} else {
		var result AnqiAiResult
		_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/ai/pseudo").Send(req).EndStruct(&result)
		if len(errs) > 0 {
			return errs[0]
		}
		if result.Code != 0 {
			return errors.New(result.Msg)
		}
		req = &result.Data
	}
	archive.Title = req.Title
	archive.Description = library.ParseDescription(strings.ReplaceAll(library.StripTags(req.Content), "\n", " "))
	archive.HasPseudo = 1
	err = w.DB.Save(archive).Error
	// 再保存内容
	archiveData.Content = req.Content
	w.DB.Save(archiveData)

	return nil
}

func (w *Website) AnqiAiGenerateArticle(keyword *model.Keyword) (int, error) {
	// 检查是否采集过
	if w.checkArticleExists(keyword.Title, "", "") {
		//log.Println("已存在于数据库", keyword.Title)
		return 1, nil
	}

	var err error
	req := &AnqiAiRequest{
		Keyword:  keyword.Title,
		Language: w.System.Language, // 以系统语言为标准
	}
	if w.AiGenerateConfig.Open {
		req, err = w.SelfAiGenerateResult(req)
		if err != nil {
			return 0, err
		}
	} else {
		var result AnqiAiResult
		_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/ai/generate").Send(req).EndStruct(&result)
		if len(errs) > 0 {
			return 0, errs[0]
		}
		if result.Code != 0 {
			return 0, errors.New(result.Msg)
		}
		req = &result.Data
	}

	var content = strings.Split(req.Content, "\n")
	if w.CollectorConfig.InsertImage == config.CollectImageInsert && len(w.CollectorConfig.Images) > 0 {
		rand.Seed(time.Now().UnixMicro())
		img := w.CollectorConfig.Images[rand.Intn(len(w.CollectorConfig.Images))]
		index := 2 + rand.Intn(len(content)-3)
		content = append(content, "")
		copy(content[index+1:], content[index:])
		content[index] = "<img src='" + img + "'/>"
	}
	categoryId := keyword.CategoryId
	if categoryId == 0 {
		if w.CollectorConfig.CategoryId == 0 {
			var category model.Category
			w.DB.Where("module_id = 1").Take(&category)
			w.CollectorConfig.CategoryId = category.Id
		}
		categoryId = w.CollectorConfig.CategoryId
	}

	archive := request.Archive{
		Title:      req.Title,
		ModuleId:   0,
		CategoryId: categoryId,
		Keywords:   keyword.Title,
		Content:    strings.Join(content, "\n"),
		KeywordId:  keyword.Id,
		OriginUrl:  keyword.Title,
		ForceSave:  true,
	}
	if w.CollectorConfig.SaveType == 0 {
		archive.Draft = true
	} else {
		archive.Draft = false
	}
	res, err := w.SaveArchive(&archive)
	if err != nil {
		log.Println("保存AI文章出错：", archive.Title, err.Error())
		return 0, nil
	}
	log.Println(res.Id, res.Title)

	return 1, nil
}

type StreamData struct {
	Content  string `json:"content"`
	Err      string `json:"err"`
	Finished bool   `json:"finished"`
}

type AiStreamStore struct {
	mu   sync.Mutex
	list map[string]*StreamData
}

func (a *AiStreamStore) UpdateStreamData(streamId, content, err string, finished bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, ok := a.list[streamId]
	if !ok {
		data = &StreamData{}
		a.list[streamId] = data
	}
	data.Content = data.Content + content
	data.Err = err
	data.Finished = finished
}

func (a *AiStreamStore) LoadStreamData(streamId string) (content, err string, finished bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, ok := a.list[streamId]
	if !ok {
		return "", "", true
	}
	content = data.Content
	err = data.Err
	finished = data.Finished
	data.Content = ""

	return
}

func (a *AiStreamStore) DeleteStreamData(streamId string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.list, streamId)
}

var AiStreamResults = AiStreamStore{
	mu:   sync.Mutex{},
	list: map[string]*StreamData{},
}

func (w *Website) AnqiLoadStreamData(streamId string) (content, err string, finished bool) {
	return AiStreamResults.LoadStreamData(streamId)
}

func (w *Website) AnqiAiGenerateStream(keyword *request.KeywordRequest) (string, error) {
	req := &AnqiAiRequest{
		Keyword:  keyword.Title,
		Demand:   keyword.Demand,
		Language: w.System.Language, // 以系统语言为标准
	}

	streamId := fmt.Sprintf("a%d", time.Now().UnixMilli())
	if w.AiGenerateConfig.Open {
		if !w.AiGenerateConfig.ApiValid {
			return "", errors.New("接口不可用")
		}
		key := w.GetOpenAIKey()
		if key == "" {
			return "", errors.New("无可用Key")
		}
		prompt := "请根据关键词生成一篇中文文章。关键词：" + req.Keyword
		if req.Language == config.LanguageEn {
			prompt = "Please generate an English article based on the keywords. Keywords: '" + req.Keyword + "'"
		}
		if len(req.Demand) > 0 {
			prompt += "\n" + req.Demand
		}
		stream, err := GetOpenAIStreamResponse(key, prompt)
		if err != nil {
			msg := err.Error()
			re, _ := regexp.Compile(`code: (\d+),`)
			match := re.FindStringSubmatch(msg)
			if len(match) > 1 {
				if match[1] == "401" || match[1] == "429" {
					// Key 已失效
					w.SetOpenAIKeyInvalid(key)
				}
			}
			return "", err
		}
		go func() {
			defer stream.Close()
			for {
				resp, err2 := stream.Recv()
				if errors.Is(err2, io.EOF) {
					break
				}
				if err2 != nil {
					err = err2
					fmt.Printf("\nStream error: %v\n", err2)
					break
				}
				AiStreamResults.UpdateStreamData(streamId, resp.Choices[0].Delta.Content, "", false)
			}
			if err != nil {
				if strings.Contains(err.Error(), "You exceeded your current quota") {
					w.SetOpenAIKeyInvalid(key)
				}

				AiStreamResults.UpdateStreamData(streamId, "", err.Error(), true)
			} else {
				AiStreamResults.UpdateStreamData(streamId, "", "", true)
			}

			time.AfterFunc(5*time.Second, func() {
				AiStreamResults.DeleteStreamData(streamId)
			})
		}()
	} else {
		buf, _ := json.Marshal(req)

		client := &http.Client{
			Timeout: 180 * time.Second,
		}
		anqiReq, err := http.NewRequest("POST", AnqiApi+"/ai/stream", bytes.NewReader(buf))
		if err != nil {
			return "", err
		}
		anqiReq.Header.Add("token", config.AnqiUser.Token)
		anqiReq.Header.Add("User-Agent", fmt.Sprintf("anqicms/%s", config.Version))
		anqiReq.Header.Add("domain", w.System.BaseUrl)
		resp, err := client.Do(anqiReq)
		if err != nil {
			return "", err
		}

		go func() {
			// 开始处理
			defer resp.Body.Close()
			reader := bufio.NewReader(resp.Body)
			for {
				line, err2 := reader.ReadBytes('\n')
				var isEof bool
				if err2 != nil {
					isEof = true
				}
				var aiResponse AnqiAiStreamResult
				err2 = json.Unmarshal(line, &aiResponse)
				if err2 != nil {
					if isEof {
						AiStreamResults.UpdateStreamData(streamId, "", "", true)
						break
					}
					continue
				}
				AiStreamResults.UpdateStreamData(streamId, aiResponse.Data, "", isEof)

				if aiResponse.Code != 0 {
					AiStreamResults.UpdateStreamData(streamId, "", aiResponse.Msg, true)
					return
				}
			}
		}()
	}

	return streamId, nil
}

func (w *Website) AnqiSyncSensitiveWords() error {
	var result AnqiSensitiveResult
	_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/sensitive/sync").EndStruct(&result)
	if len(errs) > 0 {
		return errs[0]
	}
	if result.Code != 0 {
		return errors.New(result.Msg)
	}

	if len(result.Data) > 0 {
		w.SensitiveWords = result.Data
		w.SaveSettingValue(SensitiveWordsKey, w.SensitiveWords)
	}

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
