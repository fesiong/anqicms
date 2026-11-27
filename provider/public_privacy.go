package provider

import "kandaoni.com/anqicms/model"

// SanitizePublicReview 清空公开接口不应返回的隐私字段（就地修改）。
//
// 背景：/api/review/list 等公开接口直接序列化 model.OrderReview，
// 会把评价人的 email / ip 一并返回。前端“没显示”不等于接口没暴露。
// 后台评价管理仍需要这两个字段，因此清空只发生在公开读取路径。
func SanitizePublicReview(review *model.OrderReview) *model.OrderReview {
	if review == nil {
		return nil
	}
	review.Email = ""
	review.Ip = ""
	return review
}

// SanitizePublicReviews 是 SanitizePublicReview 的切片版本。
func SanitizePublicReviews(reviews []*model.OrderReview) []*model.OrderReview {
	for _, review := range reviews {
		SanitizePublicReview(review)
	}
	return reviews
}

// SanitizePublicComment 清空公开接口不应返回的隐私字段（就地修改）。
//
// GetCommentList 会把被回复的父评论挂在 Parent 上一起序列化，
// 所以 Parent 的 email / ip 也必须一并清空，否则从父评论仍能读到。
func SanitizePublicComment(comment *model.Comment) *model.Comment {
	if comment == nil {
		return nil
	}
	comment.Email = ""
	comment.Ip = ""
	if comment.Parent != nil {
		comment.Parent.Email = ""
		comment.Parent.Ip = ""
	}
	return comment
}

// SanitizePublicComments 是 SanitizePublicComment 的切片版本。
func SanitizePublicComments(comments []*model.Comment) []*model.Comment {
	for _, comment := range comments {
		SanitizePublicComment(comment)
	}
	return comments
}
