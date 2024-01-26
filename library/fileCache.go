package library

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
)

type FileCache struct {
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

	buf, err := os.ReadFile(cacheFile)
	if err != nil {
		return err
	}
	var fileData FileCacheData
	if err = json.Unmarshal(buf, &fileData); err != nil {
		return err
	}
	// 这里实际上应该还需要对数据进行还原
	err = json.Unmarshal(fileData.Data, val)
	if err != nil {
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

func (m *FileCache) CleanAll() {
	if len(m.cachePath) == 0 {
		return
	}

	_ = os.RemoveAll(m.cachePath)
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
