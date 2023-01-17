package provider

import (
	"sync"
	"time"
)

// MaxSize 最多存储 1000 个对象
const MaxSize = 1000

type cacheData struct {
	Expire int64
	key    string
	val    interface{}
	prev   *cacheData
	next   *cacheData
}

type memCache struct {
	mu   sync.Mutex
	list map[string]*cacheData
	head *cacheData
	tail *cacheData
	size int
}

func (m *memCache) Set(key string, val interface{}, expire int64) {
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
		if m.size > MaxSize {
			delKey := m.removeTail()
			delete(m.list, delKey)
			m.size--
		}
	} else {
		//存在，替换
		m.list[key].val = val
		m.moveToHead(m.list[key])
	}
	m.mu.Unlock()
}

func (m *memCache) Get(key string) interface{} {
	if _, ok := m.list[key]; !ok {
		//数据不存在
		return nil
	}
	if m.list[key].Expire < time.Now().Unix() {
		return nil
	}
	m.mu.Lock()
	m.moveToHead(m.list[key])
	m.mu.Unlock()
	return m.list[key].val
}

func (m *memCache) Delete(key string) {
	if _, ok := m.list[key]; !ok {
		//数据不存在
		return
	}
	m.mu.Lock()
	m.removeNode(m.list[key])
	delete(m.list, key)
	m.size--
	m.mu.Unlock()
}

func (m *memCache) CleanAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.list = map[string]*cacheData{}
	m.size = 0
	m.head = &cacheData{}
	m.tail = &cacheData{}
	m.head.next = m.tail
	m.tail.prev = m.head
}

func (m *memCache) moveToHead(node *cacheData) {
	//先删
	m.removeNode(node)
	m.addToHead(node)
}

func (m *memCache) addToHead(node *cacheData) {
	m.head.next.prev = node
	node.next = m.head.next
	node.prev = m.head
	m.head.next = node
}

func (m *memCache) removeNode(node *cacheData) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

func (m *memCache) removeTail() string {
	//拿到最后一个元素
	node := m.tail.prev
	m.removeNode(node)
	return node.key
}

func (m *memCache) GC() {
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

func (w *Website) InitMemCache() {
	head := &cacheData{}
	tail := &cacheData{}
	head.next = tail
	tail.prev = head

	w.MemCache = &memCache{
		size: 0,
		list: map[string]*cacheData{},
		head: head,
		tail: tail,
	}

	//执行回收
	go w.MemCache.GC()
}
