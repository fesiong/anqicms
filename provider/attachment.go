package provider

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/chai2010/webp"
	"github.com/parnurzeal/gorequest"
	"gopkg.in/gographics/imagick.v3/imagick"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"log"
	"math"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func (w *Website) AttachmentUpload(file multipart.File, info *multipart.FileHeader, categoryId uint, attachId uint) (*model.Attachment, error) {
	db := w.DB

	fileExt := filepath.Ext(info.Filename)
	if fileExt == ".php" {
		return nil, errors.New("不允许上传php文件")
	}
	if fileExt == ".jpeg" {
		fileExt = ".jpg"
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
			return nil, errors.New(w.Lang("需要替换的图片资源不存在"))
		}
		isImage = attachment.IsImage
	}

	fileSize := info.Size
	fileName := strings.TrimSuffix(info.Filename, path.Ext(info.Filename))
	// 获取md5
	md5hash := md5.New()
	file.Seek(0, 0)
	_, err = io.Copy(md5hash, file)
	if err != nil {
		return nil, err
	}
	file.Seek(0, 0)
	md5Str := hex.EncodeToString(md5hash.Sum(nil))
	exists, _ := w.GetAttachmentByMd5(md5Str)
	if attachment != nil {
		if exists != nil && attachment.Id != exists.Id {
			return nil, errors.New(w.Lang("替换失败，已存在当前上传的资源。"))
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

	// 不是图片的时候的处理方法
	if isImage != 1 {
		bts, _ := io.ReadAll(file)
		_, err = w.Storage.UploadFile(filePath+tmpName, bts)
		if err != nil {
			return nil, err
		}
		//文件上传完成
		attachment = &model.Attachment{
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
				fmt.Println(w.Lang("无法获取图片尺寸"))
				return nil, err
			}
			imgType = "webp"
		} else {
			//无法获取图片尺寸
			fmt.Println(w.Lang("无法获取图片尺寸"))
			return nil, err
		}
	}
	imgType = strings.ToLower(imgType)
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	if imgType == "jpeg" {
		imgType = "jpg"
	}
	//只允许上传jpg,jpeg,gif,png,webp
	if imgType != "jpg" && imgType != "jpeg" && imgType != "gif" && imgType != "png" && imgType != "webp" {
		return nil, errors.New(fmt.Sprintf("%s: %s。", w.Lang("不支持的图片格式"), imgType))
	}

	if attachId == 0 {
		if w.Content.UseWebp == 1 {
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
		quality = webp.DefaulQuality
	}

	if w.Content.ResizeImage == 1 && width > resizeWidth && imgType != "gif" {
		img = library.Resize(img, resizeWidth, 0)
		width = img.Bounds().Dx()
		height = img.Bounds().Dy()
	}
	// 保存裁剪的图片
	buf, _ := encodeImage(img, imgType, quality)
	fileSize = int64(len(buf))

	// 上传原图
	_, err = w.Storage.UploadFile(filePath+tmpName, buf)
	if err != nil {
		return nil, err
	}

	//生成宽度为250的缩略图
	thumbName := "thumb_" + tmpName

	newImg := library.ThumbnailCrop(w.Content.ThumbWidth, w.Content.ThumbHeight, img, w.Content.ThumbCrop)
	buf, _ = encodeImage(newImg, imgType, quality)

	// 上传原图
	_, err = w.Storage.UploadFile(filePath+thumbName, buf)
	if err != nil {
		return nil, err
	}

	//文件上传完成
	attachment = &model.Attachment{
		FileName:     fileName,
		FileLocation: filePath + tmpName,
		FileSize:     fileSize,
		FileMd5:      md5Str,
		Width:        width,
		Height:       height,
		CategoryId:   categoryId,
		IsImage:      1,
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
		contentType := resp.Header.Get("content-type")
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

			return w.AttachmentUpload(tmpfile, fileHeader, 0, 0)
		} else {
			return nil, errors.New(w.Lang("不支持的图片格式"))
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
				_ = w.BuildThumb(v)
			}
		}
	}
}

func (w *Website) BuildThumb(attachment *model.Attachment) error {
	originPath := w.PublicPath + attachment.FileLocation

	paths, fileName := filepath.Split(attachment.FileLocation)
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
			fmt.Println(w.Lang("无法获取图片尺寸"))
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
		quality = webp.DefaulQuality
	}

	newImg := library.ThumbnailCrop(w.Content.ThumbWidth, w.Content.ThumbHeight, img, w.Content.ThumbCrop)
	buf, _ := encodeImage(newImg, imgType, quality)

	_, err = w.Storage.UploadFile(thumbPath, buf)
	if err != nil {
		return err
	}

	return nil
}

// GetAttachmentCategories 获取所有分类
func (w *Website) GetAttachmentCategories() ([]*model.AttachmentCategory, error) {
	var categories []*model.AttachmentCategory

	err := w.DB.Where("`status` = 1").Find(&categories).Error
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
		return errors.New(w.Lang("请删除分类下的图片，才能删除分类"))
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
		innerLastId := uint(0)
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
		innerLastId = uint(0)
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
		innerLastId = uint(0)
		for {
			var comments []model.Comment
			w.DB.Where("`id` > ?", innerLastId).Order("id asc").Limit(1000).Find(&comments)
			if len(comments) == 0 {
				break
			}
			innerLastId = comments[len(comments)-1].Id
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
		innerLastId = uint(0)
		for {
			var materials []model.Material
			w.DB.Where("`id` > ?", innerLastId).Order("id asc").Limit(1000).Find(&materials)
			if len(materials) == 0 {
				break
			}
			innerLastId = materials[len(materials)-1].Id
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

	// 对原图处理
	f, err := os.Open(originPath)
	if err != nil {
		return err
	}
	defer f.Close()

	newFile := strings.TrimSuffix(attachment.FileLocation, filepath.Ext(attachment.FileLocation)) + ".webp"

	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Println(w.Lang("无法获取图片尺寸"))
		return err
	}
	quality := w.Content.Quality
	if quality == 0 {
		// 默认质量是90
		quality = webp.DefaulQuality
	}
	buff := &bytes.Buffer{}
	_ = webp.Encode(buff, img, &webp.Options{Lossless: false, Quality: float32(quality)})
	err = os.WriteFile(w.PublicPath+newFile, buff.Bytes(), os.ModePerm)
	if err != nil {
		return err
	}
	_, err = w.Storage.UploadFile(newFile, buff.Bytes())
	if err != nil {
		return err
	}

	// 回写
	attachment.FileLocation = newFile
	attachment.FileMd5 = library.Md5Bytes(buff.Bytes())
	w.DB.Save(attachment)

	paths, fileName := filepath.Split(attachment.FileLocation)
	thumbPath := w.PublicPath + paths + "thumb_" + fileName

	buff.Reset()
	newImg := library.ThumbnailCrop(w.Content.ThumbWidth, w.Content.ThumbHeight, img, w.Content.ThumbCrop)

	_ = webp.Encode(buff, newImg, &webp.Options{Lossless: false, Quality: float32(quality)})

	err = os.WriteFile(thumbPath, buff.Bytes(), os.ModePerm)
	if err != nil {
		return err
	}
	_, err = w.Storage.UploadFile(paths+"thumb_"+fileName, buff.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func encodeImage(img image.Image, imgType string, quality int) ([]byte, error) {
	buff := &bytes.Buffer{}

	if imgType == "webp" {
		_ = webp.Encode(buff, img, &webp.Options{Lossless: false, Quality: float32(quality)})
		// 先返回，不用再compress
		return buff.Bytes(), nil
	} else if imgType == "jpg" {
		if err := jpeg.Encode(buff, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
	} else if imgType == "png" {
		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()
		// 创建一个8位全彩图像并保留Alpha通道
		outputImg := image.NewRGBA(image.Rect(0, 0, width, height))
		// 使用循环将24位图像的每个像素转换为8位全彩图像
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				pixel := img.At(x, y)
				originalColor := color.RGBAModel.Convert(pixel).(color.RGBA)
				alpha := originalColor.A
				// 将24位图像的RGB颜色和Alpha通道值赋值给8位全彩图像
				outputImg.Set(x, y, color.RGBA{originalColor.R, originalColor.G, originalColor.B, alpha})
			}
		}
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		err := encoder.Encode(buff, outputImg)
		if err != nil {
			newImg := image.NewRGBA(img.Bounds())
			draw.Draw(newImg, newImg.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
			draw.Draw(newImg, newImg.Bounds(), img, img.Bounds().Min, draw.Over)
			if err := jpeg.Encode(buff, newImg, &jpeg.Options{Quality: quality}); err != nil {
				return nil, err
			}
		}
	} else if imgType == "gif" {
		if err := gif.Encode(buff, img, nil); err != nil {
			return nil, err
		}
	}
	return buff.Bytes(), nil
}

// compressImage 只能压缩png/jpg
func compressImage(img []byte, quality int) ([]byte, error) {
	imagick.Initialize()
	defer imagick.Terminate()
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	mw.ReadImageBlob(img)
	if err := mw.SetImageCompressionQuality(uint(quality)); err != nil {
		return nil, err
	}
	if err := mw.SetImageFormat("PNG"); err != nil {
		return nil, err
	}
	image := mw.GetImage()
	defer image.Destroy() // 销毁当前帧图片对应实例
	return image.GetImageBlob(), nil
}
