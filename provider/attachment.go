package provider

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/parnurzeal/gorequest"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"math"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func AttachmentUpload(file multipart.File, info *multipart.FileHeader) (*model.Attachment, error) {
	db := config.DB
	//获取宽高
	bufFile := bufio.NewReader(file)
	img, imgType, err := image.Decode(bufFile)
	if err != nil {
		//无法获取图片尺寸
		fmt.Println("无法获取图片尺寸")
		return nil, err
	}
	imgType = strings.ToLower(imgType)
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	//只允许上传jpg,jpeg,gif,png
	if imgType != "jpg" && imgType != "jpeg" && imgType != "gif" && imgType != "png" {
		return nil, errors.New(fmt.Sprintf("不支持的图片格式：%s。", imgType))
	}
	if imgType == "jpeg" {
		imgType = "jpg"
	}

	fileName := strings.TrimSuffix(info.Filename, path.Ext(info.Filename))

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}
	//获取文件的MD5，检查数据库是否已经存在，存在则不用重复上传
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
	buff := &bytes.Buffer{}

	if config.JsonData.Content.ResizeImage == 1 && width > resizeWidth && imgType != "gif" {
		newImg := library.Resize(img, resizeWidth, 0)
		width = newImg.Bounds().Dx()
		height = newImg.Bounds().Dy()
		if imgType == "jpg" {
			// 保存裁剪的图片
			_ = jpeg.Encode(buff, newImg, nil)
		} else if imgType == "png" {
			// 保存裁剪的图片
			_ = png.Encode(buff, newImg)
		}
	} else {
		_, _ = io.Copy(buff, file)
	}

	tmpName := md5Str[8:24] + "." + imgType
	filePath := strconv.Itoa(time.Now().Year()) + strconv.Itoa(int(time.Now().Month())) + "/" + strconv.Itoa(time.Now().Day()) + "/"

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
	if imgType == "jpg" {
		_ = jpeg.Encode(buff, newImg, nil)
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
		FileSize:     int64(info.Size),
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
}

func DownloadRemoteImage(src string, fileName string) (*model.Attachment, error) {
	db := config.DB
	resp, body, errs := gorequest.New().Set("referer", src).Timeout(30 * time.Second).Get(src).EndBytes()
	if errs == nil {
		//处理
		contentType := resp.Header.Get("content-type")
		if contentType == "image/jpeg" || contentType == "image/jpg" || contentType == "image/png" || contentType == "image/gif" {
			//获取宽高
			bufFile := &bytes.Buffer{}
			bufFile.Write(body)
			img, imgType, err := image.Decode(bufFile)
			if err != nil {
				//无法获取图片尺寸
				fmt.Println("无法获取图片尺寸")
				return nil, err
			}
			bufFile.Reset()
			bufFile.Write(body)
			imgType = strings.ToLower(imgType)
			width := img.Bounds().Dx()
			height := img.Bounds().Dy()
			//只允许上传jpg,jpeg,gif,png
			if imgType != "jpg" && imgType != "jpeg" && imgType != "gif" && imgType != "png" {
				return nil, errors.New(fmt.Sprintf("不支持的图片格式：%s。", imgType))
			}
			if imgType == "jpeg" {
				imgType = "jpg"
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
				if attachment.Status != 1 {
					//更新status
					attachment.Status = 1
					err = attachment.Save(db)
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
			buff := &bytes.Buffer{}

			if config.JsonData.Content.ResizeImage == 1 && width > resizeWidth && imgType != "gif" {
				newImg := library.Resize(img, resizeWidth, 0)
				width = newImg.Bounds().Dx()
				height = newImg.Bounds().Dy()
				if imgType == "jpg" {
					// 保存裁剪的图片
					_ = jpeg.Encode(buff, newImg, nil)
				} else if imgType == "png" {
					// 保存裁剪的图片
					_ = png.Encode(buff, newImg)
				}
			} else {
				_, _ = io.Copy(buff, bufFile)
			}

			tmpName := md5Str[8:24] + "." + imgType
			filePath := strconv.Itoa(time.Now().Year()) + strconv.Itoa(int(time.Now().Month())) + "/" + strconv.Itoa(time.Now().Day()) + "/"

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
			if imgType == "jpg" {
				_ = jpeg.Encode(buff, newImg, nil)
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
				FileSize:     int64(len(body)),
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
			return nil, errors.New("不支持的图片格式")
		}
	}

	return nil, errs[0]
}

func GetAttachmentByMd5(md5 string) (*model.Attachment, error) {
	db := config.DB
	var attach model.Attachment

	if err := db.Unscoped().Where("`file_md5` = ?", md5).First(&attach).Error; err != nil {
		return nil, err
	}

	attach.GetThumb()

	return &attach, nil
}

func GetAttachmentById(id uint) (*model.Attachment, error) {
	db := config.DB
	var attach model.Attachment

	if err := db.Where("`id` = ?", id).First(&attach).Error; err != nil {
		return nil, err
	}

	attach.GetThumb()

	return &attach, nil
}

func GetAttachmentList(currentPage int, pageSize int) ([]*model.Attachment, int64, error) {
	var attachments []*model.Attachment
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := config.DB.Model(&model.Attachment{}).Where("`status` = 1").Order("id desc")
	if err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&attachments).Error; err != nil {
		return nil, 0, err
	}
	for i := range attachments {
		attachments[i].GetThumb()
	}

	return attachments, total, nil
}

func ThumbRebuild() {
	db := config.DB
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
		//无法获取图片尺寸
		fmt.Println("无法获取图片尺寸")
		return err
	}
	if imgType == "jpeg" {
		imgType = "jpg"
	}

	buff := &bytes.Buffer{}
	newImg := library.ThumbnailCrop(config.JsonData.Content.ThumbWidth, config.JsonData.Content.ThumbHeight, img, 1)

	if imgType == "jpg" {
		_ = jpeg.Encode(buff, newImg, nil)
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
