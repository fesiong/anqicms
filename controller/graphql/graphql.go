package graphql

import (
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"log"
)

const (
	maxQueryDepth      = 5  // 最大查询深度
	maxQueryComplexity = 50 // 最大查询复杂度（并行查询数）
)

// calculateDepth 计算 GraphQL 查询的深度
func calculateDepth(selectionSet *ast.SelectionSet) int {
	if selectionSet == nil || len(selectionSet.Selections) == 0 {
		return 1
	}
	maxDepth := 0
	for _, sel := range selectionSet.Selections {
		switch s := sel.(type) {
		case *ast.Field:
			depth := 1
			if s.SelectionSet != nil && len(s.SelectionSet.Selections) > 0 {
				depth = 1 + calculateDepth(s.SelectionSet)
			}
			if depth > maxDepth {
				maxDepth = depth
			}
		case *ast.InlineFragment:
			depth := calculateDepth(s.SelectionSet)
			if depth > maxDepth {
				maxDepth = depth
			}
		case *ast.FragmentSpread:
			// 片段引用的深度在完整解析时才能确定，保守估计为1
			if 1 > maxDepth {
				maxDepth = 1
			}
		}
	}
	return maxDepth
}

// countParallelQueries 计算并行查询数（根级别的字段数）
func countParallelQueries(selectionSet *ast.SelectionSet) int {
	if selectionSet == nil {
		return 0
	}
	count := 0
	for _, sel := range selectionSet.Selections {
		switch s := sel.(type) {
		case *ast.Field:
			count++
		case *ast.InlineFragment:
			count += countParallelQueries(s.SelectionSet)
		case *ast.FragmentSpread:
			count++
		}
	}
	return count
}

// validateQuery 验证 GraphQL 查询的安全性
func validateQuery(query string) (int, int, error) {
	src := source.NewSource(&source.Source{
		Body: []byte(query),
	})
	doc, err := parser.Parse(parser.ParseParams{
		Source: src,
	})
	if err != nil {
		return 0, 0, err
	}

	var depth, complexity int
	for _, def := range doc.Definitions {
		switch d := def.(type) {
		case *ast.OperationDefinition:
			if d.SelectionSet != nil && len(d.SelectionSet.Selections) > 0 {
				depth = calculateDepth(d.SelectionSet)
				complexity = countParallelQueries(d.SelectionSet)
			}
		}
	}

	return depth, complexity, nil
}

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

	// 验证查询安全性
	if params.Query != "" {
		depth, complexity, err := validateQuery(params.Query)
		if err != nil {
			log.Printf("GraphQL parse error: %v", err)
		} else {
			if depth > maxQueryDepth {
				ctx.JSON(iris.Map{
					"code": -1,
					"msg":  "Query too deep",
				})
				return
			}
			if complexity > maxQueryComplexity {
				ctx.JSON(iris.Map{
					"code": -1,
					"msg":  "Query too complex",
				})
				return
			}
		}
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
