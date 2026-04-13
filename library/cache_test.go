package library

import (
	"log"
	"testing"
)

func TestCache(t *testing.T) {
	type CacheTest struct {
		Key   string
		Value any
	}

	var data = CacheTest{
		Key:   "test",
		Value: "test",
	}

	cache := InitMemoryCache()

	// 测试1: 存储指针，获取值
	err := cache.Set("test1", &data, 0)
	if err != nil {
		t.Fatal(err)
	}

	var dataValue CacheTest
	err = cache.Get("test1", &dataValue)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("获取到结果1：%#v", dataValue)

	// 测试2: 存储指针，获取指针
	var dataPtr *CacheTest
	err = cache.Get("test1", &dataPtr)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("获取到结果2：%#v", dataPtr)

	// 测试3: 存储值，获取值
	err = cache.Set("test2", data, 0)
	if err != nil {
		t.Fatal(err)
	}

	var dataValue2 CacheTest
	err = cache.Get("test2", &dataValue2)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("获取到结果3：%#v", dataValue2)
}
