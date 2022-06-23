package provider

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/chai2010/webp"
	"github.com/parnurzeal/gorequest"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
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

func AttachmentUpload(file multipart.File, info *multipart.FileHeader, categoryId uint, attachId uint) (*model.Attachment, error) {
	db := dao.DB
	//获取宽高
	fileSize := info.Size
	bufFile := bufio.NewReader(file)
	img, imgType, err := image.Decode(bufFile)
	if err != nil {
		if strings.HasSuffix(info.Filename, "mp4") {
			file.Seek(0, io.SeekStart)
			return AttachmentUploadVideo(file, info, categoryId)
		}

		if strings.HasSuffix(info.Filename, "webp") {
			bufFile.Reset(file)
			img, err = webp.Decode(bufFile)
			if err != nil {
				fmt.Println(config.Lang("无法获取图片尺寸"))
				return nil, err
			}
			imgType = "webp"
		} else {
			//无法获取图片尺寸
			fmt.Println(config.Lang("无法获取图片尺寸"))
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
		return nil, errors.New(fmt.Sprintf("%s: %s。", config.Lang("不支持的图片格式"), imgType))
	}

	fileName := strings.TrimSuffix(info.Filename, path.Ext(info.Filename))

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}
	//获取文件的MD5，检查数据库是否已经存在，则采用覆盖方式处理
	md5hash := md5.New()
	bufFile = bufio.NewReader(file)
	_, err = io.Copy(md5hash, bufFile)
	if err != nil {
		return nil, err
	}
	md5Str := hex.EncodeToString(md5hash.Sum(nil))
	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	var attachment *model.Attachment
	if attachId > 0 {
		attachment, err = GetAttachmentById(attachId)
		if err != nil {
			return nil, errors.New("需要替换的图片资源不存在")
		}
		fileName = attachment.FileName
		imgType = strings.TrimPrefix(filepath.Ext(attachment.FileLocation), ".")
		// 如果上传的图片已存在，则不允许替换
		exists, err := GetAttachmentByMd5(md5Str)
		if err == nil && attachment.Id != exists.Id {
			return nil, errors.New("替换失败，已存在当前上传的图片。")
		}
	}

	if attachId == 0 {
		if config.JsonData.Content.UseWebp == 1 {
			imgType = "webp"
		}
		attachment, err = GetAttachmentByMd5(md5Str)
		if err == nil {
			if attachment.DeletedAt.Valid {
				//更新
				err = db.Model(attachment).Update("deleted_at", nil).Error
				if err != nil {
					return nil, err
				}
			}
			// 如果更换了分类
			if categoryId > 0 && attachment.CategoryId != categoryId {
				db.Model(attachment).UpdateColumn("category_id", categoryId)
			}
			fileName = attachment.FileName
			imgType = strings.TrimPrefix(filepath.Ext(attachment.FileLocation), ".")
			attachId = attachment.Id
		}
	}

	//如果图片宽度大于800，自动压缩到800, gif 不能处理
	resizeWidth := config.JsonData.Content.ResizeWidth
	if resizeWidth == 0 {
		//默认800
		resizeWidth = 800
	}
	quality := config.JsonData.Content.Quality
	if quality == 0 {
		// 默认质量是90
		quality = webp.DefaulQuality
	}
	buff := &bytes.Buffer{}

	if config.JsonData.Content.ResizeImage == 1 && width > resizeWidth && imgType != "gif" {
		img = library.Resize(img, resizeWidth, 0)
		width = img.Bounds().Dx()
		height = img.Bounds().Dy()
	}
	// 保存裁剪的图片
	if imgType == "webp" {
		_ = webp.Encode(buff, img, &webp.Options{Lossless: false, Quality: float32(quality)})
		log.Println("webp:", quality)
	} else if imgType == "jpg" {
		_ = jpeg.Encode(buff, img, &jpeg.Options{Quality: quality})
	} else if imgType == "png" {
		_ = png.Encode(buff, img)
	} else if imgType == "gif" {
		_ = gif.Encode(buff, img, nil)
	}
	fileSize = int64(buff.Len())

	tmpName := md5Str[8:24] + "." + imgType
	filePath := time.Now().Format("200601/02/")
	if attachId > 0 {
		filePath = filepath.Dir(attachment.FileLocation) + "/"
		tmpName = filepath.Base(attachment.FileLocation)
	}

	//将文件写入本地
	basePath := config.ExecPath + "public/uploads/"
	//先判断文件夹是否存在，不存在就先创建
	_, err = os.Stat(basePath + filePath)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(basePath+filePath, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}

	originFile, err := os.OpenFile(basePath+filePath+tmpName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		//无法创建
		return nil, err
	}

	defer originFile.Close()

	_, err = io.Copy(originFile, buff)
	if err != nil {
		//文件写入失败
		return nil, err
	}

	//生成宽度为250的缩略图
	thumbName := "thumb_" + tmpName

	newImg := library.ThumbnailCrop(config.JsonData.Content.ThumbWidth, config.JsonData.Content.ThumbHeight, img, config.JsonData.Content.ThumbCrop)
	if imgType == "webp" {
		_ = webp.Encode(buff, newImg, &webp.Options{Lossless: false, Quality: float32(quality)})
	} else if imgType == "jpg" {
		_ = jpeg.Encode(buff, newImg, &jpeg.Options{Quality: quality})
	} else if imgType == "png" {
		_ = png.Encode(buff, newImg)
	} else if imgType == "gif" {
		_ = gif.Encode(buff, newImg, nil)
	}

	thumbFile, err := os.OpenFile(basePath+filePath+thumbName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		//无法创建
		return nil, err
	}

	defer thumbFile.Close()

	_, err = io.Copy(thumbFile, buff)
	if err != nil {
		//文件写入失败
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
		Status:       1,
	}
	attachment.Id = attachId

	err = attachment.Save(db)
	if err != nil {
		return nil, err
	}
	attachment.GetThumb()

	return attachment, nil
}

func AttachmentUploadVideo(file multipart.File, info *multipart.FileHeader, categoryId uint) (*model.Attachment, error) {
	// 视频不做验证
	fileName := strings.TrimSuffix(info.Filename, path.Ext(info.Filename))
	//获取文件的MD5，检查数据库是否已经存在，存在则不用重复上传
	md5hash := md5.New()
	bufFile := bufio.NewReader(file)
	_, err := io.Copy(md5hash, bufFile)
	if err != nil {
		return nil, err
	}
	md5Str := hex.EncodeToString(md5hash.Sum(nil))
	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	attachment, err := GetAttachmentByMd5(md5Str)
	if err == nil {
		if attachment.DeletedAt.Valid {
			//更新
			err = dao.DB.Model(attachment).Update("deleted_at", nil).Error
			if err != nil {
				return nil, err
			}
		}
		// 如果更换了分类
		if categoryId > 0 && attachment.CategoryId != categoryId {
			dao.DB.Model(attachment).UpdateColumn("category_id", categoryId)
		}
		//直接返回
		return attachment, nil
	}

	tmpName := md5Str[8:24] + path.Ext(info.Filename)
	filePath := time.Now().Format("200601/02/")

	//将文件写入本地
	basePath := config.ExecPath + "public/uploads/"
	//先判断文件夹是否存在，不存在就先创建
	_, err = os.Stat(basePath + filePath)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(basePath+filePath, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}

	originFile, err := os.OpenFile(basePath+filePath+tmpName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		//无法创建
		return nil, err
	}

	defer originFile.Close()

	_, err = io.Copy(originFile, file)
	if err != nil {
		//文件写入失败
		return nil, err
	}

	//文件上传完成
	attachment = &model.Attachment{
		FileName:     fileName,
		FileLocation: filePath + tmpName,
		FileSize:     info.Size,
		FileMd5:      md5Str,
		CategoryId:   categoryId,
		Status:       1,
	}
	attachment.GetThumb()

	err = attachment.Save(dao.DB)
	if err != nil {
		return nil, err
	}

	return attachment, nil
}

func DownloadRemoteImage(src string, fileName string) (*model.Attachment, error) {
	db := dao.DB
	resp, body, errs := gorequest.New().Set("referer", src).Timeout(30 * time.Second).Get(src).EndBytes()
	if errs == nil {
		//处理
		contentType := resp.Header.Get("content-type")
		if contentType == "image/jpeg" || contentType == "image/jpg" || contentType == "image/png" || contentType == "image/gif" || contentType == "image/webp" {
			//获取宽高
			bufFile := &bytes.Buffer{}
			bufFile.Write(body)
			img, imgType, err := image.Decode(bufFile)
			if err != nil {
				bufFile.Reset()
				bufFile.Write(body)
				img, err = webp.Decode(bufFile)
				if err != nil {
					fmt.Println(config.Lang("无法获取图片尺寸"))
					return nil, err
				}
				imgType = "webp"
			}
			bufFile.Reset()
			bufFile.Write(body)
			imgType = strings.ToLower(imgType)
			width := img.Bounds().Dx()
			height := img.Bounds().Dy()
			//只允许上传jpg,jpeg,gif,png
			if imgType != "jpg" && imgType != "jpeg" && imgType != "gif" && imgType != "png" && imgType != "webp" {
				return nil, errors.New(fmt.Sprintf("%s: %s。", config.Lang("不支持的图片格式"), imgType))
			}
			if imgType == "jpeg" {
				imgType = "jpg"
			}
			if config.JsonData.Content.UseWebp == 1 {
				imgType = "webp"
			}

			//获取文件的MD5，检查数据库是否已经存在，存在则不用重复上传
			md5hash := md5.New()
			_, err = io.Copy(md5hash, bufFile)
			if err != nil {
				return nil, err
			}
			bufFile.Reset()
			bufFile.Write(body)
			md5Str := hex.EncodeToString(md5hash.Sum(nil))

			attachment, err := GetAttachmentByMd5(md5Str)
			if err == nil {
				if attachment.DeletedAt.Valid {
					//更新
					err = db.Model(attachment).Update("deleted_at", nil).Error
					if err != nil {
						return nil, err
					}
				}
				//直接返回
				return attachment, nil
			}

			//如果图片宽度大于800，自动压缩到800, gif 不能处理
			resizeWidth := config.JsonData.Content.ResizeWidth
			if resizeWidth == 0 {
				//默认800
				resizeWidth = 800
			}
			quality := config.JsonData.Content.Quality
			if quality == 0 {
				// 默认质量是90
				quality = webp.DefaulQuality
			}
			buff := &bytes.Buffer{}

			if config.JsonData.Content.ResizeImage == 1 && width > resizeWidth && imgType != "gif" {
				img = library.Resize(img, resizeWidth, 0)
				width = img.Bounds().Dx()
				height = img.Bounds().Dy()

			}
			if imgType == "webp" {
				_ = webp.Encode(buff, img, &webp.Options{Lossless: false, Quality: float32(quality)})
			} else if imgType == "jpg" {
				_ = jpeg.Encode(buff, img, &jpeg.Options{Quality: quality})
			} else if imgType == "png" {
				_ = png.Encode(buff, img)
			} else if imgType == "gif" {
				_ = gif.Encode(buff, img, nil)
			}
			fileSize := int64(buff.Len())

			tmpName := md5Str[8:24] + "." + imgType
			filePath := time.Now().Format("200601/02/")

			if fileName == "" {
				fileName = strings.TrimSuffix(tmpName, path.Ext(tmpName))
			}

			//将文件写入本地
			basePath := config.ExecPath + "public/uploads/"
			//先判断文件夹是否存在，不存在就先创建
			_, err = os.Stat(basePath + filePath)
			if err != nil && os.IsNotExist(err) {
				err = os.MkdirAll(basePath+filePath, os.ModePerm)
				if err != nil {
					return nil, err
				}
			}

			originFile, err := os.OpenFile(basePath+filePath+tmpName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				//无法创建
				return nil, err
			}

			defer originFile.Close()

			_, err = io.Copy(originFile, buff)
			if err != nil {
				//文件写入失败
				return nil, err
			}

			//生成宽度为250的缩略图
			thumbName := "thumb_" + tmpName

			newImg := library.ThumbnailCrop(config.JsonData.Content.ThumbWidth, config.JsonData.Content.ThumbHeight, img, config.JsonData.Content.ThumbCrop)
			if imgType == "webp" {
				_ = webp.Encode(buff, newImg, &webp.Options{Lossless: false, Quality: float32(quality)})
			} else if imgType == "jpg" {
				_ = jpeg.Encode(buff, newImg, &jpeg.Options{Quality: quality})
			} else if imgType == "png" {
				_ = png.Encode(buff, newImg)
			} else if imgType == "gif" {
				_ = gif.Encode(buff, newImg, nil)
			}

			thumbFile, err := os.OpenFile(basePath+filePath+thumbName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				//无法创建
				return nil, err
			}

			defer thumbFile.Close()

			_, err = io.Copy(thumbFile, buff)
			if err != nil {
				//文件写入失败
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
				Status:       1,
			}
			attachment.GetThumb()

			err = attachment.Save(db)
			if err != nil {
				return nil, err
			}

			return attachment, nil
		} else {
			return nil, errors.New(config.Lang("不支持的图片格式"))
		}
	}

	return nil, errs[0]
}

func GetAttachmentByMd5(md5 string) (*model.Attachment, error) {
	db := dao.DB
	var attach model.Attachment

	if err := db.Unscoped().Where("`file_md5` = ?", md5).First(&attach).Error; err != nil {
		return nil, err
	}

	attach.GetThumb()

	return &attach, nil
}

func GetAttachmentById(id uint) (*model.Attachment, error) {
	db := dao.DB
	var attach model.Attachment

	if err := db.Where("`id` = ?", id).First(&attach).Error; err != nil {
		return nil, err
	}

	attach.GetThumb()

	return &attach, nil
}

func GetAttachmentList(categoryId uint, currentPage int, pageSize int) ([]*model.Attachment, int64, error) {
	var attachments []*model.Attachment
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := dao.DB.Model(&model.Attachment{})
	if categoryId > 0 {
		builder = builder.Where("`category_id` = ?", categoryId)
	}
	builder = builder.Where("`status` = 1").Order("updated_time desc")
	if err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&attachments).Error; err != nil {
		return nil, 0, err
	}
	for i := range attachments {
		attachments[i].GetThumb()
	}

	return attachments, total, nil
}

func ThumbRebuild() {
	db := dao.DB
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
				_ = BuildThumb(v)
			}
		}
	}
}

func BuildThumb(attachment *model.Attachment) error {
	basePath := config.ExecPath + "public/uploads/"
	originPath := basePath + attachment.FileLocation

	paths, fileName := filepath.Split(attachment.FileLocation)
	thumbPath := basePath + paths + "thumb_" + fileName

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
			fmt.Println(config.Lang("无法获取图片尺寸"))
			return err
		}
		imgType = "webp"
	}
	if imgType == "jpeg" {
		imgType = "jpg"
	}
	quality := config.JsonData.Content.Quality
	if quality == 0 {
		// 默认质量是90
		quality = webp.DefaulQuality
	}

	buff := &bytes.Buffer{}
	newImg := library.ThumbnailCrop(config.JsonData.Content.ThumbWidth, config.JsonData.Content.ThumbHeight, img, config.JsonData.Content.ThumbCrop)

	if imgType == "webp" {
		_ = webp.Encode(buff, newImg, &webp.Options{Lossless: false, Quality: float32(quality)})
	} else if imgType == "jpg" {
		_ = jpeg.Encode(buff, newImg, &jpeg.Options{Quality: quality})
	} else if imgType == "png" {
		_ = png.Encode(buff, newImg)
	} else if imgType == "gif" {
		_ = gif.Encode(buff, newImg, nil)
	}

	thumbFile, err := os.OpenFile(thumbPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		//无法创建
		return err
	}

	defer thumbFile.Close()
	_, err = io.Copy(thumbFile, buff)
	if err != nil {
		//文件写入失败
		return err
	}

	return nil
}

// GetAttachmentCategories 获取所有分类
func GetAttachmentCategories() ([]*model.AttachmentCategory, error) {
	var categories []*model.AttachmentCategory

	err := dao.DB.Where("`status` = 1").Find(&categories).Error
	if err != nil {
		return nil, err
	}

	return categories, nil
}

func GetAttachmentCategoryById(id uint) (*model.AttachmentCategory, error) {
	var category model.AttachmentCategory
	if err := dao.DB.Where("id = ?", id).First(&category).Error; err != nil {
		return nil, err
	}

	return &category, nil
}

func ChangeAttachmentCategory(categoryId uint, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	dao.DB.Model(&model.Attachment{}).Where("`id` IN(?)", ids).UpdateColumn("category_id", categoryId)

	return nil
}

func DeleteAttachmentCategory(id uint) error {
	category, err := GetAttachmentCategoryById(id)
	if err != nil {
		return err
	}

	//如果存在内容，则不能删除
	var attachCount int64
	dao.DB.Model(&model.Attachment{}).Where("`category_id` = ?", category.Id).Count(&attachCount)
	if attachCount > 0 {
		return errors.New(config.Lang("请删除分类下的图片，才能删除分类"))
	}

	//执行删除操作
	err = dao.DB.Delete(category).Error

	return err
}

func SaveAttachmentCategory(req *request.AttachmentCategory) (category *model.AttachmentCategory, err error) {
	if req.Id > 0 {
		category, err = GetAttachmentCategoryById(req.Id)
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

	err = dao.DB.Save(category).Error

	if err != nil {
		return
	}
	return
}

func StartConvertImageToWebp() {
	// 根据attachment表读取每一张图片
	type replaced struct {
		From string
		To   string
	}
	lastId := uint(0)
	limit := 500
	for {
		var attaches []model.Attachment
		dao.DB.Where("`id` > ?", lastId).Order("id asc").Limit(limit).Find(&attaches)
		if len(attaches) == 0 {
			break
		}
		lastId = attaches[len(attaches)-1].Id
		var results = make([]replaced, 0, len(attaches))
		for _, v := range attaches {
			if strings.HasSuffix(v.FileLocation, ".webp") {
				continue
			}
			result := replaced{
				From: "/uploads/" + v.FileLocation,
			}
			// 先转换图片
			err := convertToWebp(&v)
			if err == nil {
				// 接着替换内容
				// 替换 attachment file_location,上一步已经完成
				result.To = "/uploads/" + v.FileLocation
				results = append(results, result)
			}
		}
		// 替换 category content,images,logo
		var categories []model.Category
		dao.DB.Find(&categories)
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
				dao.DB.Updates(&v)
			}
		}
		// 替换 archive logo,images
		innerLastId := uint(0)
		for {
			var archives []model.Archive
			dao.DB.Where("`id` > ?", innerLastId).Order("id asc").Limit(1000).Find(&archives)
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
					dao.DB.Updates(&v)
				}
			}
		}
		// 替换 archive_data content
		innerLastId = uint(0)
		for {
			var archiveData []model.ArchiveData
			dao.DB.Where("`id` > ?", innerLastId).Order("id asc").Limit(1000).Find(&archiveData)
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
					dao.DB.Updates(&v)
				}
			}
		}
		// 替换 comment content
		innerLastId = uint(0)
		for {
			var comments []model.Comment
			dao.DB.Where("`id` > ?", innerLastId).Order("id asc").Limit(1000).Find(&comments)
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
					dao.DB.Updates(&v)
				}
			}
		}
		// 替换 material content
		innerLastId = uint(0)
		for {
			var materials []model.Material
			dao.DB.Where("`id` > ?", innerLastId).Order("id asc").Limit(1000).Find(&materials)
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
					dao.DB.Updates(&v)
				}
			}
		}
		// 替换配置
		update := false
		for x := range results {
			if config.JsonData.System.SiteLogo == results[x].From {
				config.JsonData.System.SiteLogo = results[x].To
				update = true
			}
			if config.JsonData.Contact.Qrcode == results[x].From {
				config.JsonData.Contact.Qrcode = results[x].To
				update = true
			}
			if config.JsonData.Content.DefaultThumb == results[x].From {
				config.JsonData.Content.DefaultThumb = results[x].To
				update = true
			}
		}
		if update {
			config.WriteConfig()
		}
	}

	log.Println("finished convert to webp")
}

func convertToWebp(attachment *model.Attachment) error {
	basePath := config.ExecPath + "public/uploads/"
	originPath := basePath + attachment.FileLocation

	// 对原图处理
	f, err := os.Open(originPath)
	if err != nil {
		return err
	}
	defer f.Close()

	newFile := strings.TrimSuffix(attachment.FileLocation, filepath.Ext(attachment.FileLocation)) + ".webp"

	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Println(config.Lang("无法获取图片尺寸"))
		return err
	}
	quality := config.JsonData.Content.Quality
	if quality == 0 {
		// 默认质量是90
		quality = webp.DefaulQuality
	}
	buff := &bytes.Buffer{}
	_ = webp.Encode(buff, img, &webp.Options{Lossless: false, Quality: float32(quality)})
	err = ioutil.WriteFile(basePath+newFile, buff.Bytes(), os.ModePerm)
	if err != nil {
		return err
	}

	// 回写
	attachment.FileLocation = newFile
	attachment.FileMd5 = library.Md5Bytes(buff.Bytes())
	dao.DB.Save(attachment)

	paths, fileName := filepath.Split(attachment.FileLocation)
	thumbPath := basePath + paths + "thumb_" + fileName

	buff.Reset()
	newImg := library.ThumbnailCrop(config.JsonData.Content.ThumbWidth, config.JsonData.Content.ThumbHeight, img, config.JsonData.Content.ThumbCrop)

	_ = webp.Encode(buff, newImg, &webp.Options{Lossless: false, Quality: float32(quality)})

	err = ioutil.WriteFile(thumbPath, buff.Bytes(), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
