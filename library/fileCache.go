package library

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
)

type FileCache struct {
	mu        sync.Mutex
	suffix    string
	cachePath string
}

type FileCacheData struct {
	Expire int64  `json:"expire"`
	Data   []byte `json:"data"`
}

func (m *FileCache) Get(key string, val any) error {
	rv := reflect.ValueOf(val)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &json.InvalidUnmarshalError{Type: reflect.TypeOf(val)}
	}

	cacheFile := m.cachePath + key + m.suffix
	info, err := os.Stat(cacheFile)
	if err != nil {
		return err
	}

	buf, err := os.ReadFile(cacheFile)
	if err != nil {
		return err
	}
	var fileData FileCacheData
	if err = json.Unmarshal(buf, &fileData); err != nil {
		_ = os.Remove(cacheFile)
		return err
	}
	if fileData.Expire > 0 && info.ModTime().Before(time.Now().Add(-time.Duration(fileData.Expire)*time.Second)) {
		err = os.Remove(cacheFile)
		return errors.New("cache-expire")
	}
	// 这里实际上应该还需要对数据进行还原
	err = json.Unmarshal(fileData.Data, val)
	if err != nil {
		_ = os.Remove(cacheFile)
		return err
	}
	return nil
}

func (m *FileCache) Set(key string, val any, expire int64) error {
	cacheFile := m.cachePath + key + m.suffix
	valData, err := json.Marshal(val)
	if err != nil {
		return err
	}
	fileData := FileCacheData{
		Expire: expire,
		Data:   valData,
	}
	saveData, err := json.Marshal(fileData)
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFile, saveData, 0644)
}

func (m *FileCache) Delete(key string) {
	cacheFile := m.cachePath + key + m.suffix
	_ = os.Remove(cacheFile)
}

func (m *FileCache) CleanAll(prefix ...string) {
	if len(m.cachePath) == 0 {
		return
	}
	if len(prefix) > 0 {
		// 遍历cachePath，删除prefix[0]前缀的的文件
		_ = filepath.Walk(m.cachePath, func(path string, info os.FileInfo, err error) error {
			if info == nil || info.IsDir() {
				return nil
			}
			if strings.HasPrefix(path, prefix[0]) {
				_ = os.Remove(path)
			}
			return nil
		})
	} else {
		_ = os.RemoveAll(m.cachePath)
	}
}

func InitFileCache(cachePath string) Cache {
	cachePath = cachePath + "data/"
	_, err := os.Stat(cachePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		_ = os.Mkdir(cachePath, os.ModePerm)
	}
	cache := &FileCache{
		suffix:    ".cache.json",
		cachePath: cachePath,
	}

	return cache
}
