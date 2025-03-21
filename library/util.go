package library

import (
	"github.com/shirou/gopsutil/v4/mem"
	"reflect"
	"runtime"
	"strings"
)

// StructToMap 将结构体转换为 map
func StructToMap(obj interface{}) map[string]interface{} {
	objVal := reflect.ValueOf(obj)
	if objVal.Kind() == reflect.Ptr {
		objVal = objVal.Elem()
	}
	objType := objVal.Type()

	resultMap := make(map[string]interface{})
	for i := 0; i < objVal.NumField(); i++ {
		field := objVal.Field(i)
		fieldName := objType.Field(i).Name
		resultMap[strings.ToLower(fieldName)] = field.Interface()
	}
	return resultMap
}

// MapToStruct 转结构体，
func MapToStruct(m map[string]interface{}, s interface{}) error {
	// 获取结构体的反射类型
	structType := reflect.TypeOf(s).Elem()

	// 创建结构体实例
	structValue := reflect.New(structType).Elem()

	// 遍历 map
	for key, value := range m {
		// 获取结构体字段
		field := structValue.FieldByName(key)

		// 如果字段存在且是可设置的
		if field.IsValid() && field.CanSet() {
			// 将 map 中的值转换为字段对应的类型，并设置到结构体中
			mapValue := reflect.ValueOf(value)
			field.Set(mapValue.Convert(field.Type()))
		}
	}

	// 将结果赋值给目标结构体
	reflect.ValueOf(s).Elem().Set(structValue)

	return nil
}

func GetSystemMemoryUsage() (used uint64, usedPercent float64, freePercent float64) {
	v, _ := mem.VirtualMemory()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return v.Total / 1024 / 1024, (float64(m.Alloc) / float64(v.Total)) * 100, (float64(v.Available) / float64(v.Total)) * 100
}

func GetProcessMemory() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc / 1024 / 1024 // 返回MB单位
}
