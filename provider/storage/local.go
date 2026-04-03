package storage

import (
	"bytes"
	"context"
	"io"
	"kandaoni.com/anqicms/config"
	"log"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	config     *config.PluginStorageConfig
	DataPath   string
	PublicPath string
}

func NewLocalStorage(cfg *config.PluginStorageConfig, publicPath string) (*LocalStorage, error) {
	return &LocalStorage{
		config:     cfg,
		PublicPath: publicPath,
	}, nil
}

func (s *LocalStorage) Put(ctx context.Context, key string, r io.Reader) error {
	log.Println("使用 local 上传功能", key)
	realPath := s.PublicPath + key
	//先判断文件夹是否存在，不存在就先创建
	pathDir := filepath.Dir(realPath)
	_, err := os.Stat(pathDir)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(pathDir, os.ModePerm)
		if err != nil {
			// 无法创建文件夹
			return err
		}
	}
	f, err := os.Create(realPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)

	return err
}

func (s *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	realPath := s.PublicPath + key
	buf, err := os.ReadFile(realPath)
	if err == nil {
		return io.NopCloser(bytes.NewReader(buf)), nil
	}
	return nil, err
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	realPath := s.PublicPath + key
	_, err := os.Stat(realPath)
	if err == nil {
		_ = os.Remove(realPath)
	}
	return nil
}

func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	realPath := s.PublicPath + key
	_, err := os.Stat(realPath)
	if err == nil {
		return true, nil
	}
	return false, nil
}

func (s *LocalStorage) Move(ctx context.Context, src, dest string) error {
	realSrc := s.PublicPath + src
	realDest := s.PublicPath + dest
	_, err := os.Stat(realSrc)
	if err == nil {
		//先创建目录
		_, err = os.Stat(filepath.Dir(realDest))
		if err != nil {
			err = os.MkdirAll(filepath.Dir(realDest), os.ModePerm)
		}
		return os.Rename(realSrc, realDest)
	}
	return nil
}
