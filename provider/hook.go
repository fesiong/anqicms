// 钩子
// 钩子使用方法
// 钩子 的 Data 字段，根据不同的类型，数据结构不同，但都是指针类型
// 示例：文章提交前自动生成摘要
//	hook.Register(provider.BeforeArchivePost, func(ctx *provider.HookContext) error {
//		req, ok := ctx.Data.(*request.Archive)
//		if ok && req.Description == "" {
//			req.Description = library.ParseDescription(strings.ReplaceAll(provider.CleanTagsAndSpaces(req.Content), "\n", " "))
//		}
//		return nil
//	})

package provider

import "log"

type HookPoint string

const (
	BeforeArchivePost    HookPoint = "BeforeArchivePost"
	AfterArchivePost     HookPoint = "AfterArchivePost"
	BeforeArchiveRelease HookPoint = "BeforeArchiveRelease"
	AfterArchiveRelease  HookPoint = "AfterArchiveRelease"
	BeforeViewRender     HookPoint = "BeforeViewRender"
	AfterViewRender      HookPoint = "AfterViewRender"
	BeforeGuestbookPost  HookPoint = "BeforeGuestbookPost"
	AfterGuestbookPost   HookPoint = "AfterGuestbookPost"
	BeforeCommentPost    HookPoint = "BeforeCommentPost"
	AfterCommentPost     HookPoint = "AfterCommentPost"
	BeforeUploadFile     HookPoint = "BeforeUploadFile"
	AfterUploadFile      HookPoint = "AfterUploadFile"
)

type Handler func(ctx *HookContext) error

type HookContext struct {
	Point HookPoint
	Site  *Website
	Data  interface{}            // 根据钩点不同，数据结构不同，但都是指针
	Extra map[string]interface{} // 扩展数据
	Abort bool                   // 是否终止后续流程
}

var hooks = make(map[HookPoint][]Handler)

// RegisterHook 注册钩子
func RegisterHook(point HookPoint, handler Handler) {
	log.Println("RegisterHook:", point)
	hooks[point] = append(hooks[point], handler)
}

// TriggerHook 触发钩子
func TriggerHook(ctx *HookContext) error {
	for _, handler := range hooks[ctx.Point] {
		if err := handler(ctx); err != nil || ctx.Abort {
			return err
		}
	}
	return nil
}
