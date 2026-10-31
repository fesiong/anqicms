package manageController

import (
	"bytes"
	"io"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
)

// 写端点的入参护栏。
//
// 起因：端点测试里 51 个写端点在请求体为空时照样回"已保存/已删除"，24 个端点在
// id 不存在时照样回成功。后台表单都是整份提交，空载荷只可能来自误调用或自动化试探，
// 放行的后果是配置被零值覆盖（robots.txt 就这样被清成 0 字节）、删除接口什么都不删。
//
// 约定：这些 helper 返回 false 时已经把响应写好了，调用方必须立刻 return。

// isEmptyJSONPayload 判断请求体是否"什么都没带"。
// 只看结构不看语义：`{}`、`{ }`、`null`、空白都算空；`{"ids":[]}` 不算空
// （字段存在但值空，是调用方明确表达"空集合"，由各端点自己决定怎么处理）。
func isEmptyJSONPayload(body []byte) bool {
	compact := bytes.Join(bytes.Fields(body), nil)
	if len(compact) == 0 {
		return true
	}
	if bytes.Equal(compact, []byte("{}")) || bytes.Equal(compact, []byte("null")) {
		return true
	}
	return false
}

// readJSONPayload 读取并绑定请求体，空载荷直接拒绝。
// 用它替换 `if err := ctx.ReadJSON(&req); err != nil {...}` 那七行。
func readJSONPayload(ctx iris.Context, outPtr any) bool {
	body, err := ctx.GetBody()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return false
	}
	// GetBody 是否复位请求体取决于应用是否开启 DisableBodyConsumptionOnUnmarshal，
	// 这里显式补回，保证随后的 ReadJSON 仍能读到原始内容。
	ctx.Request().Body = io.NopCloser(bytes.NewReader(body))

	if isEmptyJSONPayload(body) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("EmptyRequestPayload"),
		})
		return false
	}

	if err := ctx.ReadJSON(outPtr); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return false
	}
	return true
}

// 整型泛型约束：请求结构体里的 id 有 int、uint、int64 几种写法，
// 若 helper 只收 int，每个调用点都要先做一次无意义的类型转换。
type integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// requirePositive 要求整型参数为正数（id、coupon_id 这类）。
func requirePositive[T integer](ctx iris.Context, value T, field string) bool {
	if value > 0 {
		return true
	}
	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  ctx.Tr("MissingRequiredParameter", field),
	})
	return false
}

// requireAnyID 要求"单个 id 或 id 列表"至少给了一个。
func requireAnyID[A integer, B integer](ctx iris.Context, id A, ids []B) bool {
	if id > 0 || len(ids) > 0 {
		return true
	}
	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  ctx.Tr("MissingRequiredParameter", "id"),
	})
	return false
}

// requireNonEmpty 要求字符串参数非空（field_name、ip、name 这类）。
func requireNonEmpty(ctx iris.Context, value string, field string) bool {
	if value != "" {
		return true
	}
	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  ctx.Tr("MissingRequiredParameter", field),
	})
	return false
}

// requireFound 要求"确实命中了目标记录"。
// 删除/修改类端点找不到记录时必须报错，否则调用方分不清"已经删掉"和"本来就没有"，
// 而这正是"id 不存在也回成功"的来源。
func requireFound(ctx iris.Context, found bool) bool {
	if found {
		return true
	}
	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  ctx.Tr("RecordNotFound"),
	})
	return false
}
