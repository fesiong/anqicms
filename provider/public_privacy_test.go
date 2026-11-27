package provider

import (
	"testing"

	"kandaoni.com/anqicms/model"
)

// 公开接口不得返回评价人的邮箱与 IP；后台管理仍需要这两个字段，
// 因此清空逻辑只放在公开读取路径调用的 helper 里。
func TestSanitizePublicReviewsClearsEmailAndIp(t *testing.T) {
	reviews := []*model.OrderReview{
		{
			Id:       7,
			UserName: "alice",
			Email:    "alice@example.com",
			Ip:       "203.0.113.7",
			Content:  "很好用",
			Score:    5,
		},
		nil,
	}

	got := SanitizePublicReviews(reviews)

	if len(got) != len(reviews) {
		t.Fatalf("返回值长度 = %d，期望 %d", len(got), len(reviews))
	}
	if got[0].Email != "" {
		t.Errorf("公开评价的 email 未清空：%q", got[0].Email)
	}
	if got[0].Ip != "" {
		t.Errorf("公开评价的 ip 未清空：%q", got[0].Ip)
	}
	// 反向对照：不能把整条评价清空，只清隐私字段。
	if got[0].UserName != "alice" || got[0].Content != "很好用" || got[0].Score != 5 {
		t.Errorf("非隐私字段被误改：%+v", got[0])
	}
}

func TestSanitizePublicCommentsClearsEmailAndIpIncludingParent(t *testing.T) {
	parent := &model.Comment{
		Id:       1,
		UserName: "bob",
		Email:    "bob@example.com",
		Ip:       "198.51.100.1",
		Content:  "父评论",
	}
	comments := []*model.Comment{
		{
			Id:       2,
			UserName: "carol",
			Email:    "carol@example.com",
			Ip:       "198.51.100.2",
			Content:  "子评论",
			Parent:   parent,
		},
		nil,
	}

	got := SanitizePublicComments(comments)

	if got[0].Email != "" || got[0].Ip != "" {
		t.Errorf("公开评论的 email/ip 未清空：email=%q ip=%q", got[0].Email, got[0].Ip)
	}
	if parent.Email != "" || parent.Ip != "" {
		t.Errorf("父评论的 email/ip 未清空：email=%q ip=%q", parent.Email, parent.Ip)
	}
	if got[0].UserName != "carol" || got[0].Content != "子评论" {
		t.Errorf("非隐私字段被误改：%+v", got[0])
	}
}
