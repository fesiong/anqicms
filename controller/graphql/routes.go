package graphql

import (
	"github.com/kataras/iris/v12"
)

// RegisterRoutes 注册GraphQL API路由
func RegisterRoutes(api iris.Party) {
	// GraphQL API 路由组
	// GraphQL 端点
	api.Post("/graphql", GraphQLHandler)

	// GraphQL Playground (可选，用于开发调试)
	api.Get("/playground", GraphQLPlaygroundHandler)
}
