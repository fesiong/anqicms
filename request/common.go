package request

const (
	LogActionViews = "views"
	LogActionOrder = "order"
	LogActionUser  = "user"

	LogTypeArchive     = "archive"
	LogTypeArticleView = "article_view"
	LogTypeProductView = "product_view"
	LogTypeCart        = "cart"
	LogTypeCheckout    = "checkout"
	LogTypeConfirm     = "confirm"
	LogTypePayment     = "payment"
	LogTypeFirstOrder  = "first_order"
	LogTypeRepurchase  = "repurchase"
	LogTypeOrderCreate = "order_create"
	LogTypePayCreate   = "pay_create"
	LogTypeCouponGet   = "coupon_get"
	LogTypeCouponUse   = "coupon_use"
	LogTypeUserCreate  = "user_create"
	LogTypeUserLogin   = "user_login"
)

type LogStatisticRequest struct {
	Action string `json:"action" form:"action"` // views click ...
	Type   string `json:"type" form:"type"`     // archive ...
	Id     int64  `json:"id" form:"id"`         // target id
	Code   int    `json:"code" form:"code"`     // status code
	Path   string `json:"path" form:"path"`
}
