package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
)

// eino.go 是意图内核的 Eino / AIChat 通道适配器：把声明式意图转换为 Eino 的
// schema.ToolInfo，使 AIChat 与 MCP 共用同一份 IntentCatalog 与 CapInvoker，
// 两个通道的工具语义完全一致（任务级封装、结构化输出、两阶段 scope）。

// Handler 是通道无关的意图执行函数，与 provider.toolHandler 同构。
// 用于让 AIChat 等通道消费意图内核，而无需在 provider 与 intent 间引入类型耦合。
type Handler func(ctx context.Context, argsJSON string) (string, error)

// BuildEinoTools 产出供 Eino / AIChat 使用的工具清单与执行函数。
// 与 MCP 通道共用同一份 IntentCatalog，因此两个通道工具语义一致。
func (k *Kernel) BuildEinoTools(iconf Config) ([]*schema.ToolInfo, map[string]Handler, error) {
	out := make([]*schema.ToolInfo, 0, len(k.order))
	handlers := make(map[string]Handler, len(k.order))
	for _, name := range k.order {
		spec := k.intents[name]
		if !k.allowed(spec) {
			continue
		}
		if k.scope != nil && !k.scope[spec.Domain] {
			continue
		}
		out = append(out, k.buildEinoToolInfo(spec))
		handlers[name] = k.makeEinoHandler(name, spec)
	}
	return out, handlers, nil
}

func (k *Kernel) buildEinoToolInfo(spec *IntentSpec) *schema.ToolInfo {
	params := map[string]*schema.ParameterInfo{}
	reqSet := map[string]bool{}
	for _, r := range spec.Required {
		reqSet[r] = true
	}
	for n, p := range spec.Params {
		pi := &schema.ParameterInfo{Type: dataTypeOf(p.Type), Desc: p.Desc}
		if p.Required || reqSet[n] {
			pi.Required = true
		}
		if len(p.Enum) > 0 {
			pi.Enum = p.Enum
		}
		if p.Type == "array" && p.Items != "" {
			pi.ElemInfo = &schema.ParameterInfo{Type: dataTypeOf(p.Items)}
		}
		params[n] = pi
	}
	desc := fmt.Sprintf("[%s] %s", DomainLabel(spec.Domain), spec.Desc)
	return &schema.ToolInfo{
		Name:        spec.Name,
		Desc:        desc,
		ParamsOneOf: schema.NewParamsOneOfByParams(params),
	}
}

func dataTypeOf(t string) schema.DataType {
	switch t {
	case "integer":
		return schema.Integer
	case "number":
		return schema.Number
	case "boolean":
		return schema.Boolean
	case "array":
		return schema.Array
	case "object":
		return schema.Object
	default:
		return schema.String
	}
}

func (k *Kernel) makeEinoHandler(name string, spec *IntentSpec) Handler {
	return func(ctx context.Context, argsJSON string) (string, error) {
		start := time.Now()
		res, cerr := k.Execute(ctx, name, argsJSON)
		if k.audit != nil {
			k.audit(ctx, name, string(spec.Risk), argsJSON, cerr, start)
		}
		if cerr != nil {
			return "", cerr
		}
		if res.Data != nil {
			if b, err := json.Marshal(res.Data); err == nil {
				return string(b), nil
			}
		}
		return res.Text, nil
	}
}
