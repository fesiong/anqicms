package provider

import (
	"context"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider/storage"
)

func (w *Website) GetBucket(cfg *config.PluginStorageConfig) (storage.Storage, error) {
	switch cfg.StorageType {
	case config.StorageTypeAliyun:
		return storage.NewAliyunStorage(cfg)
	case config.StorageTypeTencent:
		return storage.NewTencentStorage(cfg)
	case config.StorageTypeQiniu:
		return storage.NewQiniuStorage(cfg)
	case config.StorageTypeUpyun:
		return storage.NewUpyunStorage(cfg)
	case config.StorageTypeGoogle:
		return storage.NewGoogleStorage(cfg)
	case config.StorageTypeAws:
		return storage.NewAwsStorage(cfg)
	case config.StorageTypeR2:
		return storage.NewR2Storage(cfg)
	case config.StorageTypeFTP:
		return storage.NewFtpStorage(cfg)
	case config.StorageTypeSSH:
		return storage.NewSSHStorage(cfg, w.DataPath)
	default:
		// localStorage
		cfg.StorageType = config.StorageTypeLocal
		return storage.NewLocalStorage(cfg, w.PublicPath)
	}
}

func (w *Website) UploadFile(location string, r io.Reader) (string, error) {
	//log.Println("存储到", w.PluginStorage.StorageType)
	location = strings.TrimLeft(location, "/")

	err := w.Storage.Put(context.Background(), location, r)
	log.Println("上传结果", err, location)
	if err != nil {
		return "", err
	}
	// todo
	if w.PluginStorage.KeepLocal && w.PluginStorage.StorageType != config.StorageTypeLocal || 1 == 1 {
		var uploadReader io.Reader
		//将文件写入本地
		if seeker, ok := r.(io.Seeker); ok {
			log.Println("支持 io.Seeker")
			// 如果已经是Seeker，重置到开头
			seeker.Seek(0, io.SeekStart)
			uploadReader = r
		} else if file, ok := r.(*os.File); ok {
			log.Println("支持 os.File， 重新打开")
			file.Seek(0, io.SeekStart)
			uploadReader = file
		} else if file, ok := r.(multipart.File); ok {
			log.Println("支持 multipart.File")
			uploadReader = file
		} else {
			log.Println("无法识别的Reader")
			uploadReader = nil
		}
		if uploadReader != nil {
			localStorage, _ := storage.NewLocalStorage(w.PluginStorage, w.PublicPath)
			err = localStorage.Put(context.Background(), location, uploadReader)
		}
	}
	//
	// 上传到静态服务器
	_ = w.SyncHtmlCacheToStorage(w.PublicPath+location, location)

	return location, nil
}

func (w *Website) DeleteFile(location string) error {
	//log.Println("存储到", w.PluginStorage.StorageType)
	location = strings.TrimLeft(location, "/")
	// 额外存储一份到本地
	if w.PluginStorage.KeepLocal && w.PluginStorage.StorageType != config.StorageTypeLocal {
		//将文件写入本地
		localStorage, _ := storage.NewLocalStorage(w.PluginStorage, w.PublicPath)
		err := localStorage.Delete(context.Background(), location)
		if err != nil {
			log.Println(err.Error())
			//无法创建
			return err
		}
	}
	err := w.Storage.Delete(context.Background(), location)
	if err != nil {
		return err
	}

	return nil
}

func (w *Website) MoveFile(src, dest string) error {
	//log.Println("存储到", w.PluginStorage.StorageType)
	src = strings.TrimLeft(src, "/")
	dest = strings.TrimLeft(dest, "/")
	// 额外存储一份到本地
	if w.PluginStorage.KeepLocal && w.PluginStorage.StorageType != config.StorageTypeLocal {
		//将文件写入本地
		localStorage, _ := storage.NewLocalStorage(w.PluginStorage, w.PublicPath)
		err := localStorage.Move(context.Background(), src, dest)
		if err != nil {
			log.Println(err.Error())
			return err
		}
	}

	// 移动文件
	err := w.Storage.Move(context.Background(), src, dest)
	if err != nil {
		return err
	}

	// 移动 thumb
	paths, fileName := filepath.Split(src)
	srcThumb := paths + "thumb_" + fileName
	paths, fileName = filepath.Split(dest)
	destThumb := paths + "thumb_" + fileName
	_ = w.Storage.Move(context.Background(), srcThumb, destThumb)

	return nil
}

func (w *Website) InitBucket() {
	s, err := w.GetBucket(w.PluginStorage)
	if err != nil {
		// 退回到local
		log.Println(err.Error())
		w.PluginStorage.StorageType = config.StorageTypeLocal
		s, _ = storage.NewLocalStorage(w.PluginStorage, w.PublicPath)
	}
	w.Storage = s
}
