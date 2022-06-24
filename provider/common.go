package provider

import "kandaoni.com/anqicms/library"

const (
	IndexCacheKey = "index"
)

func CacheIndex(body []byte) {
	library.MemCache.Set(IndexCacheKey, body, 3600)
}

func GetIndexCache() []byte {
	body := library.MemCache.Get(IndexCacheKey)

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
	library.MemCache.Delete(IndexCacheKey)
}