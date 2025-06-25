package provider

import (
	"bytes"
	"context"
	"log"
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

func (w *Website) UploadFile(location string, buff []byte) (string, error) {
	//log.Println("存储到", w.PluginStorage.StorageType)
	location = strings.TrimLeft(location, "/")
	// 额外存储一份到本地
	if w.PluginStorage.KeepLocal && w.PluginStorage.StorageType != config.StorageTypeLocal {
		//将文件写入本地
		localStorage, _ := storage.NewLocalStorage(w.PluginStorage, w.PublicPath)
		err := localStorage.Put(context.Background(), location, bytes.NewReader(buff))
		if err != nil {
			log.Println(err.Error())
			//无法创建
			return "", err
		}
	}
	err := w.Storage.Put(context.Background(), location, bytes.NewReader(buff))
	log.Println("上传结果", err, location)
	if err != nil {
		return "", err
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
