package library

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"time"
)

type cacheData struct {
	Expire int64
	key    string
	val    any
	prev   *cacheData
	next   *cacheData
}

type MemoryCache struct {
	mu   sync.Mutex
	list map[string]*cacheData
	head *cacheData
	tail *cacheData
	size int
	cap  int
}

func (m *MemoryCache) Set(key string, val any, expire int64) error {
	if expire == 0 {
		expire = 7200
	}
	expire = time.Now().Unix() + expire

	m.mu.Lock()
	node := &cacheData{
		Expire: expire,
		key:    key,
		val:    val,
	}
	if _, ok := m.list[key]; !ok {
		//不存在
		m.addToHead(node)
		m.list[key] = node
		m.size++
		if m.size > m.cap {
			delKey := m.removeTail()
			delete(m.list, delKey)
			m.size--
		}
	} else {
		//存在，替换
		m.list[key].Expire = expire
		m.list[key].val = val
		m.moveToHead(m.list[key])
	}
	m.mu.Unlock()

	return nil
}

func (m *MemoryCache) Get(key string, val any) error {
	rv := reflect.ValueOf(val)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &json.InvalidUnmarshalError{Type: reflect.TypeOf(val)}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.list[key]; !ok {
		//数据不存在
		return errors.New("没有缓存数据")
	}
	if m.list[key].Expire < time.Now().Unix() {
		return errors.New("缓存数据已过期")
	}
	m.moveToHead(m.list[key])

	rv.Elem().Set(reflect.ValueOf(m.list[key].val))

	return nil
}

func (m *MemoryCache) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.list[key]; ok {
		m.removeNode(m.list[key])
		delete(m.list, key)
		m.size--
	}
}

func (m *MemoryCache) CleanAll(prefix ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(prefix) > 0 {
		for k := range m.list {
			if strings.HasPrefix(k, prefix[0]) {
				m.removeNode(m.list[k])
				delete(m.list, k)
				m.size--
			}
		}
	} else {
		m.list = make(map[string]*cacheData)
		m.size = 0
		m.head = &cacheData{}
		m.tail = &cacheData{}
		m.head.next = m.tail
		m.tail.prev = m.head
	}
}

func (m *MemoryCache) moveToHead(node *cacheData) {
	//先删
	m.removeNode(node)
	m.addToHead(node)
}

func (m *MemoryCache) addToHead(node *cacheData) {
	m.head.next.prev = node
	node.next = m.head.next
	node.prev = m.head
	m.head.next = node
}

func (m *MemoryCache) removeNode(node *cacheData) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

func (m *MemoryCache) removeTail() string {
	//拿到最后一个元素
	node := m.tail.prev
	m.removeNode(node)
	return node.key
}

func (m *MemoryCache) GC() {
	for {
		timestamp := time.Now().Unix()
		m.mu.Lock()
		for k, v := range m.list {
			if v.Expire < timestamp {
				m.removeNode(v)
				delete(m.list, k)
				m.size--
			}
		}
		m.mu.Unlock()
		//每次执行完毕休息10秒
		time.Sleep(10 * time.Second)
	}
}

func InitMemoryCache() Cache {
	head := &cacheData{}
	tail := &cacheData{}
	head.next = tail
	tail.prev = head

	// 初始化一个1万容量的内存缓存
	cache := &MemoryCache{
		cap:  10000,
		size: 0,
		list: map[string]*cacheData{},
		head: head,
		tail: tail,
	}

	//执行回收
	go cache.GC()

	return cache
}
