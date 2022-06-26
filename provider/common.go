package provider

import "kandaoni.com/anqicms/library"

const (
	IndexCacheKey = "index"

	UserAgentPc     = "pc"
	UserAgentMobile = "mobile"
)

func CacheIndex(ua string, body []byte) {
	library.MemCache.Set(IndexCacheKey+ua, body, 3600)
}

func GetIndexCache(ua string) []byte {
	body := library.MemCache.Get(IndexCacheKey + ua)

	if body == nil {
		return nil
	}

	content, ok := body.([]byte)
	if ok {
		return content
	}

	return nil
}

func DeleteCacheIndex() {
	library.MemCache.Delete(IndexCacheKey + UserAgentPc)
	library.MemCache.Delete(IndexCacheKey + UserAgentMobile)
}
