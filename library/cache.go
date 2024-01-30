package library

// Cache
// 缓存，默认使用 memory，如果客户机器内存小于2G，则改成使用 file 缓存
type Cache interface {
	Get(key string, val any) error
	Set(key string, val any, expire int64) error
	Delete(key string)
	CleanAll(prefix ...string)
}
