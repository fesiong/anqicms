package request

// CommunicationMessageRequest 询盘站内消息请求。
// sender_* 由服务端按当前登录管理员身份覆盖，客户端传了也会被改写。
type CommunicationMessageRequest struct {
	Id          int64  `json:"id"`            // 消息 ID（删除时用）
	CommOrderId int64  `json:"comm_order_id"` // 所属询盘订单 ID（发送时必填）
	SenderId    int64  `json:"sender_id"`     // 发送者 ID（服务端按当前管理员覆盖，无需传）
	SenderType  string `json:"sender_type"`   // 发送者类型，后台固定写成 admin
	SenderName  string `json:"sender_name"`   // 发送者名称（服务端按当前管理员覆盖，无需传）
	MessageType string `json:"message_type"`  // 消息类型，留空按 text 处理
	Content     string `json:"content"`       // 消息内容
}

// CommunicationOrderRequest 询盘订单请求（报价 / 关闭）。
// ⚠️ quote_price 单位是**分**，且必须 > 0，否则端点直接返回 Invalid Params。
type CommunicationOrderRequest struct {
	Id             int64                  `json:"id"`              // 询盘订单 ID
	ArchiveId      int64                  `json:"archive_id"`      // 关联商品（文档）ID
	UserId         int64                  `json:"user_id"`         // 提交询盘的会员 ID，0 表示游客
	Phone          string                 `json:"phone"`           // 联系电话
	Title          string                 `json:"title"`           // 询盘标题
	Content        string                 `json:"content"`         // 询盘/报价正文
	OrderExtra     map[string]interface{} `json:"order_extra"`     // 询盘表单自定义字段
	SenderId       int64                  `json:"sender_id"`       // 操作人 ID（服务端按当前管理员覆盖）
	SenderType     string                 `json:"sender_type"`     // 操作人类型
	SenderName     string                 `json:"sender_name"`     // 操作人名称（服务端按当前管理员覆盖）
	QuotePrice     int64                  `json:"quote_price"`     // 报价金额（分），必须 > 0
	QuoteContent   string                 `json:"quote_content"`   // 报价说明
	Images         []string               `json:"images"`          // 询盘附图 URL 列表
	SketchImages   []string               `json:"sketch_images"`   // 设计稿图片 URL 列表
	FinishedImages []string               `json:"finished_images"` // 成品图片 URL 列表
}

// CommunicationOrderImagesRequest 只改询盘订单的图片字段，其余字段不动。
type CommunicationOrderImagesRequest struct {
	Id     int64    `json:"id"`     // 询盘订单 ID
	Field  string   `json:"field"`  // 要替换的图片字段：images / sketch_images / finished_images
	Images []string `json:"images"` // 该字段的新图片 URL 列表（整体替换，不是追加）
}
