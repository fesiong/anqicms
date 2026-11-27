package controller

import (
	"strings"
	"testing"
)

// 公开读取评价/评论的入口必须清空 email / ip。
//
// 这条护栏锁的是“每个公开入口都调用了 sanitizer”，而不是某个日志文本：
// 少调一处，就会重新暴露评价人/评论人的邮箱与 IP。
// 反向对照见 TestAdminOrderReviewListKeepsPrivacyFields —— 后台不能被误伤。
func TestPublicReviewAndCommentHandlersSanitizePrivacy(t *testing.T) {
	cases := []struct {
		file string
		fn   string
		want string
	}{
		{"apiTag.go", "ApiReviewList", "SanitizePublicReviews("},
		{"apiTag.go", "ApiCommentList", "SanitizePublicComments("},
		{"apiTag.go", "ApiCommentPraise", "SanitizePublicComment("},
		{"apiOrder.go", "ApiReviewPraise", "SanitizePublicReview("},
		{"comment.go", "CommentPraise", "SanitizePublicComment("},
		{"graphql/resolvers.go", "resolveReviews", "SanitizePublicReviews("},
		{"graphql/resolvers.go", "resolveComments", "SanitizePublicComments("},
	}

	for _, c := range cases {
		src := readFuncSource(t, c.file, c.fn)
		if !strings.Contains(src, c.want) {
			t.Errorf("%s 未调用 %s：公开接口会把评价/评论的 email/ip 原样返回", c.fn, c.want)
		}
	}
}

// 后台评价列表需要 email/ip 用于管理，不能被公开脱敏逻辑误伤。
func TestAdminOrderReviewListKeepsPrivacyFields(t *testing.T) {
	src := readFuncSource(t, "manageController/pluginOrder.go", "PluginGetOrderReviews")
	for _, bad := range []string{"SanitizePublicReview(", "SanitizePublicReviews("} {
		if strings.Contains(src, bad) {
			t.Errorf("PluginGetOrderReviews 不应调用 %s：后台需要 email/ip 用于管理", bad)
		}
	}
}
