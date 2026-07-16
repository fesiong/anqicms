package middleware

import (
	"github.com/kataras/iris/v12"
)

// SecurityHeaders 添加安全响应头
func SecurityHeaders(ctx iris.Context) {
	// 防止点击劫持
	ctx.Header("X-Frame-Options", "SAMEORIGIN")
	// 禁止 MIME 类型嗅探
	ctx.Header("X-Content-Type-Options", "nosniff")
	// 控制 Referer 头信息
	ctx.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	// 限制浏览器功能权限（经纬度、麦克风、摄像头等默认禁用）
	ctx.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

	ctx.Next()
}
