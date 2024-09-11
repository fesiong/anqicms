package fate

import (
	"kandaoni.com/anqicms/request"
	"sync"
	"time"
)

type CacheData struct {
	Req   request.NameRequest
	Names []*Name
	Fate  *Fate
}

type fateData struct {
	Expire int64
	key    string
	val    *CacheData
}

type fateCache struct {
	mu   sync.Mutex
	list map[string]*fateData
	keys []string
	size int
	cap  int
}

func (f *fateCache) Set(key string, val *CacheData) {
	// 5分钟过期
	expire := time.Now().Unix() + 300

	f.mu.Lock()
	node := &fateData{
		Expire: expire,
		key:    key,
		val:    val,
	}
	if _, ok := f.list[key]; !ok {
		//不存在
		f.list[key] = node
		f.keys = append(f.keys, key)
		f.size++
		if f.size >= f.cap {
			firstKey := f.keys[0]
			f.keys = f.keys[1:]
			delete(f.list, firstKey)
			f.size--
		}
	} else {
		//存在，替换
		f.list[key].Expire = expire
		f.list[key].val = val
	}
	f.mu.Unlock()
}

func (f *fateCache) Get(key string) *CacheData {
	f.mu.Lock()
	defer f.mu.Unlock()
	if data, ok := f.list[key]; ok {
		//数据存在
		return data.val
	}
	return nil
}

func (f *fateCache) GC() {
	for {
		timestamp := time.Now().Unix()
		f.mu.Lock()
		for k, v := range f.list {
			if v.Expire < timestamp {
				delete(f.list, k)
				for i := range f.keys {
					if f.keys[i] == k {
						f.keys = append(f.keys[:i], f.keys[i+1:]...)
						break
					}
				}
				f.size--
			}
		}
		f.mu.Unlock()
		//每次执行完毕休息10秒
		time.Sleep(10 * time.Second)
	}
}

var FateCache fateCache

func init() {
	FateCache = fateCache{
		mu:   sync.Mutex{},
		cap:  500,
		size: 0,
		list: map[string]*fateData{},
		keys: []string{},
	}

	//执行回收
	go FateCache.GC()
}
