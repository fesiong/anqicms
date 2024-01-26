package provider

import (
	"github.com/shirou/gopsutil/v3/mem"
	"kandaoni.com/anqicms/library"
)

func (w *Website) InitCache() {
	// 判断内存大小
	vm, _ := mem.VirtualMemory()
	if vm.Total < 2*1024*1024*1024 {
		w.Cache = library.InitFileCache(w.CachePath)
	} else {
		w.Cache = library.InitMemoryCache()
	}
}
