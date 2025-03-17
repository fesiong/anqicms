package provider

import (
	"kandaoni.com/anqicms/library"
)

func (w *Website) InitCache() {
	// 判断内存大小
	usedMb, _, _ := library.GetSystemMemoryUsage()
	cacheType := w.GetSettingValue(CacheTypeKey)
	if cacheType == "" && usedMb <= 1*1024 {
		cacheType = "file"
	}
	if cacheType == "file" {
		w.Cache = library.InitFileCache(w.CachePath)
	} else {
		w.Cache = library.InitMemoryCache()
	}
}
