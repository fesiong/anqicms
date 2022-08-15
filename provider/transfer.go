package provider

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"log"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type TransferWebsite struct {
	Name     string `json:"name"`
	BaseUrl  string `json:"base_url"`
	Token    string `json:"token"`
	Provider string `json:"provider"`
	Status   int    `json:"status"` // 0 waiting, 1 doing,2 done
	ErrorMsg string `json:"error_msg"`
	Current  string `json:"current"`
	LastId   int64  `json:"last_id"`
}

type TransferBase struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type TransferResult struct {
	TransferBase
	Data interface{} `json:"data"`
}

type ModuleData struct {
	Id        uint                 `json:"id"`
	TableName string               `json:"table_name"`
	UrlToken  string               `json:"url_token"`
	Title     string               `json:"title"`
	Fields    []config.CustomField `json:"fields"`
	IsSystem  int                  `json:"is_system"`
	TitleName string               `json:"title_name"`
	Status    uint                 `json:"status"`
}

type CategoryData struct {
	Id             uint     `json:"id"`
	Title          string   `json:"title"`
	SeoTitle       string   `json:"seo_title"`
	Keywords       string   `json:"keywords"`
	Description    string   `json:"description"`
	Content        string   `json:"content"`
	ModuleId       uint     `json:"module_id"`
	ParentId       uint     `json:"parent_id"`
	Sort           uint     `json:"sort"`
	Status         uint     `json:"status"`
	Type           uint     `json:"type"`
	Template       string   `json:"template"`
	DetailTemplate string   `json:"detail_template"`
	UrlToken       string   `json:"url_token"`
	Images         []string `json:"images"`
	Logo           string   `json:"logo"`
	IsInherit      uint     `json:"is_inherit"`
	CreatedTime    int64    `json:"created_time"`
}

type TagData struct {
	Id          uint   `json:"id"`
	Title       string `json:"title"`
	UrlToken    string `json:"url_token"`
	Description string `json:"description"`
	CreatedTime int64  `json:"created_time"`
}

type AnchorData struct {
	Id     uint   `json:"id"`
	Title  string `json:"title"`
	Link   string `json:"link"`
	Weight int    `json:"weight"`
}

type ArchiveData struct {
	Id           uint                   `json:"id"`
	Title        string                 `json:"title"`
	SeoTitle     string                 `json:"seo_title"`
	ModuleId     uint                   `json:"module_id"`
	CategoryId   uint                   `json:"category_id"`
	Keywords     string                 `json:"keywords"`
	Description  string                 `json:"description"`
	Content      string                 `json:"content"`
	Template     string                 `json:"template"`
	Images       []string               `json:"images"`
	Extra        map[string]interface{} `json:"extra"`
	Status       uint                   `json:"status"`
	CreatedTime  int64                  `json:"created_time"`
	UpdatedTime  int64                  `json:"updated_time"`
	UrlToken     string                 `json:"url_token"`
	Views        uint                   `json:"views"`
	Tags         []string               `json:"tags"`
	CanonicalUrl string                 `json:"canonical_url"`
	FixedLink    string                 `json:"fixed_link"`
	Flag         string                 `json:"flag"`
	Draft        bool                   `json:"draft"` // 是否是存草稿
}

type TransferModules struct {
	TransferBase
	Data []ModuleData `json:"data"`
}

type TransferCategories struct {
	TransferBase
	Data []CategoryData `json:"data"`
}

type TransferArchives struct {
	TransferBase
	Data []ArchiveData `json:"data"`
}

type TransferTags struct {
	TransferBase
	Data []TagData `json:"data"`
}

type TransferAnchors struct {
	TransferBase
	Data []AnchorData `json:"data"`
}

var transferWebsite *TransferWebsite

func GetTransferTask() *TransferWebsite {

	return transferWebsite
}

func CreateTransferTask(website *request.TransferWebsite) (*TransferWebsite, error) {
	transferWebsite = &TransferWebsite{
		Name:     website.Name,
		BaseUrl:  strings.TrimRight(website.BaseUrl, "/"),
		Token:    website.Token,
		Provider: website.Provider,
		Status:   0,
	}

	// 尝试链接文件
	remoteUrl := transferWebsite.BaseUrl + "/" + transferWebsite.Provider + "2anqicms.php?a=config&from=anqicms"
	resp, err := library.Request(remoteUrl, &library.Options{Method: "POST", Type: "json", Data: transferWebsite})
	if err != nil {
		return nil, err
	}
	var result TransferResult
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		return nil, errors.New(resp.Body)
	}
	if result.Code != 0 {
		return nil, errors.New(result.Msg)
	}

	return transferWebsite, nil
}

func DeleteTransferTask() {
	transferWebsite = nil
}

// TransferWebData
// * 需要执行的操作type：
// -. 同步模型 module
// -. 同步分类 category
// -. 同步标签 tag
// -. 同步锚文本 keyword
// -. 同步文档 archive
// -. 同步单页 singlepage
// -. 同步静态资源 static
func (t *TransferWebsite) TransferWebData() {
	t.Status = 1
	// 1，module
	log.Println("正在同步模型数据")
	err := t.transferModules()
	if err != nil {
		return
	}
	// 2 category
	log.Println("正在同步分类数据")
	err = t.transferCategories()
	if err != nil {
		return
	}
	// 3 tag
	log.Println("正在同步标签数据")
	err = t.transferTags()
	if err != nil {
		return
	}
	// 4 keyword
	log.Println("正在同步锚文本数据")
	err = t.transferKeywords()
	if err != nil {
		return
	}
	// 5 archive
	log.Println("正在同步文档数据")
	err = t.transferArchives()
	if err != nil {
		return
	}
	// 6 singlepage
	log.Println("正在同步单页数据")
	err = t.transferSinglePages()
	if err != nil {
		return
	}
	// 7 static
	log.Println("正在同步静态资源数据")
	err = t.transferStatics()
	if err != nil {
		return
	}
	t.Status = 2
	t.ErrorMsg = ""

	DeleteCacheIndex()
}

func (t *TransferWebsite) transferModules() error {
	t.Current = "module"
	resp, err := t.getWebData("module", 0)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}

	var result TransferModules
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		return errors.New(resp.Body)
	}
	if result.Code != 0 {
		return errors.New(result.Msg)
	}

	for i := range result.Data {
		module := model.Module{
			TableName: result.Data[i].TableName,
			UrlToken:  result.Data[i].UrlToken,
			Title:     result.Data[i].Title,
			Fields:    result.Data[i].Fields,
			IsSystem:  result.Data[i].IsSystem,
			TitleName: result.Data[i].TitleName,
			Status:    result.Data[i].Status,
		}
		module.Id = result.Data[i].Id
		if module.UrlToken == "" {
			module.UrlToken = module.TableName
		}
		dao.DB.Save(&module)

		module.Migrate(dao.DB, true)
	}
	DeleteCacheModules()

	return nil
}

func (t *TransferWebsite) transferCategories() error {
	t.Current = "category"
	resp, err := t.getWebData("category", 0)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}
	var result TransferCategories
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		return errors.New(resp.Body)
	}
	if result.Code != 0 {
		return errors.New(result.Msg)
	}
	for i := range result.Data {
		category := model.Category{
			Title:       result.Data[i].Title,
			SeoTitle:    result.Data[i].SeoTitle,
			Keywords:    result.Data[i].Keywords,
			UrlToken:    result.Data[i].UrlToken,
			Description: result.Data[i].Description,
			Content:     ParseContent(result.Data[i].Content),
			ModuleId:    result.Data[i].ModuleId,
			ParentId:    result.Data[i].ParentId,
			Type:        config.CategoryTypeArchive,
			Sort:        result.Data[i].Sort,
			Status:      result.Data[i].Status,
		}
		category.UrlToken = strings.TrimSuffix(category.UrlToken, ".html")
		category.Id = result.Data[i].Id
		if category.UrlToken == "" {
			category.UrlToken = library.GetPinyin(category.Title)
		}
		category.UrlToken = VerifyCategoryUrlToken(category.UrlToken, category.Id)

		dao.DB.Save(&category)
	}
	DeleteCacheCategories()
	return nil
}

func (t *TransferWebsite) transferTags() error {
	t.Current = "tag"
	resp, err := t.getWebData("tag", 0)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}
	var result TransferTags
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		return errors.New(resp.Body)
	}
	if result.Code != 0 {
		return errors.New(result.Msg)
	}
	for i := range result.Data {
		tag := model.Tag{
			Title:       result.Data[i].Title,
			UrlToken:    result.Data[i].UrlToken,
			Description: result.Data[i].Description,
			Status:      1,
		}
		tag.Id = result.Data[i].Id
		if tag.UrlToken == "" {
			tag.UrlToken = library.GetPinyin(tag.Title)
		}
		tag.UrlToken = VerifyTagUrlToken(tag.UrlToken, tag.Id)
		letter := "A"
		if tag.UrlToken != "-" {
			letter = string(tag.UrlToken[0])
		}
		tag.FirstLetter = letter
		dao.DB.Save(&tag)
	}
	return nil
}

func (t *TransferWebsite) transferKeywords() error {
	t.Current = "keyword"
	resp, err := t.getWebData("keyword", 0)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}
	var result TransferAnchors
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		return errors.New(resp.Body)
	}
	if result.Code != 0 {
		return errors.New(result.Msg)
	}
	for i := range result.Data {
		anchor := model.Anchor{
			Title:     result.Data[i].Title,
			ArchiveId: 0,
			Link:      result.Data[i].Link,
			Weight:    result.Data[i].Weight,
			Status:    1,
		}
		anchor.Id = result.Data[i].Id
		dao.DB.Save(&anchor)
	}
	return nil
}

func (t *TransferWebsite) transferArchives() error {
	t.Current = "archive"
	t.LastId = 0
	for {
		resp, err := t.getWebData("archive", t.LastId)
		if err != nil {
			t.ErrorMsg = err.Error()
			t.Status = 2 // done
			return err
		}
		var result TransferArchives
		err = json.Unmarshal([]byte(resp.Body), &result)
		if err != nil {
			return errors.New(resp.Body)
		}
		if result.Code != 0 {
			return errors.New(result.Msg)
		}
		if len(result.Data) == 0 {
			break
		}
		t.LastId = int64(result.Data[len(result.Data)-1].Id)
		for i := range result.Data {
			archive := model.Archive{
				Title:       result.Data[i].Title,
				SeoTitle:    result.Data[i].SeoTitle,
				UrlToken:    result.Data[i].UrlToken,
				Keywords:    result.Data[i].Keywords,
				Description: result.Data[i].Description,
				ModuleId:    result.Data[i].ModuleId,
				CategoryId:  result.Data[i].CategoryId,
				Views:       result.Data[i].Views,
				Images:      result.Data[i].Images,
				Status:      result.Data[i].Status,
				Flag:        result.Data[i].Flag,
			}
			archive.CreatedTime = result.Data[i].CreatedTime
			archive.UpdatedTime = result.Data[i].UpdatedTime
			archive.UrlToken = strings.TrimSuffix(archive.UrlToken, ".html")
			archive.Id = result.Data[i].Id
			if archive.UrlToken == "" {
				archive.UrlToken = library.GetPinyin(archive.Title)
			}
			archive.UrlToken = VerifyArchiveUrlToken(archive.UrlToken, archive.Id)
			// 保存主表
			dao.DB.Save(&archive)
			// 保存内容表
			archiveData := model.ArchiveData{
				Content: ParseContent(result.Data[i].Content),
			}
			archiveData.Id = archive.Id
			dao.DB.Save(&archiveData)
			module := GetModuleFromCache(archive.ModuleId)
			if module != nil {
				//extra
				extraFields := map[string]interface{}{}
				if len(module.Fields) > 0 {
					for _, v := range module.Fields {
						if result.Data[i].Extra[v.FieldName] != nil {
							extraValue := result.Data[i].Extra[v.FieldName]
							if v.Type == config.CustomFieldTypeNumber {
								//只有这个类型的数据是数字，转成数字
								extraFields[v.FieldName], _ = strconv.Atoi(fmt.Sprintf("%v", extraValue))
							} else {
								extraFields[v.FieldName] = extraValue
							}
						} else {
							if v.Type == config.CustomFieldTypeNumber {
								//只有这个类型的数据是数字，转成数字
								extraFields[v.FieldName] = 0
							} else {
								extraFields[v.FieldName] = ""
							}
						}
					}
				}
				if len(extraFields) > 0 {
					// 先检查是否存在
					var existsId uint
					dao.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Pluck("id", &existsId)
					if existsId > 0 {
						// 已存在
						dao.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Updates(extraFields)
					} else {
						// 新建
						extraFields["id"] = archive.Id
						dao.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Create(extraFields)
					}
				}
			}
			// tags
			_ = SaveTagData(archive.Id, result.Data[i].Tags)
		}
	}

	return nil
}

func (t *TransferWebsite) transferSinglePages() error {
	t.Current = "singlepage"
	resp, err := t.getWebData("singlepage", 0)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}
	var result TransferCategories
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		return errors.New(resp.Body)
	}
	if result.Code != 0 {
		return errors.New(result.Msg)
	}
	for i := range result.Data {
		category := model.Category{
			Title:       result.Data[i].Title,
			SeoTitle:    result.Data[i].SeoTitle,
			Keywords:    result.Data[i].Keywords,
			UrlToken:    result.Data[i].UrlToken,
			Description: result.Data[i].Description,
			Content:     ParseContent(result.Data[i].Content),
			ModuleId:    result.Data[i].ModuleId,
			ParentId:    result.Data[i].ParentId,
			Type:        config.CategoryTypePage,
			Sort:        result.Data[i].Sort,
			Status:      result.Data[i].Status,
		}
		category.UrlToken = strings.TrimSuffix(category.UrlToken, ".html")
		// 如果已存在
		exists, err := GetCategoryById(result.Data[i].Id)
		if err == nil {
			if exists.Id == result.Data[i].Id {
				category.Id = result.Data[i].Id
			} else {
				exists, err = GetCategoryByTitle(category.Title)
				if err == nil && exists.Type == config.CategoryTypePage {
					category.Id = result.Data[i].Id
				}
			}
		} else {
			category.Id = result.Data[i].Id
		}
		if category.UrlToken == "" {
			category.UrlToken = library.GetPinyin(category.Title)
		}
		category.UrlToken = VerifyCategoryUrlToken(category.UrlToken, category.Id)

		dao.DB.Save(&category)
	}
	DeleteCacheCategories()
	return nil
}

func (t *TransferWebsite) transferStatics() error {
	t.Current = "static"
	t.LastId = 0
	tmpZipPath := config.ExecPath + "cache/transfer.zip"
	tmpFile, err := os.Create(tmpZipPath)
	if err != nil {
		return err
	}
	defer tmpFile.Close()
	for {
		resp, err := t.getWebData("static", t.LastId)
		if err != nil {
			t.ErrorMsg = err.Error()
			t.Status = 2 // done
			return err
		}
		var result TransferBase
		err = json.Unmarshal([]byte(resp.Body), &result)
		if err == nil {
			t.ErrorMsg = err.Error()
			t.Status = 2 // done
			return errors.New(result.Msg)
		}
		if resp.Body == "@end" {
			break
		}
		t.LastId += int64(len(resp.Body))
		tmpFile.WriteString(resp.Body)
	}
	// 解压
	zipReader, err := zip.OpenReader(tmpZipPath)
	if err != nil {
		t.ErrorMsg = "解压静态文件出错"
		t.Status = 2
		return errors.New(t.ErrorMsg)
	}
	defer func() {
		zipReader.Close()
		// 删除压缩包
		os.Remove(tmpZipPath)
	}()
	basePath := config.ExecPath + "public/"
	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			continue
		}
		realName := filepath.Join(basePath, f.Name)
		_ = os.MkdirAll(filepath.Dir(realName), os.ModePerm)
		newFile, err := os.Create(realName)
		if err != nil {
			reader.Close()
			continue
		}
		_, err = io.Copy(newFile, reader)
		if err != nil {
			reader.Close()
			newFile.Close()
			continue
		}

		reader.Close()
		_ = newFile.Close()

		// 如果是图片，入库到attachment
		if strings.HasSuffix(f.Name, ".jpg") ||
			strings.HasSuffix(f.Name, ".jpeg") ||
			strings.HasSuffix(f.Name, ".png") ||
			strings.HasSuffix(f.Name, ".gif") ||
			strings.HasSuffix(f.Name, ".webp") {
			insertAttachment(realName)
		}
	}

	t.Status = 2

	return nil
}

func insertAttachment(realName string) {
	basePath := config.ExecPath + "public/"
	if !strings.HasPrefix(realName, basePath) {
		return
	}
	fileLocation := strings.TrimPrefix(realName, basePath)
	var exists model.Attachment
	// location 存在跳过
	err := dao.DB.Where("`file_location` = ?", fileLocation).Take(&exists).Error
	if err == nil {
		// 已存在，跳过
		return
	}

	file, err := os.OpenFile(realName, os.O_RDWR, 0666)
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		log.Println(err)
		return
	}

	fileHeader := &multipart.FileHeader{
		Filename: filepath.Base(realName),
		Header:   nil,
		Size:     info.Size(),
	}
	// 再走一遍上传流程
	_, err = AttachmentUpload(file, fileHeader, 0, 0)
	if err != nil {
		log.Println(err)
		return
	}
}

func (t *TransferWebsite) getWebData(transferType string, lastId int64) (*library.RequestData, error) {
	remoteUrl := t.BaseUrl + "/" + t.Provider + "2anqicms.php?"
	query := make(url.Values)
	query.Set("a", "syncData")
	_t := fmt.Sprintf("%d", time.Now().Unix())
	query.Set("from", "anqicms")
	query.Set("_t", _t)
	query.Set("token", library.Md5(t.Token+_t))
	query.Set("type", transferType)
	query.Set("last_id", fmt.Sprintf("%d", lastId))

	resp, err := library.GetURLData(remoteUrl+query.Encode(), "")
	if err != nil {
		return nil, err
	}

	return resp, err
}

// ParseContent 转换content内容，使它符合编辑器使用。
func ParseContent(content string) string {
	// 已更换编辑器，不需要再做处理
	return content
	htmlR := strings.NewReader(content)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err != nil {
		return content
	}
	body := doc.Find("body")

	body.Children().Each(func(i int, item *goquery.Selection) {
		//只保留 顶层只运行 pre，blockquote,ul,ol,table,p,div
		if item.Is("blockquote") ||
			item.Is("pre") ||
			item.Is("table") ||
			item.Is("h1") ||
			item.Is("h2") ||
			item.Is("h3") ||
			item.Is("h4") ||
			item.Is("h5") ||
			item.Is("h6") ||
			item.Is("p") ||
			item.Is("div") {
			return
		}
		if item.Is("figure") {
			// 重新wrap
			tmp, _ := item.Html()
			item.ReplaceWithHtml("<p>"+tmp+"</p>")
			return
		}
		if item.Is("code") {
			// 重新wrap
			item.WrapHtml("<pre></pre>")
			return
		}
		if item.Is("img") {
			src, _ := item.Attr("src")
			dataSrc, exists2 := item.Attr("data-src")
			if exists2 {
				src = dataSrc
			}
			dataSrc, exists2 = item.Attr("data-original")
			if exists2 {
				src = dataSrc
			}
			alt, _ := item.Attr("alt")
			if src == "" {
				item.Remove()
			} else {
				item.ReplaceWithHtml("<p><img src=\""+src+"\" alt=\""+alt+"\"/></p>")
			}
			return
		}
		//其他情况
		if item.Is("ul") || item.Is("ol") {
			if item.Find("li").Length() == 0 {
				tmp, _ := item.Html()
				item.WrapInnerHtml("<li>"+tmp+"</li>")
			}
		} else {
			item.WrapHtml("<div></div>")
		}
	})

	result, err := body.Html()
	if err != nil {
		log.Println("保存错误", err)
		return content
	}
	return result
}
