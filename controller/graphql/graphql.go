package graphql

import (
	"log"
	"net/http"

	"github.com/graphql-go/graphql"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

const (
	// maxGraphQLBodyBytes GraphQL 请求体最大字节数。
	// 查询语句本身受 maxQueryBytes 限制，这里再兜一层，防止超大 body 打满内存。
	maxGraphQLBodyBytes = 256 * 1024
)

// GraphQLHandler 处理GraphQL请求
func GraphQLHandler(ctx iris.Context) {
	// 限制请求体大小，避免超大 body 打满内存
	if ctx.Request().Body != nil {
		ctx.Request().Body = http.MaxBytesReader(ctx.ResponseWriter(), ctx.Request().Body, maxGraphQLBodyBytes)
	}

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

	// 验证查询安全性：片段必须展开后再统计，否则 fragments 可以绕过复杂度限制，
	// 让超大字段集进入 graphql-go 的两两比较规则（O(n²)）打满 CPU。
	// 这里失败即拒绝，不再往下交给 graphql.Do（否则会二次解析并跳过守卫）。
	if params.Query != "" {
		if _, err := analyzeQuery(params.Query); err != nil {
			log.Printf("GraphQL query rejected: %v", err)
			ctx.JSON(iris.Map{
				"code": -1,
				"msg":  queryErrorMessage(err),
			})
			return
		}
	}

	// 通过守卫之后再取站点：不合规的请求不该碰到站点状态
	currentSite := provider.CurrentSite(ctx)

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

// GraphQLPlaygroundHandler 提供GraphQL Playground界面
func GraphQLPlaygroundHandler(ctx iris.Context) {
	// 非开发环境禁用 Playground
	if config.Server.Server.Env != "development" {
		ctx.StatusCode(404)
		return
	}
	// 使用 WriteString 输出，避免 HTML 中的 % (如 width: 100%) 被 fmt.Fprintf 当作格式符处理
	htmlContent := `
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
  <link rel="shortcut icon" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/favicon.png" />
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
</head>
<body>
  <div id="root">
    <style>
      body {
        background-color: rgb(23, 42, 58);
        font-family: Open Sans, sans-serif;
        height: 90vh;
      }
      #root {
        height: 100%;
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: center;
      }
      .loading {
        font-size: 32px;
        font-weight: 200;
        color: rgba(255, 255, 255, .6);
        margin-left: 20px;
      }
      img {
        width: 78px;
        height: 78px;
      }
      .title {
        font-weight: 400;
      }
    </style>
    <img src='https://cdn.jsdelivr.net/npm/graphql-playground-react/build/logo.png' alt=''>
    <div class="loading"> Loading
      <span class="title">GraphQL Playground</span>
    </div>
  </div>
  <script>window.addEventListener('load', function (event) {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: '/api/graphql'
      })
    })</script>
</body>
</html>
`
	ctx.ContentType("text/html; charset=utf-8")
	_, _ = ctx.WriteString(htmlContent)
}
