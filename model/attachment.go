package model

import (
	"goblog/config"
	"goblog/utils"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/satori/go.uuid"
)

type Attachment struct {
	ID       uint   `gorm:"primary_key" json:"id"`
	Title    string `json:"title"`
	Location string `json:"location"`
	FileSize uint   `json:"fileSize"`
	Width    uint   `json:"width"`
	Height   uint   `json:"height"`
	AddTime  int64  `json:"addTime"`
	Mime     string `json:"mime"`
}

// ImageUploadedInfo 图片上传后的相关信息(目录、文件路径、文件名、UUIDName、请求URL)
type ImageUploadedInfo struct {
	UploadDir      string
	UploadFilePath string
	Filename       string
	UUIDName       string
	ImgURL         string
}

// GenerateImgUploadedInfo 创建一个ImageUploadedInfo
func GenerateImgUploadedInfo(ext string) ImageUploadedInfo {
	sep := string(os.PathSeparator)
	uploadImgDir := config.ServerConfig.UploadDir
	length := utf8.RuneCountInString(uploadImgDir)
	lastChar := uploadImgDir[length-1:]
	ymStr := utils.GetTodayYM(sep)

	var uploadDir string
	if lastChar != sep {
		uploadDir = uploadImgDir + sep + ymStr
	} else {
		uploadDir = uploadImgDir + ymStr
	}

	uuidName := uuid.NewV4().String()
	filename := uuidName + ext
	uploadFilePath := uploadDir + sep + filename
	imgURL := strings.Join([]string{
		"http://" + config.ServerConfig.StaticHost + config.ServerConfig.UploadPath,
		ymStr,
		filename,
	}, "/")
	return ImageUploadedInfo{
		ImgURL:         imgURL,
		UUIDName:       uuidName,
		Filename:       filename,
		UploadDir:      uploadDir,
		UploadFilePath: uploadFilePath,
	}
}
