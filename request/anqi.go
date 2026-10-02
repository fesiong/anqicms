package request

type AnqiLoginRequest struct {
	UserName string `json:"user_name"` // 安企云账号用户名
	Password string `json:"password"`  // 安企云账号密码
}

type AnqiTemplateRequest struct {
	TemplateId    uint     `json:"template_id" ast:"-"`   // 模板 ID
	OnlyTemplate  bool     `json:"only_template"`         // 是否只分享模板（不含数据）
	AutoBackup    bool     `json:"auto_backup"`           // 分享前是否自动备份
	CategoryId    uint     `json:"category_id"`           // 模板分类ID（36=外贸模板，37=中文模板，38=免费模板）
	Name          string   `json:"name"`                  // 模板名称
	Price         int64    `json:"price"`                 // 模板售价(单位：分)
	Author        string   `json:"author"`                // 作者
	Package       string   `json:"package"`               // 模板包名
	Version       string   `json:"version"`               // 模板版本
	Description   string   `json:"description"`           // 模板简介
	Homepage      string   `json:"homepage"`              // 作者主页
	TemplateType  int      `json:"template_type" ast:"-"` // 模板类型：0=自适应，1=代码适配，2=电脑+手机
	PCThumb       string   `json:"pc_thumb"`              // PC端预览效果图(URL,通过上传附件获得)
	MobileThumb   string   `json:"mobile_thumb"`          // 移动端预览效果图(URL,通过上传附件获得)
	Content       string   `json:"content"`               // 模板完整介绍
	PreviewImages []string `json:"preview_images"`        // 模板预览图片列表
	TemplatePath  string   `json:"template_path" ast:"-"`
}

type AnqiDownloadTemplateRequest struct {
	TemplateId uint `json:"template_id"` // 模板 ID
}

// AnqiFeedbackRequest 向安企云提交使用反馈的结构体
type AnqiFeedbackRequest struct {
	Title    string   `json:"title"`   // 反馈标题
	Type     string   `json:"type"`    // 反馈类型：bug|suggest|consult
	Content  string   `json:"content"` // 反馈内容
	Domain   string   `json:"domain"`  // 联系方式
	Version  string   `json:"version" ast:"-"`
	Platform string   `json:"platform" ast:"-"`
	Images   []string `json:"images"` // 截图地址列表（通过上传附件获得）
}

type AnqiExtractRequest struct {
	Text string `json:"text"` // 需要提取关键词/摘要的文本
	Num  int    `json:"num"`  // 提取的关键词数量/摘要长度
}

type TranslateArticleRequest struct {
	Id         int64  `json:"id"`          // 文档 ID
	ToLanguage string `json:"to_language"` // 目标语言，如：en
}

type AiPseudoArticleRequest struct {
	Id int64 `json:"id"` // 文档 ID
}

type AnqiImageAiRequest struct {
	Image  string `json:"image"`  // 图生图的时候提供，格式：仅支持其中一种方式：- 通过图片 URL 传入远程图像（字符串，格式为 URI）- 通过base64传输图像，格式为 base64 编码的字符串
	Prompt string `json:"prompt"` // 生图提示词
	Size   string `json:"size"`   // 生图尺寸，如："1024x1024"
	Type   int    `json:"type"`   // 0 = 文生图，2 = 图生图
}
