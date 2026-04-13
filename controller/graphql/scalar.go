package graphql

import (
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

// JSONScalar 自定义 JSON 标量类型
var JSONScalar = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "JSON",
	Description: "The JSON scalar type represents JSON values",
	Serialize: func(value interface{}) interface{} {
		return value
	},
	ParseValue: func(value interface{}) interface{} {
		return value
	},
	ParseLiteral: func(valueAST ast.Value) interface{} {
		return parseLiteral(valueAST)
	},
})

// 辅助函数用于解析字面量
func parseLiteral(valueAST ast.Value) interface{} {
	switch valueAST := valueAST.(type) {
	case *ast.StringValue:
		return valueAST.Value
	case *ast.BooleanValue:
		return valueAST.Value
	case *ast.IntValue:
		return valueAST.Value
	case *ast.FloatValue:
		return valueAST.Value
	case *ast.ObjectValue:
		obj := make(map[string]interface{})
		for _, field := range valueAST.Fields {
			key := field.Name.Value
			val := parseLiteral(field.Value)
			obj[key] = val
		}
		return obj
	case *ast.ListValue:
		list := make([]interface{}, len(valueAST.Values))
		for i, val := range valueAST.Values {
			list[i] = parseLiteral(val)
		}
		return list
	case *ast.EnumValue:
		return valueAST.Value
	default:
		return nil
	}
}
