package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// MergeJSONPayload 把请求体中「实际出现的字段」合并进 dst 指向的结构体副本，
// 实现「部分更新」语义：
//   - payload 里显式传入的字段会覆盖 dst 对应字段（包括空字符串、0、false、null，
//     用于主动清空）；
//   - payload 里未出现的字段保持 dst 原值，不会被零值覆盖。
//
// 字段按结构体 json tag 匹配；无 tag、tag 为 "-" 或非导出字段不会被更新。
// 典型用途：设置类保存端点既接收管理后台的整包提交，也接收 MCP 意图层的
// 部分字段提交，未传字段不能被误清。
func MergeJSONPayload(payload map[string]json.RawMessage, dst any) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return errors.New("MergeJSONPayload: dst 必须是非空结构体指针")
	}
	elem := rv.Elem()
	t := elem.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		raw, ok := payload[name]
		if !ok {
			continue
		}
		if err := json.Unmarshal(raw, elem.Field(i).Addr().Interface()); err != nil {
			return fmt.Errorf("MergeJSONPayload: 字段 %s 类型不匹配: %w", name, err)
		}
	}
	return nil
}
