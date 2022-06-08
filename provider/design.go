package provider

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"os"
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
				bytes, err := ioutil.ReadFile(configFile)
				if err != nil {
					// 无法读取，只能跳过
					continue
				}
				err = json.Unmarshal(bytes, &designInfo)

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
				// 更新文件
				buf, err := json.MarshalIndent(designInfo, "", "\t")
				if err != nil {
					// 解析失败
					continue
				}

				err = ioutil.WriteFile(configFile, buf, os.ModePerm)
				if err != nil {
					// 写入失败
					//	continue
				}
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

	// 更新文件
	basePath := config.ExecPath + "template/" + req.Package
	configFile := basePath + "/" + "config.json"
	buf, err := json.MarshalIndent(designInfo, "", "\t")
	if err == nil {
		// 解析失败
		err = ioutil.WriteFile(configFile, buf, os.ModePerm)
		if err != nil {
			// 写入失败
			//	continue
		}
	}

	return nil
}

// DeleteDesignInfo 删除的模板，会被移动到 cache
func DeleteDesignInfo(packageName string) error {
	designList := GetDesignList()
	var designIndex = -1
	for i := range designList {
		if designList[i].Package == packageName {
			designIndex = i
			break
		}
	}
	if designIndex == -1 {
		return errors.New("模板不存在")
	}

	if packageName == "default" {
		return errors.New("默认模板不能删除")
	}

	basePath := config.ExecPath + "template/" + packageName
	cachePath := config.ExecPath + "cache/" + ".history/" + packageName + "/.template"
	os.RemoveAll(cachePath)
	os.Rename(basePath, cachePath)
	// 读取静态文件
	staticPath := config.ExecPath + "public/static/" + packageName
	cachePath = config.ExecPath + "cache/" + ".history/" + packageName + "/.static"
	os.RemoveAll(cachePath)
	os.Rename(staticPath, cachePath)

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
	configFile := basePath + "/" + "config.json"

	designInfo := designList[designIndex]
	// 尝试读取模板文件
	files := readAllFiles(basePath)
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
		// 更新文件
		buf, err := json.MarshalIndent(designInfo, "", "\t")
		if err == nil {
			// 解析失败
			err = ioutil.WriteFile(configFile, buf, os.ModePerm)
			if err != nil {
				// 写入失败
				//	continue
			}
		}
	}

	return &designInfo, nil
}

func GetDesignFileDetail(packageName string, filePath string, scan bool) (*response.DesignFile, error) {
	designInfo, err := GetDesignInfo(packageName, false)
	if err != nil {
		return nil, errors.New("模板不存在")
	}

	var designFileDetail response.DesignFile
	var exists = false
	var isTpl = false
	if filePath == "" && len(designInfo.TplFiles) > 0 {
		filePath = designInfo.TplFiles[0].Path
	}
	if strings.HasSuffix(filePath, ".html") {
		isTpl = true
		for i := range designInfo.TplFiles {
			if designInfo.TplFiles[i].Path == filePath {
				designFileDetail = designInfo.TplFiles[i]
				exists = true
				break
			}
		}
	} else {
		// 保存模板静态文件
		for i := range designInfo.StaticFiles {
			if designInfo.StaticFiles[i].Path == filePath {
				designFileDetail = designInfo.StaticFiles[i]
				exists = true
				break
			}
		}
	}

	if !exists {
		return nil, errors.New("文件不存在")
	}

	if !scan {
		return &designFileDetail, nil
	}

	if isTpl {
		return GetDesignTplFileDetail(packageName, designFileDetail)
	}

	return GetDesignStaticFileDetail(packageName, designFileDetail)
}

func GetDesignTplFileDetail(packageName string, designFileDetail response.DesignFile) (*response.DesignFile, error) {

	fullPath := config.ExecPath + "template/" + packageName + "/" + designFileDetail.Path
	info, err := os.Stat(fullPath)
	if err != nil {
		return &designFileDetail, nil
		//return nil, errors.New("模板文件读取失败")
	}

	bytes, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return &designFileDetail, nil
		//return nil, errors.New("模板文件读取失败")
	}

	designFileDetail.LastMod = info.ModTime().Unix()
	designFileDetail.Content = string(bytes)

	return &designFileDetail, nil
}

func GetDesignStaticFileDetail(packageName string, designFileDetail response.DesignFile) (*response.DesignFile, error) {

	fullPath := config.ExecPath + "public/static/" + packageName + "/" + designFileDetail.Path
	info, err := os.Stat(fullPath)
	if err != nil {
		return &designFileDetail, nil
		//return nil, errors.New("模板文件读取失败")
	}

	bytes, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return &designFileDetail, nil
		//return nil, errors.New("模板文件读取失败")
	}

	designFileDetail.LastMod = info.ModTime().Unix()
	designFileDetail.Content = string(bytes)

	return &designFileDetail, nil
}

func GetDesignFileHistories(packageName string, filePath string) []response.DesignFileHistory {
	designFileDetail, err := GetDesignFileDetail(packageName, filePath, false)
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

func DeleteDesignHistoryFile(packageName string, filePath string, historyHash string) error {
	designFileDetail, err := GetDesignFileDetail(packageName, filePath, false)
	if err != nil {
		return err
	}

	histories := GetDesignFileHistories(packageName, filePath)
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

func RestoreDesignFile(packageName string, filePath string, historyHash string) error {
	designFileDetail, err := GetDesignFileDetail(packageName, filePath, false)
	if err != nil {
		return err
	}

	histories := GetDesignFileHistories(packageName, filePath)
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
	if strings.HasSuffix(filePath, ".html") {
		fullPath = config.ExecPath + "template/" + packageName + "/" + designFileDetail.Path
	} else {
		// 保存模板静态文件
		fullPath = config.ExecPath + "public/static/" + packageName + "/" + designFileDetail.Path
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

func DeleteDesignFile(packageName string, filePath string) error {
	// 先验证文件名是否合法
	designInfo, err := GetDesignInfo(packageName, false)
	if err != nil {
		return errors.New("模板不存在")
	}

	var designFileDetail response.DesignFile
	var existsIndex = -1
	var isTpl = false

	if strings.HasSuffix(filePath, ".html") {
		isTpl = true
		for i := range designInfo.TplFiles {
			if designInfo.TplFiles[i].Path == filePath {
				designFileDetail = designInfo.TplFiles[i]
				existsIndex = i
				designInfo.TplFiles = append(designInfo.TplFiles[:i], designInfo.TplFiles[i+1:]...)
				break
			}
		}
	} else {
		// 保存模板静态文件
		for i := range designInfo.StaticFiles {
			if designInfo.StaticFiles[i].Path == filePath {
				designFileDetail = designInfo.StaticFiles[i]
				existsIndex = i
				designInfo.StaticFiles = append(designInfo.StaticFiles[:i], designInfo.StaticFiles[i+1:]...)
				break
			}
		}
	}

	// 只记录remark
	if existsIndex == -1 {
		return nil
	} else {
		var fullPath string
		// 保存html模板
		if isTpl {
			fullPath = config.ExecPath + "template/" + packageName + "/" + designFileDetail.Path
		} else {
			// 保存模板静态文件
			fullPath = config.ExecPath + "public/static/" + packageName + "/" + designFileDetail.Path
		}

		os.Remove(fullPath)

		// 更新文件

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
	basePath := config.ExecPath + "template/" + packageName
	configFile := basePath + "/" + "config.json"
	buf, err := json.MarshalIndent(designInfo, "", "\t")
	if err == nil {
		// 解析失败
		err = ioutil.WriteFile(configFile, buf, os.ModePerm)
		if err != nil {
			// 写入失败
			//	continue
		}
	}

	return nil
}

func SaveDesignFile(req request.SaveDesignFileRequest) error {
	// 先验证文件名是否合法
	designInfo, err := GetDesignInfo(req.Package, false)
	if err != nil {
		return errors.New("模板不存在")
	}

	var designFileDetail response.DesignFile
	var existsIndex = -1
	var isTpl = false

	if strings.HasSuffix(req.Path, ".html") {
		isTpl = true
		for i := range designInfo.TplFiles {
			if designInfo.TplFiles[i].Path == req.Path {
				designFileDetail = designInfo.TplFiles[i]
				existsIndex = i
				break
			}
		}

		// 不能越级到上级
		basePath := config.ExecPath + "template/" + req.Package + "/"
		fullPath := filepath.Clean(basePath + req.Path)
		if !strings.HasPrefix(fullPath, basePath) {
			return errors.New("模板文件保存失败")
		}
		req.Path = strings.TrimPrefix(fullPath, basePath)
		if req.RenamePath != "" && req.RenamePath != req.Path {
			newPath := filepath.Clean(basePath + req.RenamePath)
			if !strings.HasPrefix(newPath, basePath) {
				return errors.New("模板文件保存失败")
			}
			req.Path = strings.TrimPrefix(newPath, basePath)
			// 移动
			if existsIndex != -1 {
				err = os.Rename(fullPath, newPath)
				if err != nil {
					return err
				}
				designFileDetail.Path = req.Path
			}
		}

	} else {
		// 保存模板静态文件
		for i := range designInfo.StaticFiles {
			if designInfo.StaticFiles[i].Path == req.Path {
				designFileDetail = designInfo.StaticFiles[i]
				existsIndex = i
				break
			}
		}

		// 不能越级到上级
		basePath := config.ExecPath + "public/static/" + req.Package + "/"
		fullPath := filepath.Clean(basePath + req.Path)
		if !strings.HasPrefix(fullPath, basePath) {
			return errors.New("模板文件保存失败")
		}
		req.Path = strings.TrimPrefix(fullPath, basePath)
		if req.RenamePath != "" && req.RenamePath != req.Path {
			newPath := filepath.Clean(basePath + req.RenamePath)
			if !strings.HasPrefix(newPath, basePath) {
				return errors.New("模板文件保存失败")
			}
			req.Path = strings.TrimPrefix(newPath, basePath)
			// 移动
			if existsIndex != -1 {
				err = os.Rename(fullPath, newPath)
				if err != nil {
					return err
				}
				designFileDetail.Path = req.Path
			}
		}
	}

	// 只记录remark
	if existsIndex == -1 {
		// 写入文件
		designFileDetail = response.DesignFile{
			Path:    req.Path,
			Remark:  req.Remark,
			Content: "",
			LastMod: 0,
		}
		if isTpl {
			designInfo.TplFiles = append(designInfo.TplFiles, designFileDetail)
		} else {
			designInfo.StaticFiles = append(designInfo.StaticFiles, designFileDetail)
		}
	} else {
		designFileDetail.Remark = req.Remark
		if isTpl {
			designInfo.TplFiles[existsIndex] = designFileDetail
		} else {
			designInfo.StaticFiles[existsIndex] = designFileDetail
		}
	}
	// 更新文件
	basePath := config.ExecPath + "template/" + req.Package
	configFile := basePath + "/" + "config.json"
	buf, err := json.MarshalIndent(designInfo, "", "\t")
	if err == nil {
		// 解析失败
		err = ioutil.WriteFile(configFile, buf, os.ModePerm)
		if err != nil {
			// 写入失败
			//	continue
		}
	}

	// todo 更改为struct 操控模式

	if !req.UpdateContent {
		return nil
	}

	if isTpl {
		return SaveDesignTplFile(req)
	}
	// 保存模板静态文件
	return SaveDesignStaticFile(req)
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
