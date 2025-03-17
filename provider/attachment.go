package provider

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/h2non/filetype"
	"github.com/parnurzeal/gorequest"
	"golang.org/x/image/webp"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"log"
	"math"
	"math/rand"
	"mime/multipart"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// imageMagickPath 检测Imagemagick是否可用
var imageMagickPath string
var webpPath string
var pngquantPath string

func (w *Website) AttachmentUpload(file multipart.File, info *multipart.FileHeader, categoryId uint, attachId, userId uint) (*model.Attachment, error) {
	db := w.DB

	file.Seek(0, 0)
	kind, _ := filetype.MatchReader(file)
	file.Seek(0, 0)
	fileExt := "." + kind.Extension
	if kind == filetype.Unknown {
		fileExt = strings.ToLower(filepath.Ext(info.Filename))
	}
	if fileExt == ".php" || fileExt == ".jsp" {
		return nil, errors.New(w.Tr("NotAllowedToUploadPhpFiles"))
	}
	if fileExt == ".jpeg" {
		fileExt = ".jpg"
	}
	if fileExt == ".ico" || fileExt == ".bmp" {
		fileExt = ".png"
	}
	if fileExt == "." {
		fileExt = ""
	}
	isImage := 0
	if fileExt == ".jpg" || fileExt == ".png" || fileExt == ".gif" || fileExt == ".webp" {
		isImage = 1
	}

	var attachment *model.Attachment
	var err error
	if attachId > 0 {
		attachment, err = w.GetAttachmentById(attachId)
		if err != nil {
			return nil, errors.New(w.Tr("TheImageResourceToBeReplacedDoesNotExist"))
		}
		isImage = attachment.IsImage
	}

	fileSize := info.Size
	fileName := strings.TrimSuffix(info.Filename, path.Ext(info.Filename))
	// 获取md5
	md5hash := md5.New()
	_, _ = file.Seek(0, 0)
	_, err = io.Copy(md5hash, file)
	if err != nil {
		return nil, err
	}
	_, _ = file.Seek(0, 0)
	md5Str := hex.EncodeToString(md5hash.Sum(nil))
	exists, _ := w.GetAttachmentByMd5(md5Str)
	if attachment != nil {
		if exists != nil && attachment.Id != exists.Id {
			return nil, errors.New(w.Tr("ReplacementFailedCurrentlyUploadedResourcesAlreadyExist"))
		}
		fileName = attachment.FileName
		fileExt = filepath.Ext(attachment.FileLocation)
		attachId = attachment.Id
	} else if exists != nil {
		attachment = exists
		// 已存在
		if attachment.DeletedAt.Valid {
			//更新
			err = db.Unscoped().Model(attachment).Update("deleted_at", nil).Error
			if err != nil {
				return nil, err
			}
		}
		// 如果更换了分类
		if categoryId > 0 && attachment.CategoryId != categoryId {
			db.Model(attachment).UpdateColumn("category_id", categoryId)
		}
		fileName = attachment.FileName
		fileExt = filepath.Ext(attachment.FileLocation)
		attachId = attachment.Id
	}
	// 生成文件名
	tmpName := md5Str[8:24] + fileExt
	filePath := time.Now().Format("uploads/200601/02/")
	if attachId > 0 {
		filePath = filepath.Dir(attachment.FileLocation) + "/"
		tmpName = filepath.Base(attachment.FileLocation)
	}
	filePath = strings.ReplaceAll(filePath, "\\", "/")

	// 不是图片的时候的处理方法
	if isImage != 1 {
		bts, _ := io.ReadAll(file)
		fileSize = int64(len(bts))
		_, err = w.Storage.UploadFile(filePath+tmpName, bts)
		if err != nil {
			return nil, err
		}
		//文件上传完成
		attachment = &model.Attachment{
			UserId:       userId,
			FileName:     fileName,
			FileLocation: filePath + tmpName,
			FileSize:     fileSize,
			FileMd5:      md5Str,
			CategoryId:   categoryId,
			IsImage:      0,
			Status:       1,
		}
		attachment.Id = attachId
		_ = attachment.Save(w.DB)
		attachment.GetThumb(w.PluginStorage.StorageUrl)

		return attachment, nil
	}

	// 是图片的时候的处理方法
	//获取宽高
	img, imgType, err := image.Decode(file)
	file.Seek(0, 0)
	if err != nil {
		if strings.HasSuffix(info.Filename, "webp") {
			img, err = webp.Decode(file)
			file.Seek(0, 0)
			if err != nil {
				fmt.Println(w.Tr("UnableToObtainImageSize"))
				return nil, err
			}
			imgType = "webp"
		} else {
			//无法获取图片尺寸
			fmt.Println(w.Tr("UnableToObtainImageSize"))
			return nil, err
		}
	}
	imgType = strings.ToLower(imgType)
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	if imgType == "jpeg" {
		imgType = "jpg"
	}
	oriImgType := imgType
	if imgType == "ico" || imgType == "bmp" {
		imgType = "png"
	}
	//只允许上传jpg,jpeg,gif,png,webp
	if imgType != "jpg" && imgType != "gif" && imgType != "png" && imgType != "webp" {
		return nil, errors.New(w.Tr("UnsupportedImageFormatLog", imgType))
	}

	if attachId == 0 {
		// gif 不转成 webp，因为使用的webp库还不支持
		if w.Content.UseWebp == 1 && (imgType != "gif" || w.Content.ConvertGif == 1) {
			imgType = "webp"
		}
		tmpName = md5Str[8:24] + "." + imgType
	}

	//如果图片宽度大于800，自动压缩到800, gif 不能处理
	resizeWidth := w.Content.ResizeWidth
	if resizeWidth == 0 {
		//默认800
		resizeWidth = 800
	}
	quality := w.Content.Quality
	if quality == 0 {
		// 默认质量是90
		quality = config.DefaultQuality
	}
	if oriImgType == "jpg" {
		j, err2 := library.NewQuality(file)
		file.Seek(0, 0)
		if err2 == nil {
			if j.Quality() < quality {
				quality = j.Quality()
			}
		}
	}

	if w.Content.ResizeImage == 1 && width > resizeWidth && imgType != "gif" {
		img = library.Resize(img, resizeWidth, 0)
		width = img.Bounds().Dx()
		height = img.Bounds().Dy()
	}
	// 保存裁剪的图片
	// 如果服务器安装了ImageMagick，则尝试使用ImageMagick裁剪gif
	iMPath, _ := getImageMagickPath()
	addWatermark := uint(0)
	var buf []byte
	if imgType == "gif" {
		// gif 直接使用原始数据
		_, _ = file.Seek(0, 0)
		buf, _ = io.ReadAll(file)
		if iMPath != "" {
			args := []string{"-coalesce", "-layers", "Optimize"}
			buf2, err := processTempFileWithCmd(bytes.NewBuffer(buf), imgType, iMPath, args, nil)
			if err == nil {
				buf = buf2
			}
		}
	} else {
		// 如果开启图片水印功能，则加水印,gif 不处理
		if w.PluginWatermark.Open {
			wm := w.NewWatermark(w.PluginWatermark)
			if wm != nil {
				img, err = wm.DrawWatermark(img)
				if err == nil {
					addWatermark = 1
				}
			}
		}
		buf, imgType, _ = encodeImage(img, imgType, quality)
	}
	fileSize = int64(len(buf))

	// 上传原图
	_, err = w.Storage.UploadFile(filePath+tmpName, buf)
	if err != nil {
		return nil, err
	}

	//生成宽度为250的缩略图
	thumbName := "thumb_" + tmpName

	if imgType == "gif" && iMPath != "" {
		args := []string{"-coalesce", "-resize"}
		if w.Content.ThumbCrop == 0 {
			// 等比缩放
			args = append(args, fmt.Sprintf("%dx%d", w.Content.ThumbWidth, w.Content.ThumbHeight))
		} else if w.Content.ThumbCrop == 1 {
			// 补白
			args = append(args, fmt.Sprintf("%dx%d", w.Content.ThumbWidth, w.Content.ThumbHeight), "-background", "white", "-extent", fmt.Sprintf("%dx%d", w.Content.ThumbWidth, w.Content.ThumbHeight))
		} else {
			// 裁剪
			args = append(args, fmt.Sprintf("%dx%d^", w.Content.ThumbWidth, w.Content.ThumbHeight), "-gravity", "center", "-extent", fmt.Sprintf("%dx%d", w.Content.ThumbWidth, w.Content.ThumbHeight))
		}
		args = append(args, "-layers", "Optimize")
		buf2, err := processTempFileWithCmd(bytes.NewBuffer(buf), imgType, iMPath, args, nil)
		if err == nil {
			buf = buf2
		} else {
			newImg := library.ThumbnailCrop(w.Content.ThumbWidth, w.Content.ThumbHeight, img, w.Content.ThumbCrop)
			buf, _, _ = encodeImage(newImg, imgType, quality)
		}
	} else {
		newImg := library.ThumbnailCrop(w.Content.ThumbWidth, w.Content.ThumbHeight, img, w.Content.ThumbCrop)
		buf, _, _ = encodeImage(newImg, imgType, quality)
	}

	// 上传缩略图
	_, err = w.Storage.UploadFile(filePath+thumbName, buf)
	if err != nil {
		return nil, err
	}

	//文件上传完成
	attachment = &model.Attachment{
		UserId:       userId,
		FileName:     fileName,
		FileLocation: filePath + tmpName,
		FileSize:     fileSize,
		FileMd5:      md5Str,
		Width:        width,
		Height:       height,
		CategoryId:   categoryId,
		IsImage:      1,
		Watermark:    addWatermark,
		Status:       1,
	}
	attachment.Id = attachId

	err = attachment.Save(db)
	if err != nil {
		return nil, err
	}
	attachment.GetThumb(w.PluginStorage.StorageUrl)

	return attachment, nil
}

func (w *Website) DownloadRemoteImage(src string, fileName string) (*model.Attachment, error) {
	resp, body, errs := gorequest.New().Set("referer", src).Timeout(15 * time.Second).Get(src).EndBytes()
	if errs == nil {
		//处理
		contentType := strings.ToLower(resp.Header.Get("content-type"))
		if contentType == "image/jpeg" || contentType == "image/jpg" || contentType == "image/png" || contentType == "image/gif" || contentType == "image/webp" {
			if fileName == "" {
				fileName = "image"
			}
			fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName)) + "." + strings.Split(contentType, "/")[1]
			//获取宽高
			tmpfile, err := os.CreateTemp("", "download")
			if err != nil {
				return nil, err
			}
			defer os.Remove(tmpfile.Name()) // clean up
			defer tmpfile.Close()
			if _, err := tmpfile.Write(body); err != nil {
				return nil, err
			}
			tmpfile.Seek(0, 0)
			fileHeader := &multipart.FileHeader{
				Filename: filepath.Base(fileName),
				Header:   nil,
				Size:     int64(len(body)),
			}

			return w.AttachmentUpload(tmpfile, fileHeader, 0, 0, 0)
		} else {
			return nil, errors.New(w.Tr("UnsupportedImageFormat"))
		}
	}

	return nil, errs[0]
}

func (w *Website) GetAttachmentByMd5(md5 string) (*model.Attachment, error) {
	db := w.DB
	var attach model.Attachment

	if err := db.Unscoped().Where("`file_md5` = ?", md5).First(&attach).Error; err != nil {
		return nil, err
	}

	attach.GetThumb(w.PluginStorage.StorageUrl)

	return &attach, nil
}

func (w *Website) GetAttachmentById(id uint) (*model.Attachment, error) {
	db := w.DB
	var attach model.Attachment

	if err := db.Where("`id` = ?", id).First(&attach).Error; err != nil {
		return nil, err
	}

	attach.GetThumb(w.PluginStorage.StorageUrl)

	return &attach, nil
}

func (w *Website) GetAttachmentList(categoryId uint, q string, currentPage int, pageSize int) ([]*model.Attachment, int64, error) {
	var attachments []*model.Attachment
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := w.DB.Model(&model.Attachment{})
	if categoryId > 0 {
		builder = builder.Where("`category_id` = ?", categoryId)
	}
	if q != "" {
		builder = builder.Where("`file_name` like ?", "%"+q+"%")
	}
	builder = builder.Where("`status` = 1").Order("updated_time desc")
	if err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&attachments).Error; err != nil {
		return nil, 0, err
	}
	for i := range attachments {
		attachments[i].GetThumb(w.PluginStorage.StorageUrl)
	}

	return attachments, total, nil
}

func (w *Website) ThumbRebuild() {
	db := w.DB
	limit := 1000
	var total int64
	attachmentBuilder := db.Model(&model.Attachment{}).Where("`status` = 1").Order("id desc").Count(&total)
	if total == 0 {
		return
	}

	var attachments []*model.Attachment
	pager := int(math.Ceil(float64(total) / float64(limit)))
	for i := 1; i <= pager; i++ {
		offset := (i - 1) * limit
		err := attachmentBuilder.Limit(limit).Offset(offset).Scan(&attachments).Error
		if err == nil {
			for _, v := range attachments {
				_ = w.BuildThumb(v.FileLocation)
			}
		}
	}
}

func (w *Website) BuildThumb(fileLocation string) error {
	originPath := w.PublicPath + fileLocation

	paths, fileName := filepath.Split(fileLocation)
	thumbPath := paths + "thumb_" + fileName

	f, err := os.Open(originPath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, imgType, err := image.Decode(f)
	if err != nil {
		f.Seek(0, 0)
		img, err = webp.Decode(f)
		if err != nil {
			fmt.Println(w.Tr("UnableToObtainImageSize"))
			return err
		}
		imgType = "webp"
	}
	if imgType == "jpeg" {
		imgType = "jpg"
	}
	quality := w.Content.Quality
	if quality == 0 {
		// 默认质量是90
		quality = config.DefaultQuality
	}

	newImg := library.ThumbnailCrop(w.Content.ThumbWidth, w.Content.ThumbHeight, img, w.Content.ThumbCrop)
	buf, _, _ := encodeImage(newImg, imgType, quality)

	_, err = w.Storage.UploadFile(thumbPath, buf)
	if err != nil {
		return err
	}

	return nil
}

// GetAttachmentCategories 获取所有分类
func (w *Website) GetAttachmentCategories() ([]*model.AttachmentCategory, error) {
	var categories []*model.AttachmentCategory

	err := w.DB.Where("`status` = 1").Order("id desc").Find(&categories).Error
	if err != nil {
		return nil, err
	}

	return categories, nil
}

func (w *Website) GetAttachmentCategoryById(id uint) (*model.AttachmentCategory, error) {
	var category model.AttachmentCategory
	if err := w.DB.Where("id = ?", id).First(&category).Error; err != nil {
		return nil, err
	}

	return &category, nil
}

func (w *Website) ChangeAttachmentCategory(categoryId uint, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	w.DB.Model(&model.Attachment{}).Where("`id` IN(?)", ids).UpdateColumn("category_id", categoryId)

	return nil
}

func (w *Website) DeleteAttachmentCategory(id uint) error {
	category, err := w.GetAttachmentCategoryById(id)
	if err != nil {
		return err
	}

	//如果存在内容，则不能删除
	var attachCount int64
	w.DB.Model(&model.Attachment{}).Where("`category_id` = ?", category.Id).Count(&attachCount)
	if attachCount > 0 {
		return errors.New(w.Tr("PleaseDeleteTheImagesUnderTheCategoryBeforeDeletingTheCategory"))
	}

	//执行删除操作
	err = w.DB.Delete(category).Error

	return err
}

func (w *Website) SaveAttachmentCategory(req *request.AttachmentCategory) (category *model.AttachmentCategory, err error) {
	if req.Id > 0 {
		category, err = w.GetAttachmentCategoryById(req.Id)
		if err != nil {
			return nil, err
		}
	} else {
		category = &model.AttachmentCategory{
			Status: 1,
		}
	}
	category.Title = req.Title
	category.Status = 1

	err = w.DB.Save(category).Error

	if err != nil {
		return
	}
	return
}

func (w *Website) StartConvertImageToWebp() {
	// 根据attachment表读取每一张图片
	type replaced struct {
		From string
		To   string
	}
	lastId := uint(0)
	limit := 500
	for {
		var attaches []model.Attachment
		w.DB.Where("`id` > ?", lastId).Order("id asc").Limit(limit).Find(&attaches)
		if len(attaches) == 0 {
			break
		}
		lastId = attaches[len(attaches)-1].Id
		var results = make([]replaced, 0, len(attaches))
		for _, v := range attaches {
			if !strings.HasSuffix(v.FileLocation, ".jpg") &&
				!strings.HasSuffix(v.FileLocation, ".png") &&
				!strings.HasSuffix(v.FileLocation, ".gif") {
				// 只转换图片
				continue
			}
			result := replaced{
				From: "/" + v.FileLocation,
			}
			// 先转换图片
			err := w.convertToWebp(&v)
			if err == nil {
				// 接着替换内容
				// 替换 attachment file_location,上一步已经完成
				result.To = "/" + v.FileLocation
				results = append(results, result)
			}
		}
		// 替换 category content,images,logo
		var categories []model.Category
		w.DB.Find(&categories)
		for _, v := range categories {
			update := false
			for x := range results {
				if strings.HasSuffix(v.Logo, results[x].From) {
					v.Logo = strings.ReplaceAll(v.Logo, results[x].From, results[x].To)
					update = true
				}
				if len(v.Images) > 0 {
					for y := range v.Images {
						if strings.HasSuffix(v.Images[y], results[x].From) {
							v.Images[y] = strings.ReplaceAll(v.Images[y], results[x].From, results[x].To)
							update = true
						}
					}
				}
				if strings.Contains(v.Content, results[x].From) {
					v.Content = strings.ReplaceAll(v.Content, results[x].From, results[x].To)
					update = true
				}
			}
			if update {
				w.DB.Updates(&v)
			}
		}
		// 替换 archive logo,images
		innerLastId := int64(0)
		for {
			var archives []model.Archive
			w.DB.Where("`id` > ?", innerLastId).Order("id asc").Limit(1000).Find(&archives)
			if len(archives) == 0 {
				break
			}
			innerLastId = archives[len(archives)-1].Id
			for _, v := range archives {
				update := false
				for x := range results {
					if strings.HasSuffix(v.Logo, results[x].From) {
						v.Logo = strings.ReplaceAll(v.Logo, results[x].From, results[x].To)
						update = true
					}
					if len(v.Images) > 0 {
						for y := range v.Images {
							if strings.HasSuffix(v.Images[y], results[x].From) {
								v.Images[y] = strings.ReplaceAll(v.Images[y], results[x].From, results[x].To)
								update = true
							}
						}
					}
				}
				if update {
					w.DB.Updates(&v)
				}
			}
		}
		// 替换 archive_data content
		innerLastId = 0
		for {
			var archiveData []model.ArchiveData
			w.DB.Where("`id` > ?", innerLastId).Order("id asc").Limit(1000).Find(&archiveData)
			if len(archiveData) == 0 {
				break
			}
			innerLastId = archiveData[len(archiveData)-1].Id
			for _, v := range archiveData {
				update := false
				for x := range results {
					if strings.Contains(v.Content, results[x].From) {
						v.Content = strings.ReplaceAll(v.Content, results[x].From, results[x].To)
						update = true
					}
				}
				if update {
					w.DB.Updates(&v)
				}
			}
		}
		// 替换 comment content
		innerLastId = 0
		for {
			var comments []model.Comment
			w.DB.Where("`id` > ?", innerLastId).Order("id asc").Limit(1000).Find(&comments)
			if len(comments) == 0 {
				break
			}
			innerLastId = int64(comments[len(comments)-1].Id)
			for _, v := range comments {
				update := false
				for x := range results {
					if strings.Contains(v.Content, results[x].From) {
						v.Content = strings.ReplaceAll(v.Content, results[x].From, results[x].To)
						update = true
					}
				}
				if update {
					w.DB.Updates(&v)
				}
			}
		}
		// 替换 material content
		innerLastId = 0
		for {
			var materials []model.Material
			w.DB.Where("`id` > ?", innerLastId).Order("id asc").Limit(1000).Find(&materials)
			if len(materials) == 0 {
				break
			}
			innerLastId = int64(materials[len(materials)-1].Id)
			for _, v := range materials {
				update := false
				for x := range results {
					if strings.Contains(v.Content, results[x].From) {
						v.Content = strings.ReplaceAll(v.Content, results[x].From, results[x].To)
						update = true
					}
				}
				if update {
					w.DB.Updates(&v)
				}
			}
		}
		// 替换配置
		update := false
		for x := range results {
			if w.System.SiteLogo == results[x].From {
				w.System.SiteLogo = results[x].To
				update = true
			}
			if w.Contact.Qrcode == results[x].From {
				w.Contact.Qrcode = results[x].To
				update = true
			}
			if w.Content.DefaultThumb == results[x].From {
				w.Content.DefaultThumb = results[x].To
				update = true
			}
		}
		if update {
			_ = w.SaveSettingValue(SystemSettingKey, w.System)
			_ = w.SaveSettingValue(ContactSettingKey, w.Contact)
			_ = w.SaveSettingValue(ContentSettingKey, w.Content)

		}
	}

	log.Println("finished convert to webp")
}

func (w *Website) convertToWebp(attachment *model.Attachment) error {
	originPath := w.PublicPath + attachment.FileLocation
	binPath, err := getWebpPath()
	if err != nil {
		return err
	}
	if strings.HasSuffix(originPath, ".webp") {
		// 已经是webp，不需要处理
		return nil
	}
	newFile := strings.TrimSuffix(attachment.FileLocation, filepath.Ext(attachment.FileLocation)) + ".webp"
	newPath := w.PublicPath + newFile
	quality := w.Content.Quality
	if quality == 0 {
		// 默认质量是90
		quality = config.DefaultQuality
	}
	// 调用命令进行处理
	err = library.RunCmd(binPath, originPath, "−quiet", "-q", strconv.Itoa(quality), "-o", newPath)
	if err != nil {
		return err
	}
	// 检查新生成的文件，并读取它
	buf, err := os.ReadFile(newPath)
	if err != nil {
		return err
	}
	_, err = w.Storage.UploadFile(newFile, buf)
	if err != nil {
		return err
	}

	// 回写
	attachment.FileLocation = newFile
	attachment.FileMd5 = library.Md5Bytes(buf)
	w.DB.Save(attachment)

	// 对缩略图进行处理
	paths, fileName := filepath.Split(attachment.FileLocation)
	thumbPath := w.PublicPath + paths + "thumb_" + fileName
	newThumbPath := strings.TrimSuffix(thumbPath, filepath.Ext(thumbPath)) + ".webp"
	err = library.RunCmd(binPath, thumbPath, "-q", strconv.Itoa(quality), "-o", newThumbPath)
	if err != nil {
		return err
	}
	// 检查新生成的文件，并读取它
	buf, err = os.ReadFile(newPath)
	if err != nil {
		return err
	}
	_, err = w.Storage.UploadFile(strings.TrimPrefix(newThumbPath, w.PublicPath), buf)
	if err != nil {
		return err
	}

	return nil
}

// AttachmentScanUploads 扫描上传目录，所有文件类型
func (w *Website) AttachmentScanUploads(baseDir string) {
	baseDir = strings.TrimRight(baseDir, "/\\")
	files, err := os.ReadDir(baseDir)
	if err != nil {
		log.Println(err)
		return
	}
	for _, fi := range files {
		name := baseDir + "/" + fi.Name()
		if fi.IsDir() {
			// 排除 watermark and titleimage
			if fi.Name() == "watermark" || fi.Name() == "titleimage" {
				continue
			}
			w.AttachmentScanUploads(name)
		} else {
			// 是否是thumb
			if strings.HasPrefix(fi.Name(), "thumb_") {
				// 跳过
				continue
			}
			fileInfo, err := fi.Info()
			if err != nil {
				continue
			}
			var fileLocation = strings.TrimPrefix(name, w.PublicPath)
			fileExt := filepath.Ext(fi.Name())
			// 如果是图片，则生成缩略图
			isImage := 0
			if fileExt == ".jpg" || fileExt == ".png" || fileExt == ".gif" || fileExt == ".webp" {
				isImage = 1
				thumbName := baseDir + "/" + "thumb_" + fi.Name()
				_, err = os.Stat(thumbName)
				if err != nil {
					// 不存在
					_ = w.BuildThumb(fileLocation)
				}
			}

			// 检查是否存在数据库
			var existNum int64
			w.DB.Model(&model.Attachment{}).Where("`file_location` = ?", fileLocation).Count(&existNum)
			if existNum > 0 {
				continue
			}
			md5Str, err := library.Md5File(name)
			if err != nil {
				continue
			}
			//记录文件到数据库
			attachment := &model.Attachment{
				FileName:     fi.Name(),
				FileLocation: fileLocation,
				FileSize:     fileInfo.Size(),
				FileMd5:      md5Str,
				CategoryId:   0,
				IsImage:      isImage,
				Status:       1,
			}
			_ = attachment.Save(w.DB)
		}
	}
}

func (w *Website) GetRandImageFromCategory(categoryId int, title string) string {
	var img string
	// 根据分类每次只取其中一张
	var attach model.Attachment
	if categoryId >= 0 {
		w.DB.Model(&model.Attachment{}).Where("category_id = ? and is_image = ?", categoryId, 1).Order("rand()").Limit(1).Take(&attach)
	} else if categoryId == -1 {
		// 全部图片，所以每次只取其中一张
		w.DB.Model(&model.Attachment{}).Where("is_image = ?", 1).Order("rand()").Limit(1).Take(&attach)
	} else if categoryId == -2 {
		// 尝试关键词匹配图片名称
		// 每次只取其中一张
		// 先分词
		keywordSplit := WordSplit(title, false)
		// 查询attachment表，尝试匹配keywordSplit里的关键词
		tx := w.DB.Model(&model.Attachment{}).Where("is_image = ?", 1)
		var queries []string
		var args []interface{}
		for _, word := range keywordSplit {
			queries = append(queries, "name like ?")
			args = append(args, "%"+word+"%")
		}
		tx = tx.Where(strings.Join(queries, " OR "), args...)

		tx.Order("rand()").Limit(1).Take(&attach)
	}
	if len(attach.FileLocation) > 0 {
		img = w.PluginStorage.StorageUrl + "/" + attach.FileLocation
	}

	return img
}

func (w *Website) GetCategoryImages(categoryId int) []*response.TinyAttachment {
	// 根据分类读取
	var attaches []*response.TinyAttachment
	if categoryId >= 0 {
		w.DB.Model(&model.Attachment{}).Where("category_id = ? and is_image = ?", categoryId, 1).Order("rand()").Scan(&attaches)
	} else {
		// 全部图片
		w.DB.Model(&model.Attachment{}).Where("is_image = ?", 1).Scan(&attaches)
	}
	for i := range attaches {
		attaches[i].FileLocation = w.PluginStorage.StorageUrl + "/" + attaches[i].FileLocation
	}
	// 对attaches 进行随机打乱
	rand.Shuffle(len(attaches), func(i, j int) {
		attaches[i], attaches[j] = attaches[j], attaches[i]
	})

	return attaches
}

func encodeImage(img image.Image, imgType string, quality int) ([]byte, string, error) {
	// 如果 quality 为 0，则使用默认值
	if quality == 0 {
		quality = config.DefaultQuality
	}

	var buf bytes.Buffer
	realType := imgType

	// 先将图片编码到 buf，根据图片类型进行不同处理
	switch imgType {
	case "jpg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, imgType, err
		}
		// jpg 图片无需后续压缩，直接返回
		return buf.Bytes(), imgType, nil

	case "gif":
		if err := gif.Encode(&buf, img, nil); err != nil {
			return nil, imgType, err
		}

	default:
		// 其它图片类型先尝试转换为 png
		if err := png.Encode(&buf, img); err != nil {
			// 如果 png 编码失败，尝试转换成 jpg（先填充白色背景）
			newImg := image.NewRGBA(img.Bounds())
			draw.Draw(newImg, newImg.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
			draw.Draw(newImg, newImg.Bounds(), img, img.Bounds().Min, draw.Over)
			if err = jpeg.Encode(&buf, newImg, &jpeg.Options{Quality: quality}); err != nil {
				return nil, imgType, err
			}
			realType = "jpg"
		} else {
			realType = "png"
		}
	}

	// 如果目标类型是 webp，使用 libwebp 处理
	if imgType == "webp" {
		binPath, err := getWebpPath()
		if err == nil {
			// 调用 helper，指定输出后缀为 webp，传入 webp 的相关参数
			if data, err := processTempFileWithCmd(&buf, realType, binPath, []string{"-q", strconv.Itoa(quality), "-o"}, []string{"-quiet"}); err == nil {
				return data, imgType, nil
			}
		}
		// 出错则退化返回原先的结果
		return buf.Bytes(), realType, nil
	}

	// 对于 gif 和 png，尝试用 ImageMagick 优化（如果可用）
	if imPath, err := getImageMagickPath(); err == nil {
		// 如果是 png，先尝试用 pngquant 进一步压缩
		if imgType == "png" {
			if pngquant, err := getPngquantPath(); err == nil {
				minQuality := quality - 30
				if minQuality < 10 {
					minQuality = 10
				}
				args := []string{"--force", "--skip-if-larger", "--strip", "--quality", fmt.Sprintf("%d-%d", minQuality, quality), "-o"}
				if data, err := processTempFileWithCmd(&buf, imgType, pngquant, args, nil); err == nil {
					return data, imgType, nil
				}
			}
		}
		// ImageMagick 处理：对于 gif 使用 "-layers Optimize"，其它类型额外加上 "-strip", "-quality", "-depth"
		var args []string
		if imgType == "gif" {
			args = []string{"-coalesce", "-layers", "Optimize"}
		} else {
			args = []string{"-strip", "-quality", strconv.Itoa(quality), "-depth", "8"}
		}
		if data, err := processTempFileWithCmd(&buf, imgType, imPath, args, nil); err == nil {
			return data, imgType, nil
		}
	}

	// 所有处理都失败，则返回初步编码的结果
	return buf.Bytes(), imgType, nil
}

// processTempFileWithCmd 封装了将 buf 写入临时文件、调用外部命令处理、读取处理结果的逻辑。
// 参数说明：
// - buf：图片数据缓冲区。
// - tempExt：临时文件的扩展名（即原始编码格式）。
// - cmdPath：外部命令路径（如 libwebp、pngquant、ImageMagick）。
// - args：传递给外部命令的参数（注意不含输入文件路径和输出参数）。
func processTempFileWithCmd(buf *bytes.Buffer, tempExt, cmdPath string, args []string, prefixArgs []string) ([]byte, error) {
	// 创建临时文件，文件名后缀为 tempExt
	tmpFile, err := os.CreateTemp("", "*."+tempExt)
	if err != nil {
		return nil, err
	}
	// 确保临时文件关闭并删除
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()
	// 将缓冲区数据写入临时文件
	if _, err = tmpFile.Write(buf.Bytes()); err != nil {
		return nil, err
	}
	_ = tmpFile.Close()

	// 构造输出文件名
	outName := tmpFile.Name() + ".new"

	// 拼接命令参数：第一个参数为输入文件路径，后续参数、最后输出文件路径
	fullArgs := append([]string{tmpFile.Name()}, args...)
	if len(prefixArgs) > 0 {
		fullArgs = append(prefixArgs, fullArgs...)
	}
	fullArgs = append(fullArgs, outName)

	// 调用外部命令进行处理
	if err = library.RunCmd(cmdPath, fullArgs...); err != nil {
		log.Println("processTempFileWithCmd error:", err)
		return nil, err
	}

	// 检查输出文件是否生成，并读取之
	data, err := os.ReadFile(outName)
	if err != nil {
		log.Println("processTempFileWithCmd error:", err)
		return nil, err
	}
	_ = os.Remove(outName)
	return data, err
}

// UploadByChunks 分片上传处理函数
func (w *Website) UploadByChunks(file multipart.File, fileMd5 string, chunk, chunks int) (*os.File, error) {
	// 临时文件保存6小时
	maxAge := 6 * time.Hour
	// 打开临时文件夹
	tmpDir := w.CachePath + "tmp"
	// 检查并创建文件夹
	_, err := os.Stat(tmpDir)
	if err != nil && os.IsNotExist(err) {
		_ = os.MkdirAll(tmpDir, os.ModePerm)
	}
	// 清理过期的分片文件
	_ = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.ModTime().Add(maxAge).Before(time.Now()) {
			_ = os.Remove(path)
		}
		return nil
	})
	// 开始写入临时文件
	buf, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	tmpName := library.Md5(fileMd5) + "_" + strconv.Itoa(chunk) + ".part"
	err = os.WriteFile(tmpDir+"/"+tmpName, buf, os.ModePerm)
	if err != nil {
		return nil, err
	}
	// 验证分片是否全部上传完毕
	done := true
	for i := 0; i < chunks; i++ {
		tmpPart := tmpDir + "/" + library.Md5(fileMd5) + "_" + strconv.Itoa(i) + ".part"
		if _, err = os.Stat(tmpPart); err != nil && os.IsNotExist(err) {
			done = false
			break
		}
	}

	if done {
		// 将文件写入到临时文件
		tmpFile, err := os.CreateTemp(tmpDir, "upload_")
		if err != nil {
			return nil, err
		}
		for i := 0; i < chunks; i++ {
			tmpPart := tmpDir + "/" + library.Md5(fileMd5) + "_" + strconv.Itoa(i) + ".part"
			buf2, err2 := os.ReadFile(tmpPart)
			if err2 != nil {
				_ = tmpFile.Close()
				return nil, err2
			}
			_, err = tmpFile.Write(buf2)
			if err != nil {
				_ = tmpFile.Close()
				return nil, err
			}
			_ = os.Remove(tmpPart)
		}

		return tmpFile, nil
	}

	return nil, nil
}

func getImageMagickPath() (string, error) {
	if imageMagickPath != "" {
		return imageMagickPath, nil
	}
	// 同时检查 Imagemagick 7 和 Imagemagick 6
	filePath, err := exec.LookPath("magick")
	if err != nil {
		filePath, err = exec.LookPath("convert")
		if err != nil {
			return "", err
		}
	}
	imageMagickPath = filePath
	return imageMagickPath, nil
}

// 检查系统安装的pngquant，如果存在，则返回路径
// pngquant 不内置，需要自行安装
func getPngquantPath() (string, error) {
	if pngquantPath != "" {
		return pngquantPath, nil
	}
	filePath, err := exec.LookPath("pngquant")
	if err != nil {
		return "", err
	}
	pngquantPath = filePath
	return pngquantPath, nil
}

func getWebpPath() (string, error) {
	if webpPath != "" {
		return webpPath, nil
	}
	goos := runtime.GOOS
	arch := runtime.GOARCH
	binName := "cwebp_" + goos + "_" + arch
	if goos == "windows" {
		binName += ".exe"
	}
	binPath := config.ExecPath + "source/" + binName
	if _, err := os.Stat(binPath); err != nil {
		if os.IsNotExist(err) {
			// 尝试坚持系统安装的 cwebp
			binPath, err = exec.LookPath("cwebp")
			if err != nil {
				return "", err
			}
		}
	}
	webpPath = binPath
	return webpPath, nil
}
