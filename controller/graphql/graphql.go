package graphql

import (
	"github.com/graphql-go/graphql"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
	"log"
)

// GraphQLHandler 处理GraphQL请求
func GraphQLHandler(ctx iris.Context) {
	// 获取当前站点
	currentSite := provider.CurrentSite(ctx)
	// 解析GraphQL请求
	var params struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
	}

	if err := ctx.ReadJSON(&params); err != nil {
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  "invalid GraphQL params",
		})
		return
	}

	// 执行GraphQL查询
	result := graphql.Do(graphql.Params{
		Schema:         Schema,
		RequestString:  params.Query,
		VariableValues: params.Variables,
		OperationName:  params.OperationName,
		Context:        ctx.Request().Context(),
		RootObject: map[string]interface{}{
			"site": currentSite,
			"ctx":  ctx,
		},
	})

	if len(result.Errors) > 0 {
		log.Printf("GraphQL Error: %#v", result.Errors)
		ctx.JSON(iris.Map{
			"code": -1,
			"msg":  result.Errors[0].Message,
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": 0,
		"data": result.Data,
		"msg":  "",
	})
}
