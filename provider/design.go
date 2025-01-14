package provider

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm/clause"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (w *Website) GetDesignList() []response.DesignPackage {
	// 读取目录
	designPath := w.RootPath + "template"
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
				data, err := os.ReadFile(configFile)
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
				_ = w.writeDesignInfo(&designInfo)
			}

			if designInfo.Package == w.System.TemplateName {
				designInfo.Status = 1
			} else {
				designInfo.Status = 0
			}
			dataPath := designPath + "/" + designInfo.Package + "/data.db"
			_, err = os.Stat(dataPath)
			if err == nil {
				designInfo.PreviewData = true
			} else {
				designInfo.PreviewData = false
			}

			designLists = append(designLists, designInfo)
		}
	}

	return designLists
}

func (w *Website) SaveDesignInfo(req request.DesignInfoRequest) error {
	designList := w.GetDesignList()
	var designIndex = -1
	for i := range designList {
		if designList[i].Package == req.Package {
			designIndex = i
			break
		}
	}
	if designIndex == -1 {
		return errors.New(w.Tr("TemplateDoesNotExist"))
	}

	designInfo := designList[designIndex]

	designInfo.Name = req.Name
	designInfo.TemplateType = req.TemplateType
	//designInfo.Description = req.Description
	//designInfo.Version = req.Version
	//designInfo.Author = req.Author
	//designInfo.Homepage = req.Homepage
	//designInfo.Created = req.Created

	err := w.writeDesignInfo(&designInfo)

	return err
}

// DeleteDesignInfo 删除的模板，会被移动到 cache
func (w *Website) DeleteDesignInfo(packageName string) error {
	packageName = filepath.Base(packageName)
	if packageName == "" {
		return errors.New(w.Tr("TemplateDoesNotExist"))
	}
	if packageName == "default" {
		return errors.New(w.Tr("DefaultTemplateCannotBeDeleted"))
	}

	basePath := w.RootPath + "template/" + packageName
	if !strings.HasPrefix(basePath, w.RootPath+"template/") {
		return errors.New(w.Tr("TemplateDoesNotExist"))
	}

	cachePath := w.CachePath + ".history/" + packageName
	os.MkdirAll(cachePath, os.ModePerm)

	os.RemoveAll(cachePath + "/template")
	os.Rename(basePath, cachePath+"/template")
	// 读取静态文件
	staticPath := w.PublicPath + "static/" + packageName
	os.RemoveAll(cachePath + "/static")
	os.MkdirAll(cachePath, os.ModePerm)
	os.Rename(staticPath, cachePath+"/static")

	return nil
}

func (w *Website) GetDesignInfo(packageName string, scan bool) (*response.DesignPackage, error) {
	designList := w.GetDesignList()
	var designIndex = -1
	for i := range designList {
		if designList[i].Package == packageName {
			designIndex = i
			break
		}
	}
	if designIndex == -1 {
		return nil, errors.New(w.Tr("TemplateDoesNotExist"))
	}

	if !scan {
		return &designList[designIndex], nil
	}

	basePath := w.RootPath + "template/" + packageName
	var hasChange = false

	designInfo := designList[designIndex]
	// 尝试读取模板文件
	files := readAllFiles(basePath)
	if len(files) < len(designInfo.TplFiles) {
		hasChange = true
	}
	for i := range files {
		if strings.HasSuffix(files[i].Path, "config.json") || strings.HasSuffix(files[i].Path, "data.db") {
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
				Path:    fullPath,
				Remark:  "",
				Size:    files[i].Size,
				LastMod: files[i].LastMod,
			})
			hasChange = true
		}
	}
	// 读取静态文件
	staticPath := w.PublicPath + "static/" + packageName
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
				Path:    fullPath,
				Remark:  "",
				Size:    files[i].Size,
				LastMod: files[i].LastMod,
			})
			hasChange = true
		}
	}
	// 对内容进行排序
	sort.Slice(designInfo.TplFiles, func(i, j int) bool {
		first, second := designInfo.TplFiles[i], designInfo.TplFiles[j]
		return first.Path < second.Path
	})
	// 对内容进行排序
	sort.Slice(designInfo.StaticFiles, func(i, j int) bool {
		first, second := designInfo.StaticFiles[i], designInfo.StaticFiles[j]
		return first.Path < second.Path
	})

	if hasChange {
		saveFile := designInfo
		_ = w.writeDesignInfo(&saveFile)
	}

	return &designInfo, nil
}

func (w *Website) GetDesignTemplateFiles(packageName string) ([]response.DesignFile, error) {

	basePath := w.RootPath + "template/" + packageName
	// 尝试读取模板文件
	files := readAllFiles(basePath)
	var templates = make([]response.DesignFile, 0, len(files))
	for i := range files {
		if strings.HasSuffix(files[i].Path, "config.json") || strings.HasSuffix(files[i].Path, "data.db") {
			continue
		}
		files[i].Path = strings.TrimPrefix(files[i].Path, basePath+"/")
		templates = append(templates, files[i])
	}

	return templates, nil
}

func (w *Website) UploadDesignZip(file io.ReaderAt, info *multipart.FileHeader, cover string) error {
	// 解压
	zipReader, err := zip.NewReader(file, info.Size)
	if err != nil {
		return err
	}

	packageName := strings.TrimSuffix(info.Filename, path.Ext(info.Filename))
	// 先尝试读取config.json
	tmpFile, err := zipReader.Open("template/config.json")
	var designInfo response.DesignPackage
	if err == nil {
		data, err := io.ReadAll(tmpFile)
		if err == nil {
			err = json.Unmarshal(data, &designInfo)
			if err == nil {
				packageName = designInfo.Package
			}
		}
	}
	// 检查是否已经存在
	packagePath := w.RootPath + "template/" + packageName
	_, err = os.Stat(packagePath)
	if err == nil {
		// 已存在
		if cover == "cover" {
			// 覆盖
		} else {
			// 新名称
			i := 1
			for {
				packagePath = fmt.Sprintf("%stemplate/%s%d", w.RootPath, packageName, i)
				_, err = os.Stat(packagePath)
				if err != nil {
					packageName = fmt.Sprintf("%s%d", packageName, i)
					break
				}
				i++
			}
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
		f.Name = strings.ReplaceAll(f.Name, "\\", "/")
		var realName string
		// 模板文件
		if strings.HasPrefix(f.Name, "template/") {
			if fileExt != ".html" && fileExt != ".json" && fileExt != ".db" {
				continue
			}
			realName = w.RootPath + "template/" + packageName + "/" + strings.TrimPrefix(f.Name, "template/")
		}
		// static
		if strings.HasPrefix(f.Name, "static/") {
			realName = w.PublicPath + "static/" + packageName + "/" + strings.TrimPrefix(f.Name, "static/")
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
	designInfo.Package = packageName
	designInfo.Created = time.Now().Format("2006-01-02 15:04:05")
	_ = w.writeDesignInfo(&designInfo)
	// 仅需上传static部分
	// 上传到静态服务器
	_ = w.ReadAndSendLocalFiles(w.PublicPath + "static/" + packageName)

	return nil
}

func (w *Website) CreateDesignZip(packageName string) (*bytes.Buffer, error) {
	buff := &bytes.Buffer{}

	archive := zip.NewWriter(buff)
	defer archive.Close()

	// 读取模板
	basePath := w.RootPath + "template/" + packageName
	// 尝试读取模板文件
	files, err := os.ReadDir(basePath)
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
	staticPath := w.PublicPath + "static/" + packageName
	files, err = os.ReadDir(staticPath)
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
		header.Method = zip.Deflate
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

func (w *Website) UploadDesignFile(file multipart.File, info *multipart.FileHeader, packageName, fileType, filePath string) error {
	fileExt := filepath.Ext(info.Filename)
	if fileExt == ".php" {
		return errors.New(w.Tr("CannotUploadPhpFile"))
	}

	designInfo, err := w.GetDesignInfo(packageName, false)
	if err != nil {
		return err
	}

	filePath = strings.ReplaceAll(strings.ReplaceAll(filePath, "..", ""), "\\", "/")
	var realPath string
	if fileType == "static" {
		realPath = w.PublicPath + "static/" + designInfo.Package + "/" + filePath
	} else {
		if fileExt != ".html" && fileExt != ".zip" {
			return errors.New(w.Tr("PleaseUploadTheHtmlTemplate"))
		}
		realPath = w.RootPath + "template/" + designInfo.Package + "/" + filePath
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
			f.Name = strings.ReplaceAll(f.Name, "\\", "/")
			realFile := realPath + "/" + f.Name

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
		if fileType == "static" {
			// 上传到静态服务器
			_ = w.ReadAndSendLocalFiles(realPath)
		}
	} else {
		info.Filename = strings.ReplaceAll(strings.ReplaceAll(info.Filename, "..", ""), "\\", "/")
		realFile := realPath + "/" + info.Filename
		// 单独文件处理
		_ = os.MkdirAll(filepath.Dir(realFile), os.ModePerm)
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
		if fileType == "static" {
			remotePath := strings.TrimSuffix(realFile, w.PublicPath)
			// 上传到静态服务器
			_ = w.SyncHtmlCacheToStorage(realFile, remotePath)
		}
	}

	return nil
}

func (w *Website) GetDesignFileDetail(packageName, filePath, fileType string, scan bool) (*response.DesignFile, error) {
	designInfo, err := w.GetDesignInfo(packageName, false)
	if err != nil {
		return nil, errors.New(w.Tr("TemplateDoesNotExist"))
	}

	filePath = strings.ReplaceAll(strings.ReplaceAll(filePath, "..", ""), "\\", "/")
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

	var realPath string
	if fileType == "static" {
		// 资源
		realPath = w.PublicPath + "static/" + designInfo.Package + "/" + filePath
	} else {
		realPath = w.RootPath + "template/" + designInfo.Package + "/" + filePath
	}

	_, err = os.Stat(realPath)
	if err != nil && os.IsNotExist(err) {
		return nil, errors.New(w.Tr("FileDoesNotExist"))
	}

	if !exists {
		designFileDetail = response.DesignFile{
			Path: filePath,
		}
	}

	if !scan {
		return &designFileDetail, nil
	}

	if fileType == "static" {
		return w.GetDesignStaticFileDetail(packageName, designFileDetail)
	}

	return w.GetDesignTplFileDetail(packageName, designFileDetail)
}

func (w *Website) GetDesignTplFileDetail(packageName string, designFileDetail response.DesignFile) (*response.DesignFile, error) {

	fullPath := w.RootPath + "template/" + packageName + "/" + designFileDetail.Path
	info, err := os.Stat(fullPath)
	if err != nil {
		return &designFileDetail, nil
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return &designFileDetail, nil
	}

	designFileDetail.LastMod = info.ModTime().Unix()
	designFileDetail.Content = string(data)

	return &designFileDetail, nil
}

func (w *Website) GetDesignStaticFileDetail(packageName string, designFileDetail response.DesignFile) (*response.DesignFile, error) {

	fullPath := w.PublicPath + "static/" + packageName + "/" + designFileDetail.Path
	info, err := os.Stat(fullPath)
	if err != nil {
		return &designFileDetail, nil
	}

	data, err := os.ReadFile(fullPath)

	designFileDetail.LastMod = info.ModTime().Unix()
	designFileDetail.Content = string(data)

	return &designFileDetail, nil
}

func (w *Website) GetDesignFileHistories(packageName, filePath, fileType string) []response.DesignFileHistory {
	designFileDetail, err := w.GetDesignFileDetail(packageName, filePath, fileType, false)
	if err != nil {
		return nil
	}

	// 读取 .history
	pathMd5 := library.Md5(designFileDetail.Path)
	historyPath := w.CachePath + ".history/" + packageName + "/" + pathMd5
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
			Size:    files[i].Size,
		})
	}
	// 按时间排序
	sort.Slice(histories, func(i, j int) bool {
		return histories[i].LastMod > histories[j].LastMod
	})

	return histories
}

func (w *Website) StoreDesignHistory(packageName string, filePath string, content []byte) error {
	pathMd5 := library.Md5(filePath)
	historyPath := w.CachePath + ".history/" + packageName + "/" + pathMd5
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
	err = os.WriteFile(historyPath+"/"+historyHash, content, os.ModePerm)

	return err
}

func (w *Website) DeleteDesignHistoryFile(packageName, filePath, historyHash, fileType string) error {
	designFileDetail, err := w.GetDesignFileDetail(packageName, filePath, fileType, false)
	if err != nil {
		return err
	}

	histories := w.GetDesignFileHistories(packageName, filePath, fileType)
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
	historyPath := w.CachePath + ".history/" + packageName + "/" + pathMd5 + "/" + historyHash

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

func (w *Website) GetDesignFileHistoryInfo(packageName, filePath, historyHash, fileType string) (string, error) {
	designFileDetail, err := w.GetDesignFileDetail(packageName, filePath, fileType, false)
	if err != nil {
		return "", err
	}

	histories := w.GetDesignFileHistories(packageName, filePath, fileType)
	var exists = false
	for i := range histories {
		if histories[i].Hash == historyHash {
			exists = true
		}
	}
	if !exists {
		return "", errors.New(w.Tr("NoHistoryFound"))
	}

	pathMd5 := library.Md5(designFileDetail.Path)
	historyPath := w.CachePath + ".history/" + packageName + "/" + pathMd5 + "/" + historyHash

	buf, err := os.ReadFile(historyPath)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

func (w *Website) RestoreDesignFile(packageName, filePath, historyHash, fileType string) error {
	designFileDetail, err := w.GetDesignFileDetail(packageName, filePath, fileType, false)
	if err != nil {
		return err
	}

	histories := w.GetDesignFileHistories(packageName, filePath, fileType)
	var exists = false
	for i := range histories {
		if histories[i].Hash == historyHash {
			exists = true
		}
	}
	if !exists {
		return errors.New(w.Tr("NoHistoryFound"))
	}

	pathMd5 := library.Md5(designFileDetail.Path)
	historyPath := w.CachePath + ".history/" + packageName + "/" + pathMd5 + "/" + historyHash

	var fullPath string
	// 保存html模板
	if fileType == "static" {
		// 保存模板静态文件
		fullPath = w.PublicPath + "static/" + packageName + "/" + designFileDetail.Path
	} else {
		fullPath = w.RootPath + "template/" + packageName + "/" + designFileDetail.Path
	}

	_, err = os.Stat(historyPath)
	if err != nil {
		return err
	}

	err = os.Rename(historyPath, fullPath)
	if err != nil {
		return err
	}
	if fileType == "static" {
		remotePath := strings.TrimSuffix(fullPath, w.PublicPath)
		// 上传到静态服务器
		_ = w.SyncHtmlCacheToStorage(fullPath, remotePath)
	}
	return nil
}

func (w *Website) DeleteDesignFile(packageName, filePath, fileType string) error {
	// 先验证文件名是否合法
	designInfo, err := w.GetDesignInfo(packageName, false)
	if err != nil {
		return errors.New(w.Tr("TemplateDoesNotExist"))
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
		basePath = w.PublicPath + "static/" + packageName
	} else {
		basePath = w.RootPath + "template/" + packageName
	}
	fullPath := basePath + "/" + filePath
	_ = os.Remove(fullPath)

	pathMd5 := library.Md5(filePath)
	historyPath := w.CachePath + ".history/" + packageName + "/" + pathMd5
	_, err = os.Stat(historyPath)
	if err == nil {
		_ = os.RemoveAll(historyPath)
	}
	// 更新文件
	err = w.writeDesignInfo(designInfo)

	return nil
}

func (w *Website) SaveDesignFile(req request.SaveDesignFileRequest) error {
	// 先验证文件名是否合法
	designInfo, err := w.GetDesignInfo(req.Package, false)
	if err != nil {
		return errors.New(w.Tr("TemplateDoesNotExist"))
	}

	req.Path = strings.ReplaceAll(strings.ReplaceAll(req.Path, "..", ""), "\\", "/")
	// 先检查文件是否存在
	var basePath string
	if req.Type == "static" {
		basePath = w.PublicPath + "static/" + req.Package + "/"
	} else {
		basePath = w.RootPath + "template/" + req.Package + "/"
	}
	fullPath := basePath + req.Path

	if req.UpdateContent {
		// 修改内容
		if req.Type == "static" {
			return w.SaveDesignStaticFile(req)
		}
		// 保存模板静态文件
		return w.SaveDesignTplFile(req)
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
			req.RenamePath = strings.ReplaceAll(strings.ReplaceAll(req.RenamePath, "..", ""), "\\", "/")
			newPath := basePath + req.RenamePath
			req.Path = strings.TrimPrefix(newPath, basePath)
			// 移动
			_, err = os.Stat(fullPath)
			if err != nil {
				return err
			}
			_ = os.MkdirAll(filepath.Dir(newPath), os.ModePerm)
			err = os.Rename(fullPath, newPath)
			if err != nil {
				return err
			}
			designFileDetail.Path = req.RenamePath
			if req.Type == "static" {
				remotePath := strings.TrimSuffix(newPath, w.PublicPath)
				// 上传到静态服务器
				_ = w.SyncHtmlCacheToStorage(newPath, remotePath)
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
		err = w.writeDesignInfo(designInfo)
		if req.Type == "static" {
			remotePath := strings.TrimSuffix(fullPath, w.PublicPath)
			// 上传到静态服务器
			_ = w.SyncHtmlCacheToStorage(fullPath, remotePath)
		}

		return err
	}
}

func (w *Website) CopyDesignFile(req request.CopyDesignFileRequest) error {
	if req.NewPath == "" && req.NewPath == req.Path {
		return errors.New(w.Tr("TheNewFileNameIsTheSameAsTheCopiedFile"))
	}
	// 先验证文件名是否合法
	designInfo, err := w.GetDesignInfo(req.Package, false)
	if err != nil {
		return errors.New(w.Tr("TemplateDoesNotExist"))
	}
	req.Path = strings.ReplaceAll(strings.ReplaceAll(req.Path, "..", ""), "\\", "/")
	req.NewPath = strings.ReplaceAll(strings.ReplaceAll(req.NewPath, "..", ""), "\\", "/")
	// 先检查文件是否存在
	var basePath string
	if req.Type == "static" {
		basePath = w.PublicPath + "static/" + req.Package + "/"
	} else {
		basePath = w.RootPath + "template/" + req.Package + "/"
	}
	fullPath := basePath + req.Path

	// 修改备注名称等
	var designFileDetail response.DesignFile
	var existsIndex = -1
	if req.Type == "static" {
		for i := range designInfo.StaticFiles {
			if designInfo.StaticFiles[i].Path == req.NewPath {
				designFileDetail = designInfo.StaticFiles[i]
				existsIndex = i
				break
			}
		}
	} else {
		for i := range designInfo.TplFiles {
			if designInfo.TplFiles[i].Path == req.NewPath {
				designFileDetail = designInfo.TplFiles[i]
				existsIndex = i
				break
			}
		}
	}
	if existsIndex != -1 {
		// 文件已存在
		return errors.New(w.Tr("TheNewFileAlreadyExists"))
	}
	// 如果进行了重命名
	newPath := basePath + req.NewPath
	req.Path = strings.TrimPrefix(newPath, basePath)
	// 开始复制
	_ = os.MkdirAll(filepath.Dir(newPath), os.ModePerm)
	oldFile, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}
	err = os.WriteFile(newPath, oldFile, os.ModePerm)
	if err != nil {
		return err
	}
	//
	// 写入文件
	designFileDetail = response.DesignFile{
		Path:    req.NewPath,
		Remark:  req.Remark,
		Content: "",
		LastMod: 0,
	}
	if req.Type != "static" {
		designInfo.TplFiles = append(designInfo.TplFiles, designFileDetail)
	} else {
		designInfo.StaticFiles = append(designInfo.StaticFiles, designFileDetail)
	}
	// 更新文件
	err = w.writeDesignInfo(designInfo)
	if req.Type == "static" {
		remotePath := strings.TrimSuffix(newPath, w.PublicPath)
		// 上传到静态服务器
		_ = w.SyncHtmlCacheToStorage(newPath, remotePath)
	}

	return err
}

func (w *Website) writeDesignInfo(designInfo *response.DesignPackage) error {
	// 更新文件
	basePath := w.RootPath + "template/" + designInfo.Package + "/"
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
	baseStaticPath := w.PublicPath + "static/" + designInfo.Package + "/"
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
		err = os.WriteFile(configFile, buf, os.ModePerm)
	}

	return err
}

func (w *Website) SaveDesignTplFile(req request.SaveDesignFileRequest) error {
	// 不能越级到上级
	req.Path = strings.ReplaceAll(strings.ReplaceAll(req.Path, "..", ""), "\\", "/")
	basePath := w.RootPath + "template/" + req.Package + "/"
	fullPath := basePath + req.Path

	// 尝试创建历史记录
	_, err := os.Stat(fullPath)
	if err == nil {
		// 文件存在，验证内容的md5, 如果一致，就不保存
		oldBytes, _ := os.ReadFile(fullPath)
		oldMd5 := library.Md5Bytes(oldBytes)
		newMd5 := library.Md5(req.Content)

		if oldMd5 == newMd5 {
			if req.Content != "" {
				// MD5 一致, 直接返回
				return nil
			}
		} else {
			// 否则，写入历史记录文件
			_ = w.StoreDesignHistory(req.Package, req.Path, oldBytes)
			// 写入历史失败不报错
		}
	} else {
		// 文件不存在
		filePath := filepath.Dir(fullPath)
		_, err = os.Stat(filePath)
		if os.IsNotExist(err) {
			_ = os.MkdirAll(filePath, os.ModePerm)
		}
	}

	err = os.WriteFile(fullPath, []byte(req.Content), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func (w *Website) SaveDesignStaticFile(req request.SaveDesignFileRequest) error {
	// 不能越级到上级
	req.Path = strings.ReplaceAll(strings.ReplaceAll(req.Path, "..", ""), "\\", "/")
	basePath := w.PublicPath + "static/" + req.Package + "/"
	fullPath := basePath + req.Path

	// 尝试创建历史记录
	_, err := os.Stat(fullPath)
	if err == nil {
		// 文件存在，验证内容的md5, 如果一致，就不保存
		oldBytes, _ := os.ReadFile(fullPath)
		oldMd5 := library.Md5Bytes(oldBytes)
		newMd5 := library.Md5(req.Content)

		if oldMd5 == newMd5 {
			if req.Content != "" {
				// MD5 一致, 直接返回
				return nil
			}
		} else {
			// 否则，写入历史记录文件
			_ = w.StoreDesignHistory(req.Package, req.Path, oldBytes)
			// 写入历史失败不报错
		}
	} else {
		// 文件不存在
		filePath := filepath.Dir(fullPath)
		_, err = os.Stat(filePath)
		if os.IsNotExist(err) {
			_ = os.MkdirAll(filePath, os.ModePerm)
		}
	}

	err = os.WriteFile(fullPath, []byte(req.Content), os.ModePerm)
	if err != nil {
		return err
	}
	// 更新文件
	if req.Type == "static" {
		remotePath := strings.TrimSuffix(fullPath, w.PublicPath)
		// 上传到静态服务器
		_ = w.SyncHtmlCacheToStorage(fullPath, remotePath)
	}
	return nil
}

func (w *Website) RestoreDesignData(packageName string) error {
	dataPath := w.RootPath + "template/" + packageName + "/data.db"
	_, err := os.Stat(dataPath)
	if err != nil {
		return errors.New(w.Tr("NoDataToInitialize"))
	}

	zipReader, err := zip.OpenReader(dataPath)
	if err != nil {
		return errors.New(w.Tr("FailedToReadData"))
	}
	defer zipReader.Close()

	settings, err := zipReader.Open("settings")
	if err == nil {
		w.restoreSingleData("settings", settings)
	}
	modules, err := zipReader.Open("modules")
	if err == nil {
		w.restoreSingleData("modules", modules)
	}
	// 需要先处理 settings 和 modules，再开始处理其他表
	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() || f.Name == "settings" || f.Name == "modules" {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			continue
		}
		w.restoreSingleData(f.Name, reader)
		reader.Close()
	}

	return nil
}

// restoreSingleData
//
//	    	anchors              []model.Anchor
//			anchorData           []model.AnchorData
//			archives             []model.Archive
//			archiveData          []model.ArchiveData
//			attachments          []model.Attachment
//			attachmentCategories []model.AttachmentCategory
//			categories           []model.Category
//			archiveCategories    []model.ArchiveCategory
//			comments             []model.Comment
//			guestbooks           []model.Guestbook
//			keywords             []model.Keyword
//			links                []model.Link
//			materials            []model.Material
//			materialCategories   []model.MaterialCategory
//			materialData         []model.MaterialData
//			modules              []model.Module
//			navs                 []model.Nav
//			navTypes             []model.NavType
//			redirects            []model.Redirect
//			settings             []model.Setting
//			tags                 []model.Tag
//			tagData              []model.TagData
//			userGroups           []model.UserGroup
func (w *Website) restoreSingleData(name string, reader io.ReadCloser) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return
	}
	if name == "settings" {
		var settings []model.Setting
		err = json.Unmarshal(data, &settings)
		if err != nil {
			return
		}
		for _, v := range settings {
			// 处理system,防止域名覆盖
			if v.Key == SystemSettingKey {
				var systemSetting config.SystemConfig
				_ = json.Unmarshal([]byte(v.Value), &systemSetting)
				systemSetting.TemplateName = w.System.TemplateName
				systemSetting.BaseUrl = w.System.BaseUrl
				systemSetting.MobileUrl = w.System.MobileUrl
				systemSetting.AdminUrl = w.System.AdminUrl
				buf, err := json.Marshal(systemSetting)
				if err == nil {
					v.Value = string(buf)
				}
			} else if v.Key == StorageSettingKey || v.Key == ImportApiSettingKey || v.Key == AnqiSettingKey {
				continue
			}
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
		w.InitSetting()
	} else if name == "modules" {
		var modules []model.Module
		err = json.Unmarshal(data, &modules)
		if err != nil {
			return
		}
		for _, v := range modules {
			v.Database = w.Mysql.Database
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
			// 更新模型数据
			tplPath := fmt.Sprintf("%s/%s", w.GetTemplateDir(), v.TableName)
			v.Migrate(w.DB, tplPath, true)
		}
		w.DeleteCacheModules()
	} else if name == "categories" {
		var categories []model.Category
		err = json.Unmarshal(data, &categories)
		if err != nil {
			return
		}
		for _, v := range categories {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
		w.DeleteCacheCategories()
	} else if name == "archives" {
		var archives []model.Archive
		err = json.Unmarshal(data, &archives)
		if err != nil {
			return
		}
		for _, v := range archives {
			// archive 还有 附加表数据
			if v.Extra != nil {
				module, err := w.GetModuleById(v.ModuleId)
				if err == nil {
					extraFields := map[string]interface{}{
						"id": v.Id,
					}
					for ek, ev := range v.Extra {
						extraFields[ek] = ev.Value
					}
					// 先检查是否存在
					var existsId uint
					w.DB.Table(module.TableName).Where("`id` = ?", v.Id).Pluck("id", &existsId)
					if existsId > 0 {
						// 已存在
						w.DB.Table(module.TableName).Where("`id` = ?", v.Id).Updates(extraFields)
					} else {
						// 新建
						extraFields["id"] = v.Id
						w.DB.Table(module.TableName).Where("`id` = ?", v.Id).Create(extraFields)
					}
				}
			}
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
		w.DeleteCacheFixedLinks()
	} else if name == "archiveData" {
		var archiveData []model.ArchiveData
		err = json.Unmarshal(data, &archiveData)
		if err != nil {
			return
		}
		for _, v := range archiveData {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "archiveCategories" {
		var archiveCategories []model.ArchiveCategory
		err = json.Unmarshal(data, &archiveCategories)
		if err != nil {
			return
		}
		for _, v := range archiveCategories {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "tags" {
		var tags []model.Tag
		err = json.Unmarshal(data, &tags)
		if err != nil {
			return
		}
		for _, v := range tags {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "tagData" {
		var tagData []model.TagData
		err = json.Unmarshal(data, &tagData)
		if err != nil {
			return
		}
		for _, v := range tagData {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "anchors" {
		var anchors []model.Anchor
		err = json.Unmarshal(data, &anchors)
		if err != nil {
			return
		}
		for _, v := range anchors {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "anchorData" {
		var anchorData []model.AnchorData
		err = json.Unmarshal(data, &anchorData)
		if err != nil {
			return
		}
		for _, v := range anchorData {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "attachments" {
		var attachments []model.Attachment
		err = json.Unmarshal(data, &attachments)
		if err != nil {
			return
		}
		for _, v := range attachments {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "attachmentCategories" {
		var attachmentCategories []model.AttachmentCategory
		err = json.Unmarshal(data, &attachmentCategories)
		if err != nil {
			return
		}
		for _, v := range attachmentCategories {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "comments" {
		var comments []model.Comment
		err = json.Unmarshal(data, &comments)
		if err != nil {
			return
		}
		for _, v := range comments {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "guestbooks" {
		var guestbooks []model.Guestbook
		err = json.Unmarshal(data, &guestbooks)
		if err != nil {
			return
		}
		for _, v := range guestbooks {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "keywords" {
		var keywords []model.Keyword
		err = json.Unmarshal(data, &keywords)
		if err != nil {
			return
		}
		for _, v := range keywords {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "links" {
		var links []model.Link
		err = json.Unmarshal(data, &links)
		if err != nil {
			return
		}
		for _, v := range links {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "materials" {
		var materials []model.Material
		err = json.Unmarshal(data, &materials)
		if err != nil {
			return
		}
		for _, v := range materials {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "materialCategories" {
		var materialCategories []model.MaterialCategory
		err = json.Unmarshal(data, &materialCategories)
		if err != nil {
			return
		}
		for _, v := range materialCategories {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "materialData" {
		var materialData []model.MaterialData
		err = json.Unmarshal(data, &materialData)
		if err != nil {
			return
		}
		for _, v := range materialData {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "navTypes" {
		var navTypes []model.NavType
		err = json.Unmarshal(data, &navTypes)
		if err != nil {
			return
		}
		for _, v := range navTypes {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else if name == "navs" {
		var navs []model.Nav
		err = json.Unmarshal(data, &navs)
		if err != nil {
			return
		}
		for _, v := range navs {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
		w.DeleteCacheNavs()
	} else if name == "redirects" {
		var redirects []model.Redirect
		err = json.Unmarshal(data, &redirects)
		if err != nil {
			return
		}
		for _, v := range redirects {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
		w.DeleteCacheRedirects()
	} else if name == "userGroups" {
		var userGroups []model.UserGroup
		err = json.Unmarshal(data, &userGroups)
		if err != nil {
			return
		}
		for _, v := range userGroups {
			w.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v)
		}
	} else {
		name = strings.ReplaceAll(name, "\\", "/")
		realFile := w.PublicPath + name

		_ = os.MkdirAll(filepath.Dir(realFile), os.ModePerm)
		os.WriteFile(realFile, data, os.ModePerm)
	}
}

func (w *Website) BackupDesignData(packageName string) error {
	dataPath := w.RootPath + "template/" + packageName + "/data.db"
	zipFile, err := os.Create(dataPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()
	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	// 每项数据不超过200条
	var maxLimit = 200

	// 开始逐个写入数据
	var anchors []model.Anchor
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&anchors)
	if len(anchors) > 0 {
		_ = w.writeDataToZip("anchors", anchors, zw)
		var anchorIds = make([]uint, 0, len(anchors))
		for i := range anchors {
			anchorIds = append(anchorIds, anchors[i].Id)
		}
		var anchorData []model.AnchorData
		w.DB.Where("`anchor_id` IN(?)", anchorIds).Find(&anchorData)
		if len(anchorData) > 0 {
			_ = w.writeDataToZip("anchorData", anchorData, zw)
		}
	}
	var archives []model.Archive
	w.DB.Order("`id` desc").Limit(maxLimit).Find(&archives)
	if len(archives) > 0 {
		var archiveIds = make([]int64, 0, len(archives))
		for i := range archives {
			archiveIds = append(archiveIds, archives[i].Id)
			archives[i].Extra = w.GetArchiveExtra(archives[i].ModuleId, archives[i].Id, false)
		}
		_ = w.writeDataToZip("archives", archives, zw)
		var archiveData []model.ArchiveData
		w.DB.Where("`id` IN(?)", archiveIds).Find(&archiveData)
		if len(archiveData) > 0 {
			_ = w.writeDataToZip("archiveData", archiveData, zw)
		}
		// 文档关联分类
		var archiveCategories []model.ArchiveCategory
		w.DB.Where("`archive_id` IN(?)", archiveIds).Find(&archiveCategories)
		if len(archiveCategories) > 0 {
			_ = w.writeDataToZip("archiveCategories", archiveCategories, zw)
		}
	}
	var attachments []model.Attachment
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&attachments)
	if len(attachments) > 0 {
		_ = w.writeDataToZip("attachments", attachments, zw)
		for i := range attachments {
			// read file from local, real file and thumb file
			fullPath := w.PublicPath + attachments[i].FileLocation
			_ = w.writeFileToZip(attachments[i].FileLocation, fullPath, zw)
			// thumb file
			thumbName := filepath.Dir(attachments[i].FileLocation) + "/thumb_" + filepath.Base(attachments[i].FileLocation)
			thumbPath := w.PublicPath + thumbName
			_ = w.writeFileToZip(thumbName, thumbPath, zw)
		}
	}
	var attachmentCategories []model.AttachmentCategory
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&attachmentCategories)
	if len(attachmentCategories) > 0 {
		_ = w.writeDataToZip("attachmentCategories", attachmentCategories, zw)
	}
	var categories []model.Category
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&categories)
	if len(categories) > 0 {
		_ = w.writeDataToZip("categories", categories, zw)
	}
	var comments []model.Comment
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&comments)
	if len(comments) > 0 {
		_ = w.writeDataToZip("comments", comments, zw)
	}
	var guestbooks []model.Guestbook
	w.DB.Order("`id` desc").Limit(maxLimit).Find(&guestbooks)
	if len(guestbooks) > 0 {
		_ = w.writeDataToZip("guestbooks", guestbooks, zw)
	}
	var keywords []model.Keyword
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&keywords)
	if len(keywords) > 0 {
		_ = w.writeDataToZip("keywords", keywords, zw)
	}
	var links []model.Link
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&links)
	if len(links) > 0 {
		_ = w.writeDataToZip("links", links, zw)
	}
	var materials []model.Material
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&materials)
	if len(materials) > 0 {
		_ = w.writeDataToZip("materials", materials, zw)
		var materialIds = make([]uint, 0, len(materials))
		for i := range materials {
			materialIds = append(materialIds, materials[i].Id)
		}
		var materialData []model.MaterialData
		w.DB.Where("`material_id` IN(?)", materialIds).Find(&materialData)
		if len(materialData) > 0 {
			_ = w.writeDataToZip("materialData", materialData, zw)
		}
	}
	var materialCategories []model.MaterialCategory
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&materialCategories)
	if len(materialCategories) > 0 {
		_ = w.writeDataToZip("materialCategories", materialCategories, zw)
	}
	var modules []model.Module
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&modules)
	if len(modules) > 0 {
		_ = w.writeDataToZip("modules", modules, zw)
	}
	var navs []model.Nav
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&navs)
	if len(navs) > 0 {
		_ = w.writeDataToZip("navs", navs, zw)
	}
	var navTypes []model.NavType
	w.DB.Order("`id` desc").Limit(maxLimit).Find(&navTypes)
	if len(navTypes) > 0 {
		_ = w.writeDataToZip("navTypes", navTypes, zw)
	}
	var redirects []model.Redirect
	w.DB.Order("`id` desc").Limit(maxLimit).Find(&redirects)
	if len(redirects) > 0 {
		_ = w.writeDataToZip("redirects", redirects, zw)
	}
	var settings []model.Setting
	w.DB.Where("`key` NOT IN(?)", []string{SendmailSettingKey, ImportApiSettingKey, StorageSettingKey, PaySettingKey, WeappSettingKey, WechatSettingKey, AnqiSettingKey}).Find(&settings)
	if len(settings) > 0 {
		_ = w.writeDataToZip("settings", settings, zw)
	}
	var tags []model.Tag
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&tags)
	if len(tags) > 0 {
		_ = w.writeDataToZip("tags", tags, zw)
		var tagIds = make([]uint, 0, len(tags))
		for i := range tags {
			tagIds = append(tagIds, tags[i].Id)
		}
		var tagData []model.TagData
		w.DB.Where("`tag_id` IN(?)", tagIds).Find(&tagData)
		if len(tagData) > 0 {
			_ = w.writeDataToZip("tagData", tagData, zw)
		}
	}
	var userGroups []model.UserGroup
	w.DB.Where("`status` = 1").Order("`id` desc").Limit(maxLimit).Find(&userGroups)
	if len(userGroups) > 0 {
		_ = w.writeDataToZip("userGroups", userGroups, zw)
	}
	return nil
}

func (w *Website) writeFileToZip(name string, filePath string, zw *zip.Writer) error {
	fullName := filePath
	file, err := os.Open(fullName)
	if err != nil {
		return err
	}
	defer file.Close()
	info, _ := file.Stat()
	header, err := zip.FileInfoHeader(info)
	header.Name = name
	header.Method = zip.Deflate
	if err != nil {
		return err
	}
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, _ = io.Copy(writer, file)
	_ = file.Close()

	return nil
}

func (w *Website) writeDataToZip(name string, data interface{}, zw *zip.Writer) error {
	buf, err := json.Marshal(data)
	if err != nil {
		return err
	}
	// 全局替换 域名
	buf = bytes.ReplaceAll(buf, []byte(w.System.BaseUrl), []byte{})
	if w.PluginStorage.StorageUrl != w.System.BaseUrl {
		buf = bytes.ReplaceAll(buf, []byte(w.PluginStorage.StorageUrl), []byte{})
	}
	size := len(buf)
	header := &zip.FileHeader{
		Name:               name,
		UncompressedSize64: uint64(size),
		Method:             zip.Deflate,
	}
	header.Modified = time.Now()
	header.SetMode(os.ModePerm)

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(buf)
	if err != nil {
		return err
	}

	return nil
}

func readAllFiles(dir string) []response.DesignFile {
	if !isDirExist(dir) {
		return []response.DesignFile{}
	}

	files, _ := os.ReadDir(dir)
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
			info, err := file.Info()
			if err != nil {
				continue
			}
			fileList = append(fileList, response.DesignFile{
				Path:    fullName,
				LastMod: info.ModTime().Unix(),
				Size:    info.Size(),
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
