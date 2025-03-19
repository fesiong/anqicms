package provider

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
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
	"unicode/utf8"

	"github.com/parnurzeal/gorequest"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

const AnqiApi = "https://auth.anqicms.com/auth"

var ErrDoing = errors.New("doing")

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

type AnqiAiRequest struct {
	Keyword    string `json:"keyword"`
	Demand     string `json:"demand"`
	Language   string `json:"language"`
	ToLanguage string `json:"to_language"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	TextLength int64  `json:"text_length"`
	AiRemain   int64  `json:"ai_remain"`
	Async      bool   `json:"async"`      // 是否异步处理
	Cost       bool   `json:"cost"`       // 支付需要支付
	ArticleId  int64  `json:"article_id"` // 本地的文档ID

	Type  int   `json:"type"`
	ReqId int64 `json:"req_id"`
}

type AnqiAiPlanRequest struct {
	ReqId int64 `json:"req_id"`
}

// AnqiAiResult 是结合了2中结构体的内容，一种是plan，一种是article
type AnqiAiResult struct {
	Id          int64  `json:"id"`
	CreatedTime int64  `json:"created_time"`
	Type        int    `json:"type"`
	Language    string `json:"language"`
	ToLanguage  string `json:"to_language"`
	Keyword     string `json:"keyword"`
	Demand      string `json:"demand"`
	PayCount    int64  `json:"pay_count"`
	Status      int    `json:"status"`

	Title         string `json:"title"`
	Content       string `json:"content"`
	ReqId         int64  `json:"req_id"`
	ArticleId     int64  `json:"-"`
	UseSelf       bool   `json:"-"`
	OriginTitle   string `json:"origin_title"`
	OriginContent string `json:"origin_content"`
}

type AnqiAiResponse struct {
	Code int          `json:"code"`
	Msg  string       `json:"msg"`
	Data AnqiAiResult `json:"data"`
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

type AnqiExtractResult struct {
	Code int      `json:"code"`
	Msg  string   `json:"msg"`
	Data []string `json:"data"`
}

type AnqiTranslateHtmlRequest struct {
	Uri         string   `json:"uri"`
	Html        string   `json:"html"`
	Language    string   `json:"language"`    // 源语言，如果不传，则会自动推断
	ToLanguage  string   `json:"to_language"` // 目标语言，必传
	IgnoreClass []string `json:"ignore_class"`
	IgnoreId    []string `json:"ignore_id"`
}

type AnqiTranslateHtmlResult struct {
	Uri        string `json:"uri"` // 这个一般不用传
	Html       string `json:"html"`
	Language   string `json:"language"`
	ToLanguage string `json:"to_language"`
	Status     int    `json:"status"`
	Count      int64  `json:"count"`     // 总量
	UseCount   int64  `json:"use_count"` // 用量
	Remark     string `json:"remark"`
}

type AnqiTranslateHtmlResponse struct {
	Code int                     `json:"code"`
	Msg  string                  `json:"msg"`
	Data AnqiTranslateHtmlResult `json:"data"`
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
		return errors.New(w.Tr("PleaseLogInToAnqicmsAccountFirst"))
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
		return errors.New(w.Tr("PleaseLogInToAnqicmsAccountFirst"))
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
		return nil, errors.New(w.Tr("PleaseLogInToAnqicmsAccountFirst"))
	}

	// 上传之前，先进行图片处理，减少数据的传输
	img, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		// 处理成 webp
		buf, _, err := encodeImage(img, "webp", 85)
		if err == nil {
			data = buf
		}
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
		return errors.New(w.Tr("ErrorInReadingDownloadAddress"))
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
	err := w.UploadDesignZip(file, info, "")
	if err != nil {
		return err
	}

	return nil
}

// AnqiTranslateString 翻译任意文本内容
func (w *Website) AnqiTranslateString(req *AnqiAiRequest) (result *AnqiAiResult, err error) {
	if req.Language == "" {
		isEnglish := CheckContentIsEnglish(req.Title)
		if isEnglish {
			req.Language = config.LanguageEn
		} else {
			req.Language = config.LanguageZh
		}
	}
	if req.ToLanguage == "" {
		return nil, errors.New(w.Tr("PleaseSelectTargetLanguage"))
	}
	// 检查是否在记录中存在结果。如果是有记录，则标记 useSelf = true
	logData := QueryTranslateLog(&AnqiAiResult{Title: req.Title, OriginContent: req.Content, ToLanguage: req.ToLanguage})
	if logData != nil {
		result = &AnqiAiResult{
			Type:          config.AiArticleTypeTranslate,
			Language:      req.Language,
			ToLanguage:    req.ToLanguage,
			Keyword:       req.Keyword,
			Demand:        req.Demand,
			Status:        config.AiArticleStatusCompleted,
			Title:         logData.Title,
			Content:       logData.Content,
			OriginTitle:   logData.OriginTitle,
			OriginContent: logData.OriginContent,
			ArticleId:     req.ArticleId,
			UseSelf:       true,
		}
		return result, nil
	}

	originTitle := req.Title
	originContent := req.Content
	// 如果设置了使用AI翻译，则使用自己翻译
	if w.PluginTranslate.Engine != config.TranslateEngineDefault {
		req, err = w.SelfAiTranslateResult(req)
		if err != nil {
			return nil, err
		}
		result = &AnqiAiResult{
			Type:          config.AiArticleTypeTranslate,
			Language:      req.Language,
			ToLanguage:    req.ToLanguage,
			Keyword:       req.Keyword,
			Demand:        req.Demand,
			Status:        config.AiArticleStatusCompleted,
			Title:         req.Title,
			Content:       req.Content,
			OriginTitle:   originTitle,
			OriginContent: originContent,
			ArticleId:     req.ArticleId,
			UseSelf:       true,
		}

		// 记录翻译记录
		AddTranslateLog(result)

		return result, nil
	} else {
		var res AnqiAiResponse
		_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/translate").Send(req).EndStruct(&res)
		if len(errs) > 0 {
			return nil, errs[0]
		}
		if res.Code != 0 {
			return nil, errors.New(res.Msg)
		}
		if res.Data.Status != config.AiArticleStatusCompleted {
			res.Data.Status = config.AiArticleStatusDoing
		}
		res.Data.UseSelf = false
		res.Data.ArticleId = req.ArticleId
		if res.Data.Status == config.AiArticleStatusCompleted {
			// 记录翻译记录
			res.Data.OriginTitle = originTitle
			res.Data.OriginContent = originContent
			AddTranslateLog(&res.Data)
		}

		return &res.Data, nil
	}
}

func (w *Website) AnqiAiPseudoArticle(archive *model.Archive, isDraft bool) error {
	archiveData, err := w.GetArchiveDataById(archive.Id)
	if err != nil {
		return err
	}

	req := &AnqiAiRequest{
		Title:    archive.Title,
		Content:  archiveData.Content,
		Language: w.System.Language, // 以系统语言为标准
		Async:    true,              // 异步返回结果
	}
	if w.AiGenerateConfig.AiEngine != config.AiEngineDefault {
		req, err = w.SelfAiPseudoResult(req)
		if err != nil {
			return err
		}
		archive.Title = req.Title
		archive.Description = library.ParseDescription(strings.ReplaceAll(library.StripTags(req.Content), "\n", " "))
		archive.HasPseudo = 1
		tx := w.DB
		if isDraft {
			tx = tx.Model(&model.ArchiveDraft{})
		} else {
			tx = tx.Model(&model.Archive{})
		}
		err = tx.Where("id = ?", archive.Id).UpdateColumns(map[string]interface{}{
			"title":       archive.Title,
			"description": archive.Description,
			"has_pseudo":  archive.HasPseudo,
		}).Error
		// 再保存内容
		archiveData.Content = req.Content
		w.DB.Save(archiveData)
		// 添加到plan，并标记完成
		result := AnqiAiResult{
			Type:      config.AiArticleTypeAiPseudo,
			Language:  req.Language,
			Keyword:   req.Keyword,
			Demand:    req.Demand,
			Status:    config.AiArticleStatusCompleted,
			Title:     req.Title,
			Content:   req.Content,
			ArticleId: archive.Id,
		}
		_, err = w.SaveAiArticlePlan(&result, true)
	} else {
		var result AnqiAiResponse
		_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/ai/pseudo").Send(req).EndStruct(&result)
		if len(errs) > 0 {
			return errs[0]
		}
		if result.Code != 0 {
			return errors.New(result.Msg)
		}
		// 添加到plan中
		result.Data.Status = config.AiArticleStatusDoing
		result.Data.ArticleId = archive.Id
		_, err = w.SaveAiArticlePlan(&result.Data, false)
		if err != nil {
			return err
		}
	}

	return nil
}

// AnqiAiGenerateArticle 该函数尝试采用同步返回
func (w *Website) AnqiAiGenerateArticle(keyword *model.Keyword) (int, error) {
	// 检查是否生成过
	_, err := w.GetAiArticlePlanByKeyword(config.AiArticleTypeGenerate, keyword.Title)
	if err == nil {
		//log.Println("已存在于数据库", keyword.Title)
		return 1, nil
	}

	req := &AnqiAiRequest{
		Keyword:  keyword.Title,
		Language: w.System.Language, // 以系统语言为标准
		Async:    true,
	}
	if w.AiGenerateConfig.AiEngine != config.AiEngineDefault {
		req, err = w.SelfAiGenerateResult(req)
		if err != nil {
			return 0, err
		}
		var content = strings.Split(req.Content, "\n")
		if w.AiGenerateConfig.InsertImage == config.CollectImageInsert && len(w.AiGenerateConfig.Images) > 0 {
			rd := rand.New(rand.NewSource(time.Now().UnixNano()))
			img := w.AiGenerateConfig.Images[rd.Intn(len(w.AiGenerateConfig.Images))]
			index := len(content) / 3
			content = append(content, "")
			copy(content[index+1:], content[index:])
			imgTag := "<img src='" + img + "' alt='" + req.Title + "' />"
			// ![新的图片](http://xxx/xxx.webp)
			if w.Content.Editor == "markdown" {
				imgTag = fmt.Sprintf("![%s](%s)", req.Title, img)
			}
			content[index] = imgTag
		}
		if w.AiGenerateConfig.InsertImage == config.CollectImageCategory {
			// 根据分类每次只取其中一张
			img := w.GetRandImageFromCategory(w.AiGenerateConfig.ImageCategoryId, keyword.Title)
			if len(img) > 0 {
				index := len(content) / 3
				content = append(content, "")
				copy(content[index+1:], content[index:])
				imgTag := "<img src='" + img + "' alt='" + req.Title + "' />"
				// ![新的图片](http://xxx/xxx.webp)
				if w.Content.Editor == "markdown" {
					imgTag = fmt.Sprintf("![%s](%s)", req.Title, img)
				}
				content[index] = imgTag
			}
		}

		categoryId := keyword.CategoryId
		if categoryId == 0 {
			if len(w.AiGenerateConfig.CategoryIds) > 0 {
				categoryId = w.AiGenerateConfig.CategoryIds[rand.New(rand.NewSource(time.Now().UnixNano())).Intn(len(w.AiGenerateConfig.CategoryIds))]
			} else if w.AiGenerateConfig.CategoryId > 0 {
				categoryId = w.AiGenerateConfig.CategoryId
			}
			if categoryId == 0 {
				var category model.Category
				w.DB.Where("module_id = 1").Take(&category)
				w.AiGenerateConfig.CategoryIds = []uint{category.Id}
				categoryId = category.Id
			}
		}

		archiveReq := request.Archive{
			Title:      req.Title,
			ModuleId:   0,
			CategoryId: categoryId,
			Keywords:   keyword.Title,
			Content:    strings.Join(content, "\n"),
			KeywordId:  keyword.Id,
			OriginUrl:  keyword.Title,
			ForceSave:  true,
		}
		if w.AiGenerateConfig.SaveType == 0 {
			archiveReq.Draft = true
		} else {
			archiveReq.Draft = false
		}
		archive, err2 := w.SaveArchive(&archiveReq)
		if err2 != nil {
			log.Println("保存AI文章出错：", archiveReq.Title, err2.Error())
			return 0, nil
		}
		// 添加到plan，并标记完成
		result := AnqiAiResult{
			Type:      config.AiArticleTypeGenerate,
			Language:  req.Language,
			Keyword:   req.Keyword,
			Demand:    req.Demand,
			Status:    config.AiArticleStatusCompleted,
			Title:     req.Title,
			Content:   req.Content,
			ArticleId: archive.Id,
		}
		_, err = w.SaveAiArticlePlan(&result, true)
	} else {
		var result AnqiAiResponse
		_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/ai/generate").Send(req).EndStruct(&result)
		if len(errs) > 0 {
			return 0, errs[0]
		}
		if result.Code != 0 {
			return 0, errors.New(result.Msg)
		}
		// 添加到plan中
		result.Data.Status = config.AiArticleStatusDoing
		plan, err2 := w.SaveAiArticlePlan(&result.Data, false)
		if err2 != nil {
			return 0, err2
		}
		// 同步等待数据, 最多等待5分钟, 10秒检查一次
		for i := 0; i < 30; i++ {
			time.Sleep(10 * time.Second)
			err = w.AnqiSyncAiPlanResult(plan)
			if err == nil {
				break
			}
			if !errors.Is(err, ErrDoing) {
				// 不是继续等待的类型，跳出
				break
			}
		}
	}

	return 1, nil
}

func (w *Website) AnqiSyncAiPlanResult(plan *model.AiArticlePlan) error {
	var err error
	// 重新检查状态
	plan, err = w.GetAiArticlePlanById(plan.Id)
	if err != nil {
		w.DB.Model(plan).UpdateColumn("status", config.AiArticleStatusError)
		return err
	}
	if plan.ReqId == 0 {
		w.DB.Model(plan).UpdateColumn("status", config.AiArticleStatusError)
		return errors.New("req-id is empty")
	}
	if plan.Status != config.AiArticleStatusDoing {
		return errors.New(w.Tr("ThePlanHasBeenProcessed"))
	}
	req := &AnqiAiRequest{
		ReqId: plan.ReqId,
	}
	var result AnqiAiResponse
	_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/ai/syncplan").Send(req).EndStruct(&result)
	if len(errs) > 0 {
		return errs[0]
	}
	if result.Code != 0 {
		plan.Status = config.AiArticleStatusError
		w.DB.Model(plan).UpdateColumn("status", plan.Status)
		return errors.New(result.Msg)
	}
	if result.Data.Status == config.AiArticleStatusDoing || result.Data.Status == config.AiArticleStatusWaiting {
		// 进行中，跳过
		return ErrDoing
	}
	if result.Data.Status == config.AiArticleStatusCompleted {
		// 异步返回的不记录log
		// 成功
		if plan.ArticleId > 0 {
			// 更新文章
			// 如果是草稿，则更新草稿箱，查询正式表不存在的话，就认为是草稿
			_, err = w.GetArchiveById(plan.ArticleId)
			tx := w.DB
			if err != nil {
				// 不存在，视为草稿
				tx = tx.Model(&model.ArchiveDraft{})
			} else {
				tx = tx.Model(&model.Archive{})
			}
			tx.Where("`id` = ?", plan.ArticleId).UpdateColumns(map[string]interface{}{
				"title":       result.Data.Title,
				"description": library.ParseDescription(strings.ReplaceAll(library.StripTags(result.Data.Content), "\n", " ")),
			})
			// 再保存内容
			w.DB.Model(&model.ArchiveData{}).Where("`id` = ?", plan.ArticleId).UpdateColumn("content", result.Data.Content)
			// 更新plan
			plan.Status = config.AiArticleStatusCompleted
			w.DB.Model(plan).UpdateColumn("status", plan.Status)
		} else {
			// 先检查是否已经入库过了
			_, err2 := w.GetArchiveByTitle(result.Data.Title)
			if err2 == nil {
				// 已存在
				plan.Status = config.AiArticleStatusCompleted
				w.DB.Model(plan).UpdateColumn("status", plan.Status)
				return nil
			}
			var content = strings.Split(req.Content, "\n")
			if w.AiGenerateConfig.InsertImage == config.CollectImageInsert && len(w.AiGenerateConfig.Images) > 0 {
				rd := rand.New(rand.NewSource(time.Now().UnixNano()))
				img := w.AiGenerateConfig.Images[rd.Intn(len(w.AiGenerateConfig.Images))]
				index := len(content) / 3
				content = append(content, "")
				copy(content[index+1:], content[index:])
				imgTag := "<img src='" + img + "' alt='" + req.Title + "' />"
				content[index] = imgTag
			}
			if w.AiGenerateConfig.InsertImage == config.CollectImageCategory {
				// 根据分类每次只取其中一张
				img := w.GetRandImageFromCategory(w.AiGenerateConfig.ImageCategoryId, result.Data.Keyword)
				if len(img) > 0 {
					index := len(content) / 3
					content = append(content, "")
					copy(content[index+1:], content[index:])
					imgTag := "<img src='" + img + "' alt='" + req.Title + "' />"
					content[index] = imgTag
				}
			}
			var keyword *model.Keyword
			categoryId := w.AiGenerateConfig.CategoryId
			if len(w.AiGenerateConfig.CategoryIds) > 0 {
				categoryId = w.AiGenerateConfig.CategoryIds[rand.New(rand.NewSource(time.Now().UnixNano())).Intn(len(w.AiGenerateConfig.CategoryIds))]
			}
			keyword, err = w.GetKeywordByTitle(plan.Keyword)
			if err == nil {
				if keyword.CategoryId > 0 {
					categoryId = keyword.CategoryId
				}
			}
			if categoryId == 0 {
				var category model.Category
				w.DB.Where("module_id = 1").Take(&category)
				w.AiGenerateConfig.CategoryIds = []uint{category.Id}
				categoryId = category.Id
			}

			archive := request.Archive{
				Title:      result.Data.Title,
				ModuleId:   0,
				CategoryId: categoryId,
				Keywords:   result.Data.Keyword,
				Content:    result.Data.Content,
				ForceSave:  true,
			}
			if keyword != nil {
				archive.KeywordId = keyword.Id
				archive.OriginUrl = keyword.Title
			}
			if w.AiGenerateConfig.SaveType == 0 {
				archive.Draft = true
			} else {
				archive.Draft = false
			}
			res, err := w.SaveArchive(&archive)
			if err != nil {
				log.Println("保存AI文章出错：", archive.Title, err.Error())
				return err
			}
			// 更新plan
			plan.ArticleId = res.Id
			plan.Status = config.AiArticleStatusCompleted
			w.DB.Model(plan).UpdateColumns(map[string]interface{}{
				"status":     plan.Status,
				"article_id": plan.ArticleId,
			})
		}
		// 如果是AI改写
		if plan.Type == config.AiArticleTypeAiPseudo {
			w.DB.Model(&model.Archive{}).Where("`id` = ?", plan.ArticleId).UpdateColumn("has_pseudo", 1)
		}
	} else {
		// 其它类型
		plan.Status = result.Data.Status
		w.DB.Model(plan).UpdateColumn("status", plan.Status)
		return errors.New(w.Tr("OtherErrors"))
	}

	return nil
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
	if w.AiGenerateConfig.AiEngine != config.AiEngineDefault {
		prompt := "请根据关键词生成一篇中文文章。关键词：" + req.Keyword
		if req.Language == config.LanguageEn {
			prompt = "Please generate an English article based on the keywords. Keywords: '" + req.Keyword + "'"
		}
		if len(req.Demand) > 0 {
			prompt += "\n" + req.Demand
		}
		if w.AiGenerateConfig.AiEngine == config.AiEngineOpenAI {
			if !w.AiGenerateConfig.ApiValid {
				return "", errors.New(w.Tr("InterfaceUnavailable"))
			}
			key := w.GetOpenAIKey()
			if key == "" {
				return "", errors.New(w.Tr("NoAvailableKey"))
			}
			stream, err := w.GetOpenAIStreamResponse(key, prompt)
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
		} else if w.AiGenerateConfig.AiEngine == config.AiEngineSpark {
			buf, err := GetSparkStream(w.AiGenerateConfig.Spark, prompt)
			if err != nil {
				return "", err
			}
			go func() {
				for {
					line := <-buf

					if line == "EOF" {
						break
					}

					AiStreamResults.UpdateStreamData(streamId, line, "", false)
				}
				AiStreamResults.UpdateStreamData(streamId, "", "", true)

				time.AfterFunc(5*time.Second, func() {
					AiStreamResults.DeleteStreamData(streamId)
				})
			}()
		} else {
			// 错误
			return "", errors.New(w.Tr("NoAiGenerationSourceSelected"))
		}
	} else {
		buf, _ := json.Marshal(req)

		client := &http.Client{
			Timeout: 300 * time.Second,
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
		w2 := websites.MustGet(w.Id)
		w2.SensitiveWords = result.Data
		w.SaveSettingValue(SensitiveWordsKey, w.SensitiveWords)
	}

	return nil
}

func (w *Website) AnqiExtractKeywords(req *request.AnqiExtractRequest) ([]string, error) {
	var result AnqiExtractResult

	_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/extract/keywords").Send(req).EndStruct(&result)
	if len(errs) > 0 {
		return nil, errs[0]
	}
	if result.Code != 0 {
		return nil, errors.New(result.Msg)
	}

	if len(result.Data) > 0 {
		return result.Data, nil
	}

	return nil, nil
}

func (w *Website) AnqiExtractDescription(req *request.AnqiExtractRequest) ([]string, error) {
	var result AnqiExtractResult

	_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/extract/summarize").Send(req).EndStruct(&result)
	if len(errs) > 0 {
		return nil, errs[0]
	}
	if result.Code != 0 {
		return nil, errors.New(result.Msg)
	}

	if len(result.Data) > 0 {
		return result.Data, nil
	}

	return nil, nil
}

func AddTranslateLog(result *AnqiAiResult) {
	// translate log 只记录到主站点的库里
	db := GetDefaultDB()

	// md5 的值 = 原始字符串+翻译的语言
	md5Str := library.Md5(result.OriginTitle + "-" + result.OriginContent + "-" + result.ToLanguage)
	logData := model.TranslateLog{
		Language:      result.Language,
		ToLanguage:    result.ToLanguage,
		OriginTitle:   result.OriginTitle,
		OriginContent: result.OriginContent,
		Md5:           md5Str,
		Title:         result.Title,
		Content:       result.Content,
	}

	db.Model(&model.TranslateLog{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "md5"}},
		UpdateAll: true,
	}).Where("`md5` = ?", md5Str).Create(&logData)
}

func QueryTranslateLog(result *AnqiAiResult) *model.TranslateLog {
	// translate log 只记录到主站点的库里
	db := GetDefaultDB()
	// md5 的值 = 原始字符串+翻译的语言
	md5Str := library.Md5(result.OriginTitle + "-" + result.OriginContent + "-" + result.ToLanguage)

	var logData model.TranslateLog
	err := db.Model(&model.TranslateLog{}).Where("`md5` = ?", md5Str).Take(&logData).Error
	if err != nil {
		return nil
	}

	return &logData
}

func AddTranslateHtmlLog(result *AnqiTranslateHtmlResult) {
	// translate log 只记录到主站点的库里
	db := GetDefaultDB()

	logData := model.TranslateHtmlLog{
		Uri:        result.Uri,
		Language:   result.Language,
		ToLanguage: result.ToLanguage,
		Count:      result.Count,
		UseCount:   result.UseCount,
		Status:     result.Status,
		Remark:     result.Remark,
	}

	db.Model(&model.TranslateHtmlLog{}).Create(&logData)
}

// AnqiTranslateHtml 多语言html翻译，只能使用官方接口，从原站语言翻译成目标语言
func (w *Website) AnqiTranslateHtml(req *AnqiTranslateHtmlRequest) (content string, err error) {
	if req.Language == "" {
		req.Language = w.System.Language
	}
	if req.ToLanguage == "" {
		return "", errors.New(w.Tr("PleaseSelectTargetLanguage"))
	}

	var res AnqiTranslateHtmlResponse
	_, _, errs := w.NewAuthReq(gorequest.TypeJSON).Post(AnqiApi + "/translate/html").Send(req).EndStruct(&res)
	if len(errs) > 0 {
		msg := errs[0].Error()
		if utf8.RuneCountInString(msg) > 190 {
			msg = string([]rune(msg)[:190])
		}
		AddTranslateHtmlLog(&AnqiTranslateHtmlResult{
			Uri:        req.Uri,
			Html:       "",
			Language:   req.Language,
			ToLanguage: req.ToLanguage,
			Remark:     msg,
		})
		return "", errs[0]
	}
	if res.Code != 0 {
		msg := res.Msg
		if utf8.RuneCountInString(msg) > 190 {
			msg = string([]rune(msg)[:190])
		}
		AddTranslateHtmlLog(&AnqiTranslateHtmlResult{
			Uri:        req.Uri,
			Html:       "",
			Language:   req.Language,
			ToLanguage: req.ToLanguage,
			Remark:     msg,
		})
		return "", errors.New(res.Msg)
	}
	// 是否需要记录翻译日志？要的
	res.Data.Uri = req.Uri // 因为返回的没有这个，因此这里要补上
	res.Data.Status = 1
	AddTranslateHtmlLog(&res.Data)

	return res.Data.Html, nil
}

func (w *Website) NewAuthReq(contentType string) *gorequest.SuperAgent {
	req := gorequest.New().
		SetDoNotClearSuperAgent(true).
		TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
		Timeout(300*time.Second).
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
	// if the file is old
	if strings.HasSuffix(self, ".old") {
		self = strings.TrimRight(self, ".old")
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

func Shutdown() {
	sites := GetWebsites()
	for _, w := range sites {
		// 关闭数据库
		if w.DB != nil {
			db, err := w.DB.DB()
			if err == nil {
				_ = db.Close()
			}
		}
		// 关闭日志文件
		if w.StatisticLog != nil {
			w.StatisticLog.Close()
		}
	}
}
