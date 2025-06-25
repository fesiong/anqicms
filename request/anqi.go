package request

type AnqiLoginRequest struct {
	UserName string `json:"user_name"`
	Password string `json:"password"`
}

type AnqiTemplateRequest struct {
	TemplateId    uint     `json:"template_id"`
	OnlyTemplate  bool     `json:"only_template"`
	AutoBackup    bool     `json:"auto_backup"`
	Name          string   `json:"name"`
	Price         int64    `json:"price"`
	Author        string   `json:"author"`
	Package       string   `json:"package"`
	Version       string   `json:"version"`
	Description   string   `json:"description"`
	Homepage      string   `json:"homepage"`
	TemplateType  int      `json:"template_type"`
	PCThumb       string   `json:"pc_thumb"`
	MobileThumb   string   `json:"mobile_thumb"`
	Content       string   `json:"content"`
	PreviewImages []string `json:"preview_images"`
	TemplatePath  string   `json:"template_path"`
}

type AnqiFeedbackRequest struct {
	Title    string   `json:"title"`
	Type     string   `json:"type"`
	Content  string   `json:"content"`
	Domain   string   `json:"domain"`
	Version  string   `json:"version"`
	Platform string   `json:"platform"`
	Images   []string `json:"images"`
}

type AnqiExtractRequest struct {
	Text string `json:"text"`
	Num  int    `json:"num"`
}
