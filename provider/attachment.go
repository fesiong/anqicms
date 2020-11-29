package provider

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/nfnt/resize"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"log"
	"mime/multipart"
	"os"
	"path"
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
	width := uint(img.Bounds().Dx())
	height := uint(img.Bounds().Dy())
	fmt.Println("width = ", width, " height = ", height)
	//只允许上传jpg,jpeg,gif,png
	if imgType != "jpg" && imgType != "jpeg" && imgType != "gif" && imgType != "png" {
		return nil, errors.New(fmt.Sprintf("不支持的图片格式：%s。", imgType))
	}
	if imgType == "jpeg" {
		imgType = "jpg"
	}

	fileName := strings.TrimSuffix(info.Filename, path.Ext(info.Filename))
	log.Printf(fileName)

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

	//如果图片宽度大于750，自动压缩到750, gif 不能处理
	buff := &bytes.Buffer{}

	if width > 750 && imgType != "gif" {
		newImg := library.Resize(750, 0, img, resize.Lanczos3)
		width = uint(newImg.Bounds().Dx())
		height = uint(newImg.Bounds().Dy())
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

	//_, err = c.Object.Put(context.Background(), filePath + tmpName, buff, nil)
	//if err != nil {
	//	ctx.JSON(iris.Map{
	//		"status": config.StatusFailed,
	//		"msg":    err.Error(),
	//	})
	//	return
	//}
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

	originFile, err := os.OpenFile(basePath + filePath + tmpName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
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

	newImg := library.ThumbnailCrop(250, 250, img)
	if imgType == "jpg" {
		_ = jpeg.Encode(buff, newImg, nil)
	} else if imgType == "png" {
		_ = png.Encode(buff, newImg)
	} else if imgType == "gif" {
		_ = gif.Encode(buff, newImg, nil)
	}

	thumbFile, err := os.OpenFile(basePath + filePath + thumbName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
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
		Id:           0,
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

func GetAttachmentByMd5(md5 string) (*model.Attachment, error) {
	db := config.DB
	var attach model.Attachment

	if err := db.Where("`status` != 99").Where("`file_md5` = ?", md5).First(&attach).Error; err != nil {
		return nil, err
	}

	attach.GetThumb()

	return &attach, nil
}