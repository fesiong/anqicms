package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type cacheData struct {
	Expire int64
	key    string
	val    any
}

type MemoryCache struct {
	mu          sync.RWMutex // 使用读写锁提高并发性能
	list        map[string]*cacheData
	lastCleanup time.Time // 上次清理时间
	lastGC      time.Time // 上次GC时间
	pending     map[string]*sync.WaitGroup
	// 缓存统计
	hits   uint64 // 缓存命中次数
	misses uint64 // 缓存未命中次数
	total  uint64 // 总访问次数
}

func (m *MemoryCache) Set(key string, val any, expire int64) error {
	if expire == 0 {
		expire = 7200
	}
	expire = time.Now().Unix() + expire

	node := &cacheData{
		Expire: expire,
		key:    key,
		val:    val,
	}

	m.mu.Lock()
	m.list[key] = node

	// 定期清理过期数据
	if time.Since(m.lastCleanup) > time.Minute*5 {
		go m.cleanExpiredOrOldest()
		m.lastCleanup = time.Now()
	}
	m.mu.Unlock()

	return nil
}

func (m *MemoryCache) Get(key string, val any) error {
	// 增加总访问次数
	atomic.AddUint64(&m.total, 1)

	rv := reflect.ValueOf(val)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &json.InvalidUnmarshalError{Type: reflect.TypeOf(val)}
	}

	// 先使用读锁检查
	m.mu.RLock()
	node, ok := m.list[key]
	if !ok {
		m.mu.RUnlock()
		// 增加未命中次数
		atomic.AddUint64(&m.misses, 1)
		return errors.New("没有缓存数据")
	}

	// 检查是否过期
	if node.Expire < time.Now().Unix() {
		m.mu.RUnlock()
		// 异步删除过期数据
		go m.Delete(key)
		// 增加未命中次数
		atomic.AddUint64(&m.misses, 1)
		return errors.New("缓存数据已过期")
	}

	// 复制值，避免在锁内进行可能耗时的反射操作
	cachedVal := node.val
	m.mu.RUnlock()

	// 增加命中次数
	atomic.AddUint64(&m.hits, 1)

	// 设置返回值
	cachedValRef := reflect.ValueOf(cachedVal)
	elem := rv.Elem()
	// 如果类型匹配，直接赋值
	if cachedValRef.Type().AssignableTo(elem.Type()) {
		elem.Set(cachedValRef)
		return nil
	}

	// 如果缓存值是指针，且指针指向的类型与目标类型匹配，则解引用
	if cachedValRef.Kind() == reflect.Ptr {
		if cachedValRef.Elem().Type().AssignableTo(elem.Type()) {
			elem.Set(cachedValRef.Elem())
			return nil
		}
	}

	return fmt.Errorf("type error: cache: %v, respect: %v", cachedValRef.Type(), elem.Type())
}

func (m *MemoryCache) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.list, key)
}

func (m *MemoryCache) CleanAll(prefix ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(prefix) > 0 {
		for k := range m.list {
			if strings.HasPrefix(k, prefix[0]) {
				delete(m.list, k)
			}
		}
	} else {
		// 完全重置缓存
		m.list = make(map[string]*cacheData)
	}

	// 更新最后清理时间
	m.lastCleanup = time.Now()
}

// 清理部分缓存，按照过期时间排序，清理最早过期的数据
func (m *MemoryCache) CleanByPercent(percent float64) int {
	if percent <= 0 || percent >= 100 {
		return 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 计算需要删除的数量
	totalSize := len(m.list)
	toRemove := int(float64(totalSize) * percent / 100)
	if toRemove <= 0 {
		return 0
	}

	// 收集所有缓存项并按过期时间排序
	items := make([]*cacheData, 0, totalSize)
	for _, item := range m.list {
		items = append(items, item)
	}

	// 按过期时间排序，优先删除即将过期的
	sort.Slice(items, func(i, j int) bool {
		return items[i].Expire < items[j].Expire
	})

	// 删除指定比例的缓存
	removed := 0
	for i := 0; i < toRemove && i < len(items); i++ {
		delete(m.list, items[i].key)
		removed++
	}

	// 更新最后清理时间
	m.lastCleanup = time.Now()
	return removed
}

// 清理过期或最旧的数据
func (m *MemoryCache) cleanExpiredOrOldest() {
	now := time.Now().Unix()
	expiredKeys := make([]string, 0, 32) // 预分配一个合理的初始容量

	m.mu.Lock()
	// 先收集过期的key
	for key, item := range m.list {
		if item.Expire < now {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// 批量删除过期数据
	for _, key := range expiredKeys {
		delete(m.list, key)
	}
	m.mu.Unlock()

	// 如果有过期数据被清理，更新最后清理时间
	if len(expiredKeys) > 0 {
		m.lastCleanup = time.Now()
	}
}

func (m *MemoryCache) Pending(key string) (*sync.WaitGroup, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	wg, ok := m.pending[key]
	return wg, ok
}

func (m *MemoryCache) AddPending(key string, wg *sync.WaitGroup) {
	m.mu.Lock()
	m.pending[key] = wg
	m.mu.Unlock()
}

func (m *MemoryCache) DelPending(key string) {
	m.mu.Lock()
	delete(m.pending, key)
	m.mu.Unlock()
}

func (m *MemoryCache) GC() {
	for {
		// 每分钟检查一次系统内存使用情况
		if time.Since(m.lastGC) > time.Minute {
			_, appUsedPercent, sysFreePercent := GetSystemMemoryUsage()

			// 如果应用内存使用率超过65%，清理一部分缓存
			if appUsedPercent > 65 || (sysFreePercent < 10 && len(m.list) > 10000) {
				// 清理25%的缓存
				m.CleanByPercent(25)
			}
			m.lastGC = time.Now()
		}

		// 定期清理过期数据
		m.cleanExpiredOrOldest()

		// 每次执行完毕休息30秒
		time.Sleep(30 * time.Second)
	}
}

func InitMemoryCache() Cache {
	// 初始化内存缓存
	cache := &MemoryCache{
		list:        make(map[string]*cacheData),
		lastCleanup: time.Now(),
		lastGC:      time.Now(),
		pending:     make(map[string]*sync.WaitGroup),
	}

	// 执行回收
	go cache.GC()

	return cache
}
