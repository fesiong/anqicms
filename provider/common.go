package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
)

func (w *Website) DeleteCacheIndex() {
	w.RemoveHtmlCache("/")
	go func() {
		// 上传到静态服务器
		cachePath := w.CachePath + "pc"
		w.BuildIndexCache()
		_ = w.SyncHtmlCacheToStorage(cachePath+"/index.html", "index.html")
	}()
}

func init() {
	// check what if this server can visit google
	go func() {
		resp, err := library.GetURLData("https://www.google.com", "", 5)
		if err != nil {
			config.GoogleValid = false
		} else {
			config.GoogleValid = true
			log.Println("google-status", resp.StatusCode)
		}
	}()
}
