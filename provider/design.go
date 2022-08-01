package provider

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func GetDesignList() []response.DesignPackage {
	// 读取目录
	designPath := config.ExecPath + "template"
	entries, err := os.ReadDir(designPath)
	if err != nil {
		return nil
	}

	var designLists []response.DesignPackage

	for _, v := range entries {
		// .开头的排除
		if strings.HasPrefix(v.Name(), ".") {
			continue
		}
		if v.IsDir() {
			var hasChange = false
			configFile := designPath + "/" + v.Name() + "/" + "config.json"
			var designInfo response.DesignPackage
			info, err := os.Stat(configFile)
			if err != nil {
				// 文件不存在
				// 尝试生成
				designInfo = response.DesignPackage{
					Name:        v.Name(),
					Version:     "",
					Created:     time.Now().Format("2006-01-02 15:04:05"),
					TplFiles:    nil,
					StaticFiles: nil,
				}
				hasChange = true
			} else {
				data, err := ioutil.ReadFile(configFile)
				if err != nil {
					// 无法读取，只能跳过
					continue
				}
				err = json.Unmarshal(data, &designInfo)

				if err != nil {
					// 解析失败
					designInfo = response.DesignPackage{
						Name:        v.Name(),
						Version:     "",
						Created:     info.ModTime().Format("2006-01-02 15:04:05"),
						TplFiles:    nil,
						StaticFiles: nil,
					}
					hasChange = true
				}
			}
			if designInfo.Package != v.Name() {
				designInfo.Package = v.Name()
				hasChange = true
			}

			if hasChange {
				_ = writeDesignInfo(&designInfo)
			}

			if designInfo.Package == config.JsonData.System.TemplateName {
				designInfo.Status = 1
			} else {
				designInfo.Status = 0
			}

			designLists = append(designLists, designInfo)
		}
	}

	return designLists
}

func SaveDesignInfo(req request.DesignInfoRequest) error {
	designList := GetDesignList()
	var designIndex = -1
	for i := range designList {
		if designList[i].Package == req.Package {
			designIndex = i
			break
		}
	}
	if designIndex == -1 {
		return errors.New("模板不存在")
	}

	designInfo := designList[designIndex]

	designInfo.Name = req.Name
	designInfo.TemplateType = req.TemplateType
	//designInfo.Description = req.Description
	//designInfo.Version = req.Version
	//designInfo.Author = req.Author
	//designInfo.Homepage = req.Homepage
	//designInfo.Created = req.Created

	err := writeDesignInfo(&designInfo)

	return err
}

// DeleteDesignInfo 删除的模板，会被移动到 cache
func DeleteDesignInfo(packageName string) error {
	if packageName == "" {
		return errors.New("模板不存在")
	}
	if packageName == "default" {
		return errors.New("默认模板不能删除")
	}

	basePath := config.ExecPath + "template/" + packageName
	basePath = filepath.Clean(basePath)
	if !strings.HasPrefix(basePath, config.ExecPath + "template/") {
		return errors.New("模板不存在")
	}

	cachePath := config.ExecPath + "cache/" + ".history/" + packageName
	os.MkdirAll(cachePath, os.ModePerm)

	os.RemoveAll(cachePath + "/template")
	os.Rename(basePath, cachePath + "/template")
	// 读取静态文件
	staticPath := config.ExecPath + "public/static/" + packageName
	os.RemoveAll(cachePath + "/static")
	os.MkdirAll(cachePath, os.ModePerm)
	os.Rename(staticPath, cachePath + "/static")

	return nil
}

func GetDesignInfo(packageName string, scan bool) (*response.DesignPackage, error) {
	designList := GetDesignList()
	var designIndex = -1
	for i := range designList {
		if designList[i].Package == packageName {
			designIndex = i
			break
		}
	}
	if designIndex == -1 {
		return nil, errors.New("模板不存在")
	}

	if !scan {
		return &designList[designIndex], nil
	}

	basePath := config.ExecPath + "template/" + packageName
	var hasChange = false

	designInfo := designList[designIndex]
	// 尝试读取模板文件
	files := readAllFiles(basePath)
	if len(files) < len(designInfo.TplFiles) {
		hasChange = true
	}
	for i := range files {
		if strings.HasSuffix(files[i].Path, "config.json") {
			continue
		}
		fullPath := strings.TrimPrefix(files[i].Path, basePath+"/")
		var exists = false
		for j := range designInfo.TplFiles {
			if designInfo.TplFiles[j].Path == fullPath {
				designInfo.TplFiles[j].Size = files[i].Size
				designInfo.TplFiles[j].LastMod = files[i].LastMod
				exists = true
				// 已存在，跳过
				break
			}
		}
		if !exists {
			designInfo.TplFiles = append(designInfo.TplFiles, response.DesignFile{
				Path:   fullPath,
				Remark: "",
				Size: files[i].Size,
				LastMod: files[i].LastMod,
			})
			hasChange = true
		}
	}
	// 读取静态文件
	staticPath := config.ExecPath + "public/static/" + packageName
	files = readAllFiles(staticPath)
	if len(files) < len(designInfo.StaticFiles) {
		hasChange = true
	}
	for i := range files {
		fullPath := strings.TrimPrefix(files[i].Path, staticPath+"/")
		var exists = false
		for j := range designInfo.StaticFiles {
			if designInfo.StaticFiles[j].Path == fullPath {
				designInfo.StaticFiles[j].Size = files[i].Size
				designInfo.StaticFiles[j].LastMod = files[i].LastMod
				exists = true
				// 已存在，跳过
				break
			}
		}
		if !exists {
			designInfo.StaticFiles = append(designInfo.StaticFiles, response.DesignFile{
				Path:   fullPath,
				Remark: "",
				Size: files[i].Size,
				LastMod: files[i].LastMod,
			})
			hasChange = true
		}
	}

	if hasChange {
		saveFile := designInfo
		_ = writeDesignInfo(&saveFile)
	}

	return &designInfo, nil
}

func UploadDesignZip(file multipart.File, info *multipart.FileHeader) error {
	// 解压
	zipReader, err := zip.NewReader(file, info.Size)
	if err != nil {
		return err
	}

	packageName := strings.TrimSuffix(info.Filename, path.Ext(info.Filename))
	// 先尝试读取config.json
	tmpFile, err := zipReader.Open("template/config.json")
	if err == nil {
		data, err := ioutil.ReadAll(tmpFile)
		if err == nil {
			var designInfo response.DesignPackage
			err = json.Unmarshal(data, &designInfo)
			if err == nil {
				packageName = designInfo.Package
			}
		}
	}
	// 检查是否已经存在
	packagePath := config.ExecPath + "template/" + packageName
	_, err = os.Stat(packagePath)
	if err == nil {
		// 已存在
		i := 1
		for {
			packagePath = fmt.Sprintf("%stemplate/%s%d", config.ExecPath, packageName, i)
			_, err = os.Stat(packagePath)
			if err != nil {
				packageName = fmt.Sprintf("%s%d", packageName, i)
				break
			}
			i++
		}
	}

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		fileExt := filepath.Ext(f.Name)
		if fileExt == ".php" {
			continue
		}
		f.Name = strings.ReplaceAll(f.Name, "..", "")
		f.Name = strings.ReplaceAll(f.Name, "\\", "")
		var realName string
		// 模板文件
		if strings.HasPrefix(f.Name, "template/") {
			if fileExt != ".html" && fileExt != ".json" {
				continue
			}
			basePath := filepath.Join(config.ExecPath, "template", packageName)
			realName = filepath.Clean(basePath + "/" + strings.TrimPrefix(f.Name, "template/"))
			if !strings.HasPrefix(realName, basePath) {
				continue
			}
		}
		// static
		if strings.HasPrefix(f.Name, "static/") {
			basePath := filepath.Join(config.ExecPath, "public/static", packageName)
			realName = filepath.Clean(basePath + "/" + strings.TrimPrefix(f.Name, "static/"))
			if !strings.HasPrefix(realName, basePath) {
				continue
			}
		}

		reader, err := f.Open()
		if err != nil {
			continue
		}
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
	}

	return nil
}

func CreateDesignZip(packageName string) (*bytes.Buffer, error) {
	buff := &bytes.Buffer{}

	archive := zip.NewWriter(buff)
	defer archive.Close()

	// 读取模板
	basePath := config.ExecPath + "template/" + packageName
	// 尝试读取模板文件
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return nil, err
	}
	for _, info := range files {
		fullName := basePath + "/" + info.Name()
		file, err := os.Open(fullName)
		if err != nil {
			return nil, err
		}
		err = compress(file, "template", archive)
		if err != nil {
			return nil, err
		}
	}
	// 读取静态文件
	staticPath := config.ExecPath + "public/static/" + packageName
	files, err = ioutil.ReadDir(staticPath)
	if err != nil {
		return nil, err
	}
	for _, info := range files {
		fullName := staticPath + "/" + info.Name()
		file, err := os.Open(fullName)
		if err != nil {
			return nil, err
		}
		err = compress(file, "static", archive)
		if err != nil {
			return nil, err
		}
	}

	return buff, nil
}

func compress(file *os.File, prefix string, zw *zip.Writer) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	defer file.Close()
	if strings.HasPrefix(info.Name(), ".") {
		return nil
	}
	if prefix != "" {
		prefix += "/"
	}
	if info.IsDir() {
		prefix = prefix + info.Name()
		fileInfos, err := file.Readdir(-1)
		if err != nil {
			return err
		}
		for _, fi := range fileInfos {
			f, err := os.Open(file.Name() + "/" + fi.Name())
			if err != nil {
				return err
			}
			err = compress(f, prefix, zw)
			if err != nil {
				return err
			}
		}
	} else {
		header, err := zip.FileInfoHeader(info)
		header.Name = prefix + header.Name
		if err != nil {
			return err
		}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, file)
		if err != nil {
			return err
		}
	}
	return nil
}

func UploadDesignFile(file multipart.File, info *multipart.FileHeader, packageName, fileType, filePath string) error {
	fileExt := filepath.Ext(info.Filename)
	if fileExt == ".php" {
		return errors.New("不能上传php文件")
	}

	designInfo, err := GetDesignInfo(packageName, false)
	if err != nil {
		return err
	}

	var basePath string
	filePath = strings.ReplaceAll(filePath, "..", "")
	var realPath string
	if fileType == "static" {
		// 资源
		basePath = filepath.Join(config.ExecPath, "public/static", designInfo.Package)
		realPath = filepath.Clean(basePath + "/" + filePath)
		if !strings.HasPrefix(realPath, basePath) {
			return errors.New("不能越级上传模板")
		}
	} else {
		if fileExt != ".html" && fileExt != ".zip" {
			return errors.New("请上传html模板")
		}
		basePath = filepath.Join(config.ExecPath, "template", designInfo.Package)
		realPath = filepath.Clean(basePath + "/" + filePath)
		if !strings.HasPrefix(realPath, basePath) {
			return errors.New("不能越级上传模板")
		}
	}
	realPath = strings.TrimRight(realPath, "/")
	if fileExt == ".zip" {
		// 解压
		zipReader, err := zip.NewReader(file, info.Size)
		if err != nil {
			return err
		}
		for _, f := range zipReader.File {
			if f.FileInfo().IsDir() {
				continue
			}
			ext := filepath.Ext(f.Name)
			if ext == ".php" {
				continue
			}
			if fileType != "static" && ext != ".html" {
				continue
			}
			f.Name = strings.ReplaceAll(f.Name, "..", "")
			f.Name = strings.ReplaceAll(f.Name, "\\", "")
			realFile := filepath.Clean(realPath + "/" + f.Name)
			if !strings.HasPrefix(realFile, basePath) {
				continue
			}

			reader, err := f.Open()
			if err != nil {
				continue
			}
			_ = os.MkdirAll(filepath.Dir(realFile), os.ModePerm)
			newFile, err := os.Create(realFile)
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
		}
	} else {
		info.Filename = strings.ReplaceAll(info.Filename, "..", "")
		info.Filename = strings.ReplaceAll(info.Filename, "\\", "")
		realFile := filepath.Clean(realPath + "/" + info.Filename)
		if !strings.HasPrefix(realFile, basePath) {
			return errors.New("不能越级上传文件")
		}
		// 单独文件处理
		newFile, err := os.Create(realFile)
		if err != nil {
			return err
		}
		_, err = io.Copy(newFile, file)
		if err != nil {
			newFile.Close()
			return err
		}

		_ = newFile.Close()
	}

	return nil
}

func GetDesignFileDetail(packageName, filePath, fileType string, scan bool) (*response.DesignFile, error) {
	designInfo, err := GetDesignInfo(packageName, false)
	if err != nil {
		return nil, errors.New("模板不存在")
	}

	filePath = strings.ReplaceAll(filePath, "..", "")
	var designFileDetail response.DesignFile
	var exists = false
	if filePath == "" && len(designInfo.TplFiles) > 0 {
		filePath = designInfo.TplFiles[0].Path
	}
	if fileType == "static" {
		// 保存模板静态文件
		for i := range designInfo.StaticFiles {
			if designInfo.StaticFiles[i].Path == filePath {
				designFileDetail = designInfo.StaticFiles[i]
				exists = true
				break
			}
		}
	} else {
		for i := range designInfo.TplFiles {
			if designInfo.TplFiles[i].Path == filePath {
				designFileDetail = designInfo.TplFiles[i]
				exists = true
				break
			}
		}
	}

	var basePath string
	var realPath string
	if fileType == "static" {
		// 资源
		basePath = filepath.Join(config.ExecPath, "public/static", designInfo.Package)
		realPath = filepath.Clean(basePath + "/" + filePath)
		if !strings.HasPrefix(realPath, basePath) {
			return nil, errors.New("文件不存在")
		}
	} else {
		basePath = filepath.Join(config.ExecPath, "template", designInfo.Package)
		realPath = filepath.Clean(basePath + "/" + filePath)
		if !strings.HasPrefix(realPath, basePath) {
			return nil, errors.New("文件不存在")
		}
	}

	_, err = os.Stat(realPath)
	if err != nil && os.IsNotExist(err) {
		return nil, errors.New("文件不存在")
	}

	if !exists {
		designFileDetail = response.DesignFile{
			Path:    filePath,
		}
	}

	if !scan {
		return &designFileDetail, nil
	}

	if fileType == "static" {
		return GetDesignStaticFileDetail(packageName, designFileDetail)
	}

	return GetDesignTplFileDetail(packageName, designFileDetail)
}

func GetDesignTplFileDetail(packageName string, designFileDetail response.DesignFile) (*response.DesignFile, error) {

	fullPath := config.ExecPath + "template/" + packageName + "/" + designFileDetail.Path
	info, err := os.Stat(fullPath)
	if err != nil {
		return &designFileDetail, nil
		//return nil, errors.New("模板文件读取失败")
	}

	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return &designFileDetail, nil
		//return nil, errors.New("模板文件读取失败")
	}

	designFileDetail.LastMod = info.ModTime().Unix()
	designFileDetail.Content = string(data)

	return &designFileDetail, nil
}

func GetDesignStaticFileDetail(packageName string, designFileDetail response.DesignFile) (*response.DesignFile, error) {

	fullPath := config.ExecPath + "public/static/" + packageName + "/" + designFileDetail.Path
	info, err := os.Stat(fullPath)
	if err != nil {
		return &designFileDetail, nil
		//return nil, errors.New("模板文件读取失败")
	}

	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return &designFileDetail, nil
		//return nil, errors.New("模板文件读取失败")
	}

	designFileDetail.LastMod = info.ModTime().Unix()
	designFileDetail.Content = string(data)

	return &designFileDetail, nil
}

func GetDesignFileHistories(packageName, filePath, fileType string) []response.DesignFileHistory {
	designFileDetail, err := GetDesignFileDetail(packageName, filePath, fileType, false)
	if err != nil {
		return nil
	}

	// 读取 .history
	pathMd5 := library.Md5(designFileDetail.Path)
	historyPath := config.ExecPath + "cache/" + ".history/" + packageName + "/" + pathMd5
	_, err = os.Stat(historyPath)
	if err != nil {
		return nil
	}

	files := readAllFiles(historyPath)
	var histories = make([]response.DesignFileHistory, 0, len(files))
	for i := range files {
		histories = append(histories, response.DesignFileHistory{
			Hash:    filepath.Base(files[i].Path),
			LastMod: files[i].LastMod,
			Size: files[i].Size,
		})
	}

	return histories
}

func StoreDesignHistory(packageName string, filePath string, content []byte) error {
	pathMd5 := library.Md5(filePath)
	historyPath := config.ExecPath + "cache/" + ".history/" + packageName + "/" + pathMd5
	historyHash := library.Md5Bytes(content)
	// 先判断目录是否存在
	_, err := os.Stat(historyPath)
	if err != nil && os.IsNotExist(err) {
		// 尝试创建目录
		err = os.MkdirAll(historyPath, os.ModePerm)
		if err != nil {
			return err
		}
	}
	// 开始写入文件
	err = ioutil.WriteFile(historyPath+"/"+historyHash, content, os.ModePerm)

	return err
}

func DeleteDesignHistoryFile(packageName, filePath, historyHash, fileType string) error {
	designFileDetail, err := GetDesignFileDetail(packageName, filePath, fileType, false)
	if err != nil {
		return err
	}

	histories := GetDesignFileHistories(packageName, filePath, fileType)
	var exists = false
	for i := range histories {
		if histories[i].Hash == historyHash {
			exists = true
		}
	}
	if !exists {
		return nil
	}

	pathMd5 := library.Md5(designFileDetail.Path)
	historyPath := config.ExecPath + "cache/" + ".history/" + packageName + "/" + pathMd5 + "/" + historyHash

	_, err = os.Stat(historyPath)
	if err != nil {
		return nil
	}

	err = os.Remove(historyPath)
	if err != nil {
		return err
	}

	return nil
}

func RestoreDesignFile(packageName, filePath, historyHash, fileType string) error {
	designFileDetail, err := GetDesignFileDetail(packageName, filePath, fileType, false)
	if err != nil {
		return err
	}

	histories := GetDesignFileHistories(packageName, filePath, fileType)
	var exists = false
	for i := range histories {
		if histories[i].Hash == historyHash {
			exists = true
		}
	}
	if !exists {
		return errors.New("未找到历史记录")
	}

	pathMd5 := library.Md5(designFileDetail.Path)
	historyPath := config.ExecPath + "cache/" + ".history/" + packageName + "/" + pathMd5 + "/" + historyHash

	var fullPath string
	// 保存html模板
	if fileType == "static" {
		// 保存模板静态文件
		fullPath = config.ExecPath + "public/static/" + packageName + "/" + designFileDetail.Path
	} else {
		fullPath = config.ExecPath + "template/" + packageName + "/" + designFileDetail.Path
	}

	_, err = os.Stat(historyPath)
	if err != nil {
		return err
	}

	err = os.Rename(historyPath, fullPath)
	if err != nil {
		return err
	}

	return nil
}

func DeleteDesignFile(packageName, filePath, fileType string) error {
	// 先验证文件名是否合法
	designInfo, err := GetDesignInfo(packageName, false)
	if err != nil {
		return errors.New("模板不存在")
	}

	if fileType == "static" {
		// 保存模板静态文件
		for i := range designInfo.StaticFiles {
			if designInfo.StaticFiles[i].Path == filePath {
				designInfo.StaticFiles = append(designInfo.StaticFiles[:i], designInfo.StaticFiles[i+1:]...)
				break
			}
		}
	} else {
		for i := range designInfo.TplFiles {
			if designInfo.TplFiles[i].Path == filePath {
				designInfo.TplFiles = append(designInfo.TplFiles[:i], designInfo.TplFiles[i+1:]...)
				break
			}
		}
	}

	// 删除物理文件
	var basePath string
	if fileType == "static" {
		// 静态文件
		basePath = config.ExecPath + "public/static/" + packageName
	} else {
		basePath = config.ExecPath + "template/" + packageName
	}
	fullPath := filepath.Clean(basePath + "/" + filePath)
	if strings.HasPrefix(fullPath, basePath) {
		os.Remove(fullPath)

		// 暂时不删除 history
		//pathMd5 := library.Md5(designFileDetail.Path)
		//historyPath := config.ExecPath + "cache/" + ".history/" + packageName + "/" + pathMd5
		//_, err = os.Stat(historyPath)
		//if err != nil {
		//	return nil
		//}
		//os.RemoveAll(historyPath)
	}
	// 更新文件
	err = writeDesignInfo(designInfo)

	return nil
}

func SaveDesignFile(req request.SaveDesignFileRequest) error {
	// 先验证文件名是否合法
	designInfo, err := GetDesignInfo(req.Package, false)
	if err != nil {
		return errors.New("模板不存在")
	}

	// 先检查文件是否存在
	var basePath string
	if req.Type == "static" {
		basePath = config.ExecPath + "public/static/" + req.Package + "/"
	} else {
		basePath = config.ExecPath + "template/" + req.Package + "/"
	}
	fullPath := filepath.Clean(basePath + req.Path)
	if !strings.HasPrefix(fullPath, basePath) {
		return errors.New("模板文件不存在")
	}

	if req.UpdateContent {
		// 修改内容
		if req.Type == "static" {
			return SaveDesignStaticFile(req)
		}
		// 保存模板静态文件
		return SaveDesignTplFile(req)
	} else {
		// 修改备注名称等
		var designFileDetail response.DesignFile
		var existsIndex = -1
		if req.Type == "static" {
			for i := range designInfo.StaticFiles {
				if designInfo.StaticFiles[i].Path == req.Path {
					designFileDetail = designInfo.StaticFiles[i]
					existsIndex = i
					break
				}
			}
		} else {
			for i := range designInfo.TplFiles {
				if designInfo.TplFiles[i].Path == req.Path {
					designFileDetail = designInfo.TplFiles[i]
					existsIndex = i
					break
				}
			}
		}
		// 如果进行了重命名
		if req.RenamePath != "" && req.RenamePath != req.Path {
			newPath := filepath.Clean(basePath + req.RenamePath)
			if !strings.HasPrefix(newPath, basePath) {
				return errors.New("模板文件保存失败")
			}
			req.Path = strings.TrimPrefix(newPath, basePath)
			// 移动
			_, err = os.Stat(fullPath)
			if err != nil {
				return err
			}
			err = os.Rename(fullPath, newPath)
			if err != nil {
				return err
			}

		}
		//
		if existsIndex == -1 {
			if req.Remark != "" {
				// 写入文件
				designFileDetail = response.DesignFile{
					Path:    req.Path,
					Remark:  req.Remark,
					Content: "",
					LastMod: 0,
				}
				if req.Type != "static" {
					designInfo.TplFiles = append(designInfo.TplFiles, designFileDetail)
				} else {
					designInfo.StaticFiles = append(designInfo.StaticFiles, designFileDetail)
				}
			}
		} else {
			designFileDetail.Remark = req.Remark
			if req.Type != "static" {
				designInfo.TplFiles[existsIndex] = designFileDetail
			} else {
				designInfo.StaticFiles[existsIndex] = designFileDetail
			}
		}
		// 更新文件
		err = writeDesignInfo(designInfo)

		return err
	}
}

func writeDesignInfo(designInfo *response.DesignPackage) error {
	// 更新文件
	basePath := config.ExecPath + "template/" + designInfo.Package + "/"
	configFile := basePath + "config.json"
	// 保存之前，清理只记录有remark的文件到列表
	var newFiles []response.DesignFile
	for i := range designInfo.TplFiles {
		if designInfo.TplFiles[i].Remark != "" {
			_, err := os.Stat(basePath + designInfo.TplFiles[i].Path)
			if err != nil || !os.IsNotExist(err) {
				newFiles = append(newFiles, designInfo.TplFiles[i])
			}
		}
	}
	designInfo.TplFiles = newFiles

	var newStaticFiles []response.DesignFile
	baseStaticPath := config.ExecPath + "public/static/" + designInfo.Package + "/"
	for i := range designInfo.StaticFiles {
		if designInfo.StaticFiles[i].Remark != "" {
			_, err := os.Stat(baseStaticPath + designInfo.StaticFiles[i].Path)
			if err != nil || !os.IsNotExist(err) {
				newStaticFiles = append(newStaticFiles, designInfo.StaticFiles[i])
			}
		}
	}
	designInfo.StaticFiles = newStaticFiles

	buf, err := json.MarshalIndent(designInfo, "", "\t")
	if err == nil {
		// 解析失败
		err = ioutil.WriteFile(configFile, buf, os.ModePerm)
	}

	return err
}

func SaveDesignTplFile(req request.SaveDesignFileRequest) error {
	// 不能越级到上级
	basePath := config.ExecPath + "template/" + req.Package + "/"
	fullPath := filepath.Clean(basePath + req.Path)

	// 尝试创建历史记录
	_, err := os.Stat(fullPath)
	if err == nil {
		// 文件存在，验证内容的md5, 如果一致，就不保存
		oldBytes, _ := ioutil.ReadFile(fullPath)
		oldMd5 := library.Md5Bytes(oldBytes)
		newMd5 := library.Md5(req.Content)

		if oldMd5 == newMd5 {
			if req.Content != "" {
				// MD5 一致, 直接返回
				return nil
			}
		} else {
			// 否则，写入历史记录文件
			_ = StoreDesignHistory(req.Package, req.Path, oldBytes)
			// 写入历史失败不报错
		}
	} else {
		// 文件不存在
		filePath := filepath.Dir(fullPath)
		_, err = os.Stat(filePath)
		if os.IsNotExist(err) {
			os.MkdirAll(filePath, os.ModePerm)
		}
	}

	err = ioutil.WriteFile(fullPath, []byte(req.Content), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func SaveDesignStaticFile(req request.SaveDesignFileRequest) error {
	// 不能越级到上级
	basePath := config.ExecPath + "public/static/" + req.Package + "/"
	fullPath := filepath.Clean(basePath + req.Path)

	// 尝试创建历史记录
	_, err := os.Stat(fullPath)
	if err == nil {
		// 文件存在，验证内容的md5, 如果一致，就不保存
		oldBytes, _ := ioutil.ReadFile(fullPath)
		oldMd5 := library.Md5Bytes(oldBytes)
		newMd5 := library.Md5(req.Content)

		if oldMd5 == newMd5 {
			if req.Content != "" {
				// MD5 一致, 直接返回
				return nil
			}
		} else {
			// 否则，写入历史记录文件
			_ = StoreDesignHistory(req.Package, req.Path, oldBytes)
			// 写入历史失败不报错
		}
	} else {
		// 文件不存在
		filePath := filepath.Dir(fullPath)
		_, err = os.Stat(filePath)
		if os.IsNotExist(err) {
			os.MkdirAll(filePath, os.ModePerm)
		}
	}

	err = ioutil.WriteFile(fullPath, []byte(req.Content), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func readAllFiles(dir string) []response.DesignFile {
	if !isDirExist(dir) {
		return []response.DesignFile{}
	}

	files, _ := ioutil.ReadDir(dir)
	var fileList []response.DesignFile
	for _, file := range files {
		// .开头的，除了 .htaccess 其他都排除
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}

		fullName := dir + "/" + file.Name()

		if file.IsDir() {
			fileList = append(fileList, readAllFiles(fullName)...)
		} else {
			fileList = append(fileList, response.DesignFile{
				Path:    fullName,
				LastMod: file.ModTime().Unix(),
				Size:    file.Size(),
			})
		}
	}

	return fileList
}

func isDirExist(dir string) bool {
	fi, err := os.Stat(dir)

	if err != nil {
		return os.IsExist(err)
	}

	return fi.IsDir()
}
