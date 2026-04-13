package request

type CommunicationMessageRequest struct {
	Id          int64  `json:"id"`
	CommOrderId int64  `json:"comm_order_id"`
	SenderId    int64  `json:"sender_id"`
	SenderType  string `json:"sender_type"`
	SenderName  string `json:"sender_name"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
}

type CommunicationOrderRequest struct {
	Id             int64                  `json:"id"`
	ArchiveId      int64                  `json:"archive_id"`
	UserId         int64                  `json:"user_id"`
	Phone          string                 `json:"phone"`
	Title          string                 `json:"title"`
	Content        string                 `json:"content"`
	OrderExtra     map[string]interface{} `json:"order_extra"`
	SenderId       int64                  `json:"sender_id"`
	SenderType     string                 `json:"sender_type"`
	SenderName     string                 `json:"sender_name"`
	QuotePrice     int64                  `json:"quote_price"`
	QuoteContent   string                 `json:"quote_content"`
	Images         []string               `json:"images"`
	SketchImages   []string               `json:"sketch_images"`
	FinishedImages []string               `json:"finished_images"`
}

type CommunicationOrderImagesRequest struct {
	Id     int64    `json:"id"`
	Field  string   `json:"field"`
	Images []string `json:"images"`
}
