package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
)

const (
	IndexCacheKey = "index"

	UserAgentPc     = "pc"
	UserAgentMobile = "mobile"
)

func (w *Website) CacheIndex(ua string, body []byte) {
	w.MemCache.Set(IndexCacheKey+ua, body, 3600)
}

func (w *Website) GetIndexCache(ua string) []byte {
	body := w.MemCache.Get(IndexCacheKey + ua)

	if body == nil {
		return nil
	}

	content, ok := body.([]byte)
	if ok {
		return content
	}

	return nil
}

func (w *Website) DeleteCacheIndex() {
	w.MemCache.Delete(IndexCacheKey + UserAgentPc)
	w.MemCache.Delete(IndexCacheKey + UserAgentMobile)
}

func init() {
	// check what if this server can visit google
	go func() {
		resp, err := library.GetURLData("https://www.google.com", "", 5)
		if err != nil {
			config.GoogleValid = true
		} else {
			log.Println("google-status", resp.StatusCode)
		}
	}()
}
