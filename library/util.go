package library

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v4/mem"
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
		fieldName := strings.ToLower(objType.Field(i).Name)
		jsonTag := objType.Field(i).Tag.Get("json")
		if jsonTag != "" {
			fieldName = strings.Split(jsonTag, ",")[0]
		}
		resultMap[fieldName] = field.Interface()
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

func GetTopDomain(domain string) string {
	//第二后缀部分，com,org,gov,net,和cn所有2个字母的部分，可以认为是双后缀
	items := strings.Split(domain, ".")
	if len(items) <= 2 {
		//只有2个，就是顶级了
		return domain
	}
	//先截取成三个
	items = items[len(items)-3:]
	//先判断是否是双后缀，
	if items[1] == "com" || items[1] == "org" || items[1] == "gov" || items[1] == "net" || (len(items[1]) == 2 && items[2] == "cn") {
		//认为是双后缀
		return strings.Join(items, ".")
	}
	//一般的域名
	items = items[1:]
	return strings.Join(items, ".")
}
