package library

import (
	"math/rand"
	"strconv"
	"sync"
	"time"
)

type verifyCode struct {
	Expire int64
	key    string
	code   string
}

type verifyCodeCache struct {
	mu   sync.Mutex
	list map[string]*verifyCode
	size int
}

var CodeCache verifyCodeCache

func (v *verifyCodeCache) Generate(key string) string {
	expire := time.Now().Unix() + 1800

	code := strconv.Itoa(100000 + rand.Intn(900000))
	if _, ok := v.list[key]; ok {
		return v.Generate(key)
	}
	v.mu.Lock()
	node := &verifyCode{
		Expire: expire,
		key:    key,
		code:   code,
	}
	v.list[key] = node
	v.size++

	v.mu.Unlock()

	return code
}

func (v *verifyCodeCache) Get(key string, clear bool) string {
	if _, ok := v.list[key]; !ok {
		//数据不存在
		return ""
	}
	if clear {
		delete(v.list, key)
	}

	return v.list[key].code
}

func (v *verifyCodeCache) GetByCode(code string, clear bool) (key string) {
	for k := range v.list {
		if v.list[k].code == code {
			key = k

			if clear {
				delete(v.list, key)
			}
			return
		}
	}

	return
}

func (v *verifyCodeCache) Verify(key, code string, clear bool) bool {
	value := v.Get(key, clear)

	return value == code
}

func (v *verifyCodeCache) Delete(code string) {
	if _, ok := v.list[code]; !ok {
		//数据不存在
		return
	}
	v.mu.Lock()
	delete(v.list, code)
	v.size--
	v.mu.Unlock()
}

func (v *verifyCodeCache) GC() {
	for {
		timestamp := time.Now().Unix()
		v.mu.Lock()
		for k, item := range v.list {
			if item.Expire < timestamp {
				delete(v.list, k)
				v.size--
			}
		}
		v.mu.Unlock()
		time.Sleep(5 * time.Second)
	}
}

func init() {
	CodeCache = verifyCodeCache{
		size: 0,
		list: map[string]*verifyCode{},
	}
	//执行回收
	go CodeCache.GC()
}
