package provider

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"kandaoni.com/anqicms/config"
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
	"unicode/utf8"
)

type TransferWebsite struct {
	w        *Website
	Name     string `json:"name"`
	BaseUrl  string `json:"base_url"`
	Token    string `json:"token"`
	Provider string `json:"provider"`
	Status   int    `json:"status"` // 0 waiting, 1 doing,2 done
	ErrorMsg string `json:"error_msg"`
	Current  string `json:"current"`
	LastId   int64  `json:"last_id"`
	LastMod  string `json:"last_mod"`
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
	SeoTitle    string `json:"seo_title"`
	Keywords    string `json:"keywords"`
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
	Id           int64                  `json:"id"`
	Title        string                 `json:"title"`
	SeoTitle     string                 `json:"seo_title"`
	ModuleId     uint                   `json:"module_id"`
	CategoryId   uint                   `json:"category_id"`
	Keywords     string                 `json:"keywords"`
	Description  string                 `json:"description"`
	Content      string                 `json:"content"`
	Template     string                 `json:"template"`
	Images       []string               `json:"images"`
	Logo         string                 `json:"logo"`
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
	LastMod string        `json:"last_mod"`
	Data    []ArchiveData `json:"data"`
}

type TransferTags struct {
	TransferBase
	Data []TagData `json:"data"`
}

type TransferAnchors struct {
	TransferBase
	Data []AnchorData `json:"data"`
}

func (w *Website) GetTransferTask() *TransferWebsite {
	return w.transferWebsite
}

func (w *Website) CreateTransferTask(website *request.TransferWebsite) (*TransferWebsite, error) {
	w.transferWebsite = &TransferWebsite{
		w:        w,
		Name:     website.Name,
		BaseUrl:  strings.TrimRight(website.BaseUrl, "/"),
		Token:    website.Token,
		Provider: website.Provider,
		Status:   0,
	}
	// 尝试链接文件
	remoteUrl := w.transferWebsite.BaseUrl + "/" + w.transferWebsite.Provider + "2anqicms.php?a=config&from=anqicms"
	resp, err := library.Request(remoteUrl, &library.Options{Method: "POST", Type: "json", Data: w.transferWebsite})
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

	return w.transferWebsite, nil
}

func (w *Website) DeleteTransferTask() {
	w.transferWebsite = nil
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
func (t *TransferWebsite) TransferWebData(req *request.TransferTypes) {
	t.Status = 1
	var typeMap = map[string]bool{}
	for _, v := range req.Types {
		typeMap[v] = true
	}
	// 1，module
	if typeMap["module"] {
		log.Println("正在同步模型数据")
		err := t.transferModules(req.ModuleIds)
		if err != nil {
			return
		}
	}
	// 2 category
	if typeMap["category"] {
		log.Println("正在同步分类数据")
		err := t.transferCategories()
		if err != nil {
			return
		}
	}
	// 3 tag
	log.Println("正在同步标签数据")
	if typeMap["tag"] {
		err := t.transferTags()
		if err != nil {
			return
		}
	}
	// 4 keyword
	if typeMap["keyword"] {
		log.Println("正在同步锚文本数据")
		err := t.transferKeywords()
		if err != nil {
			return
		}
	}
	// 5 archive
	if typeMap["archive"] {
		log.Println("正在同步文档数据")
		err := t.transferArchives(req.ModuleIds)
		if err != nil {
			return
		}
	}
	// 6 singlepage
	if typeMap["singlepage"] {
		log.Println("正在同步单页数据")
		err := t.transferSinglePages()
		if err != nil {
			return
		}
	}
	// 7 static
	if typeMap["static"] {
		log.Println("正在同步静态资源数据")
		err := t.transferStatics()
		if err != nil {
			return
		}
	}
	t.Status = 2
	t.ErrorMsg = ""

	t.w.RemoveHtmlCache()
}

func (t *TransferWebsite) GetModules() ([]ModuleData, error) {
	resp, err := t.getWebData("module", 0, "")
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return nil, err
	}

	var result TransferModules
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return nil, errors.New(resp.Body)
	}
	if result.Code != 0 {
		t.ErrorMsg = result.Msg
		t.Status = 2 // done
		return nil, errors.New(result.Msg)
	}

	return result.Data, nil
}

func (t *TransferWebsite) transferModules(moduleIds []uint) error {
	t.Current = "module"
	resp, err := t.getWebData("module", 0, "")
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}

	var result TransferModules
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return errors.New(resp.Body)
	}
	if result.Code != 0 {
		t.ErrorMsg = result.Msg
		t.Status = 2 // done
		return errors.New(result.Msg)
	}

	for i := range result.Data {
		// 如果选择了模块，则只导入对应模块
		exist := false
		for _, tmpModId := range moduleIds {
			if tmpModId == result.Data[i].Id {
				exist = true
				break
			}
		}
		if !exist {
			continue
		}
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
		t.w.DB.Save(&module)
		module.Database = t.w.Mysql.Database
		tplPath := fmt.Sprintf("%s/%s", t.w.GetTemplateDir(), module.TableName)
		module.Migrate(t.w.DB, tplPath, true)
	}
	t.w.DeleteCacheModules()

	return nil
}

func (t *TransferWebsite) transferCategories() error {
	t.Current = "category"
	resp, err := t.getWebData("category", 0, "")
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}
	var result TransferCategories
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return errors.New(resp.Body)
	}
	if result.Code != 0 {
		t.ErrorMsg = result.Msg
		t.Status = 2 // done
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
		if utf8.RuneCountInString(category.Title) > 190 {
			category.Title = string([]rune(category.Title)[:190])
		}
		if utf8.RuneCountInString(category.Keywords) > 250 {
			category.Keywords = string([]rune(category.Keywords)[:250])
		}
		if utf8.RuneCountInString(category.SeoTitle) > 250 {
			category.SeoTitle = string([]rune(category.SeoTitle)[:250])
		}
		if category.Description == "" {
			category.Description = library.ParseDescription(strings.ReplaceAll(library.StripTags(category.Content), "\n", " "))
		} else if utf8.RuneCountInString(category.Description) > 1000 {
			// 字段最大支持1000
			category.Description = string([]rune(category.Description)[:1000])
		}
		category.UrlToken = strings.TrimSuffix(category.UrlToken, ".html")
		category.Id = result.Data[i].Id
		if category.UrlToken == "" {
			category.UrlToken = library.GetPinyin(category.Title, t.w.Content.UrlTokenType == config.UrlTokenTypeSort)
		}
		category.UrlToken = t.w.VerifyCategoryUrlToken(category.UrlToken, category.Id)

		t.w.DB.Save(&category)
	}
	t.w.DeleteCacheCategories()
	return nil
}

func (t *TransferWebsite) transferTags() error {
	t.Current = "tag"
	resp, err := t.getWebData("tag", 0, "")
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}
	var result TransferTags
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return errors.New(resp.Body)
	}
	if result.Code != 0 {
		t.ErrorMsg = result.Msg
		t.Status = 2 // done
		return errors.New(result.Msg)
	}
	for i := range result.Data {
		tag := model.Tag{
			Title:       result.Data[i].Title,
			Keywords:    result.Data[i].Keywords,
			SeoTitle:    result.Data[i].SeoTitle,
			UrlToken:    result.Data[i].UrlToken,
			Description: result.Data[i].Description,
			Status:      1,
		}
		if utf8.RuneCountInString(tag.Title) > 190 {
			tag.Title = string([]rune(tag.Title)[:190])
		}
		if utf8.RuneCountInString(tag.Keywords) > 250 {
			tag.Keywords = string([]rune(tag.Keywords)[:250])
		}
		if utf8.RuneCountInString(tag.SeoTitle) > 250 {
			tag.SeoTitle = string([]rune(tag.SeoTitle)[:250])
		}
		if utf8.RuneCountInString(tag.Description) > 1000 {
			// 字段最大支持1000
			tag.Description = string([]rune(tag.Description)[:1000])
		}
		tag.Id = result.Data[i].Id
		tag.UrlToken = t.w.VerifyTagUrlToken(tag.UrlToken, tag.Title, tag.Id)
		letter := "A"
		if tag.UrlToken != "-" {
			letter = string(tag.UrlToken[0])
		}
		tag.FirstLetter = letter
		t.w.DB.Save(&tag)
	}
	return nil
}

func (t *TransferWebsite) transferKeywords() error {
	t.Current = "keyword"
	resp, err := t.getWebData("keyword", 0, "")
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}
	var result TransferAnchors
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return errors.New(resp.Body)
	}
	if result.Code != 0 {
		t.ErrorMsg = result.Msg
		t.Status = 2 // done
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
		t.w.DB.Save(&anchor)
	}
	return nil
}

func (t *TransferWebsite) transferArchives(moduleIds []uint) error {
	t.Current = "archive"
	t.LastId = 0
	t.LastMod = ""
	for {
		resp, err := t.getWebData("archive", t.LastId, t.LastMod)
		if err != nil {
			t.ErrorMsg = err.Error()
			t.Status = 2 // done
			return err
		}
		var result TransferArchives
		err = json.Unmarshal([]byte(resp.Body), &result)
		if err != nil {
			t.ErrorMsg = err.Error()
			t.Status = 2 // done
			return errors.New(resp.Body)
		}
		if result.Code != 0 {
			t.ErrorMsg = result.Msg
			t.Status = 2 // done
			return errors.New(result.Msg)
		}
		if len(result.Data) == 0 {
			break
		}
		t.LastMod = result.LastMod
		t.LastId = int64(result.Data[len(result.Data)-1].Id)
		for i := range result.Data {
			// 如果选择了模块，则只导入对应模块
			if len(moduleIds) > 0 {
				exist := false
				for _, tmpModId := range moduleIds {
					if tmpModId == result.Data[i].ModuleId {
						exist = true
						break
					}
				}
				if !exist {
					continue
				}
			}
			// 迁移过来需要保持ID不变
			archive := model.ArchiveDraft{
				Archive: model.Archive{
					Title:       result.Data[i].Title,
					SeoTitle:    result.Data[i].SeoTitle,
					UrlToken:    result.Data[i].UrlToken,
					Keywords:    result.Data[i].Keywords,
					Description: result.Data[i].Description,
					ModuleId:    result.Data[i].ModuleId,
					CategoryId:  result.Data[i].CategoryId,
					Views:       result.Data[i].Views,
					Images:      result.Data[i].Images,
				},
				Status: result.Data[i].Status,
			}
			if utf8.RuneCountInString(archive.Title) > 190 {
				archive.Title = string([]rune(archive.Title)[:190])
			}
			if utf8.RuneCountInString(archive.Keywords) > 250 {
				archive.Keywords = string([]rune(archive.Keywords)[:250])
			}
			if utf8.RuneCountInString(archive.SeoTitle) > 250 {
				archive.SeoTitle = string([]rune(archive.SeoTitle)[:250])
			}
			if archive.Description == "" {
				archive.Description = library.ParseDescription(strings.ReplaceAll(library.StripTags(result.Data[i].Content), "\n", " "))
			} else if utf8.RuneCountInString(archive.Description) > 1000 {
				// 字段最大支持1000
				archive.Description = string([]rune(archive.Description)[:1000])
			}
			if result.Data[i].Logo != "" {
				archive.Images = append(archive.Images, result.Data[i].Logo)
			}
			for x := range archive.Images {
				archive.Images[x] = strings.Replace(archive.Images[x], t.BaseUrl, "", 1)
			}
			archive.CreatedTime = result.Data[i].CreatedTime
			archive.UpdatedTime = result.Data[i].UpdatedTime
			archive.UrlToken = strings.TrimSuffix(archive.UrlToken, ".html")
			archive.Id = result.Data[i].Id
			archive.UrlToken = t.w.VerifyArchiveUrlToken(archive.UrlToken, archive.Title, archive.Id)
			// 先保存为草稿
			t.w.DB.Save(&archive)
			// 如果status == 1，则保存为正式表
			if archive.Status == config.ContentStatusOK {
				realArchive := &archive.Archive
				// 保存到正式表
				t.w.DB.Save(&realArchive)
				// 并删除草稿
				t.w.DB.Delete(&archive)
			}
			// 保存内容表
			archiveData := model.ArchiveData{
				Content: ParseContent(result.Data[i].Content),
			}
			archiveData.Id = archive.Id
			t.w.DB.Save(&archiveData)
			module := t.w.GetModuleFromCache(archive.ModuleId)
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
					t.w.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Pluck("id", &existsId)
					if existsId > 0 {
						// 已存在
						t.w.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Updates(extraFields)
					} else {
						// 新建
						extraFields["id"] = archive.Id
						t.w.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Create(extraFields)
					}
				}
			}
			// categories
			if result.Data[i].CategoryId > 0 {
				_ = t.w.SaveArchiveCategories(archive.Id, []uint{result.Data[i].CategoryId})
			}
			// tags
			_ = t.w.SaveTagData(archive.Id, result.Data[i].Tags)
			// flags
			if len(result.Data[i].Flag) > 0 {
				_ = t.w.SaveArchiveFlags(archive.Id, strings.Split(result.Data[i].Flag, ","))
			}
		}
	}

	return nil
}

func (t *TransferWebsite) transferSinglePages() error {
	t.Current = "singlepage"
	resp, err := t.getWebData("singlepage", 0, "")
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}
	var result TransferCategories
	err = json.Unmarshal([]byte(resp.Body), &result)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return errors.New(resp.Body)
	}
	if result.Code != 0 {
		t.ErrorMsg = result.Msg
		t.Status = 2 // done
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
		if utf8.RuneCountInString(category.Title) > 190 {
			category.Title = string([]rune(category.Title)[:190])
		}
		if utf8.RuneCountInString(category.Keywords) > 250 {
			category.Keywords = string([]rune(category.Keywords)[:250])
		}
		if utf8.RuneCountInString(category.SeoTitle) > 250 {
			category.SeoTitle = string([]rune(category.SeoTitle)[:250])
		}
		if category.Description == "" {
			category.Description = library.ParseDescription(strings.ReplaceAll(library.StripTags(category.Content), "\n", " "))
		} else if utf8.RuneCountInString(category.Description) > 1000 {
			// 字段最大支持1000
			category.Description = string([]rune(category.Description)[:1000])
		}
		if result.Data[i].Logo != "" {
			category.Logo = strings.Replace(result.Data[i].Logo, t.BaseUrl, "", 1)
		}
		category.UrlToken = strings.TrimSuffix(category.UrlToken, ".html")
		// 如果已存在，如果类型不一样，则新增id，如果类型一样，则覆盖
		exists, err := t.w.GetCategoryById(result.Data[i].Id)
		if err == nil {
			if exists.Type != config.CategoryTypePage {
				exists2, err := t.w.GetCategoryByTitle(category.Title)
				if err == nil {
					// 如果名称相同，但类型不同，则跳过当前数据
					if exists2.Type != config.CategoryTypePage {
						continue
					} else {
						category.Id = exists2.Id
					}
				} else {
					// 不用旧ID，按新内容写入
				}
			} else {
				category.Id = result.Data[i].Id
			}
		} else {
			category.Id = result.Data[i].Id
		}
		if category.UrlToken == "" {
			category.UrlToken = library.GetPinyin(category.Title, t.w.Content.UrlTokenType == config.UrlTokenTypeSort)
		}
		category.UrlToken = t.w.VerifyCategoryUrlToken(category.UrlToken, category.Id)
		t.w.DB.Save(&category)
	}
	t.w.DeleteCacheCategories()
	return nil
}

func (t *TransferWebsite) transferStatics() error {
	t.Current = "static"
	t.LastId = 0
	tmpZipPath := t.w.CachePath + "transfer.zip"
	tmpFile, err := os.Create(tmpZipPath)
	if err != nil {
		t.ErrorMsg = err.Error()
		t.Status = 2 // done
		return err
	}
	defer tmpFile.Close()
	for {
		resp, err := t.getWebData("static", t.LastId, "")
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
		t.ErrorMsg = t.w.Tr("ErrorInDecompressingStaticFiles")
		t.Status = 2
		return errors.New(t.ErrorMsg)
	}
	defer func() {
		zipReader.Close()
		// 删除压缩包
		os.Remove(tmpZipPath)
	}()
	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			continue
		}
		realName := filepath.Join(t.w.PublicPath, f.Name)
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
			t.w.insertAttachment(realName, 1)
		} else if strings.HasSuffix(f.Name, ".mp4") ||
			strings.HasSuffix(f.Name, ".webm") {
			t.w.insertAttachment(realName, 0)
		}
	}

	t.Status = 2

	return nil
}

func (w *Website) insertAttachment(realName string, isImage int) {
	if !strings.HasPrefix(realName, w.PublicPath) {
		return
	}
	fileLocation := strings.TrimPrefix(realName, w.PublicPath)
	var exists model.Attachment
	// location 存在跳过
	err := w.DB.Where("`file_location` = ?", fileLocation).Take(&exists).Error
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
	md5hash := md5.New()
	_, err = io.Copy(md5hash, file)
	file.Seek(0, 0)
	if err != nil {
		return
	}
	md5Str := hex.EncodeToString(md5hash.Sum(nil))

	attachment := &model.Attachment{
		FileName:     filepath.Base(fileLocation),
		FileLocation: fileLocation,
		FileSize:     info.Size(),
		FileMd5:      md5Str,
		CategoryId:   0,
		IsImage:      isImage,
		Status:       1,
	}
	err = attachment.Save(w.DB)
	if err != nil {
		return
	}
	attachment.GetThumb(w.PluginStorage.StorageUrl)
	fileHeader := &multipart.FileHeader{
		Filename: filepath.Base(realName),
		Header:   nil,
		Size:     info.Size(),
	}
	// 再走一遍上传流程
	_, err = w.AttachmentUpload(file, fileHeader, 0, attachment.Id, 0)
	if err != nil {
		log.Println(err)
		return
	}
}

func (t *TransferWebsite) getWebData(transferType string, lastId int64, lastMod string) (*library.RequestData, error) {
	remoteUrl := t.BaseUrl + "/" + t.Provider + "2anqicms.php?"
	query := make(url.Values)
	query.Set("a", "syncData")
	_t := fmt.Sprintf("%d", time.Now().Unix())
	query.Set("from", "anqicms")
	query.Set("_t", _t)
	query.Set("token", library.Md5(t.Token+_t))
	query.Set("type", transferType)
	query.Set("last_id", fmt.Sprintf("%d", lastId))
	query.Set("last_mod", lastMod)
	resp, err := library.GetURLData(remoteUrl+query.Encode(), "", 100)
	if err != nil {
		return nil, err
	}

	return resp, err
}

// ParseContent 转换content内容，使它符合编辑器使用。
func ParseContent(content string) string {
	// 已更换编辑器，不需要再做处理
	return content
}
