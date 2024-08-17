package provider

import (
	"github.com/shirou/gopsutil/v3/mem"
	"kandaoni.com/anqicms/library"
)

func (w *Website) InitCache() {
	// 判断内存大小
	vm, _ := mem.VirtualMemory()
	cacheType := w.GetSettingValue(CacheTypeKey)
	if cacheType == "" && vm.Total <= 1*1024*1024*1024 {
		cacheType = "file"
	}
	if cacheType == "file" {
		w.Cache = library.InitFileCache(w.CachePath)
	} else {
		w.Cache = library.InitMemoryCache()
	}
}
