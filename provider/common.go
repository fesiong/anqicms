package provider

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
