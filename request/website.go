package request

import "kandaoni.com/anqicms/config"

type WebsiteRequest struct {
	Id            uint               `json:"id,omitempty"`   // 站点ID
	RootPath      string             `json:"root_path"`      // 站点根目录
	Name          string             `json:"name"`           // 站点名称
	Status        uint               `json:"status"`         // 站点状态
	Mysql         config.MysqlConfig `json:"mysql"`          // 站点数据库配置。
	AdminUser     string             `json:"admin_user"`     // 管理员用户名
	AdminPassword string             `json:"admin_password"` // 管理员密码，新建站点必填。
	BaseUrl       string             `json:"base_url"`       // 站点网址
	FrontUrl      string             `json:"front_url"`      // 前台网址（前后端分离的站点可配置）
	PreviewData   bool               `json:"preview_data"`   // 创建站点时是否导入模板数据
	Initialed     bool               `json:"initialed" ast:"-"`
	Template      string             `json:"template"` // 站点模板包名，默认 default
}

type WebsiteDeleteRequest struct {
	Id         uint `json:"id" validate:"required"` // 站点ID
	RemoveFile bool `json:"remove_file,omitempty"`  // 删除站点时是否删除文件
}

// WebsiteDetailResponse 是**读接口专用**的站点详情结构体。
//
// 为什么不复用 WebsiteRequest：那个是**写请求**结构体，带 AdminPassword
// 这类只应在输入方向出现的字段，且 json tag 没有 omitempty——于是即便值是空串，
// 读接口 GetWebsiteInfo 返回时也会带上 "admin_password": ""。
//
// 虽然目前是空串（密码加密存储、不会明文回传），但让读接口带着 password 字段
// 本身就是坏设计：
//   - 调用方会误以为那里有可用的凭据；
//   - 将来任何一处给它赋上值都会变成明文泄露。
//
// 读接口只声明真正要输出的字段，从结构上杜绝这类误用。
type WebsiteDetailResponse struct {
	Id        uint                   `json:"id,omitempty"` // 站点ID
	RootPath  string                 `json:"root_path"`  // 站点根目录
	Name      string                 `json:"name"`       // 站点名称
	Status    uint                   `json:"status"`     // 站点状态
	Mysql     WebsiteMysqlSafe       `json:"mysql"`      // 站点数据库配置（密码已脱敏）
	AdminUser string                 `json:"admin_user"` // 管理员用户名（不含密码）
	BaseUrl   string                 `json:"base_url"`   // 站点网址
	FrontUrl  string                 `json:"front_url"`  // 前台网址（前后端分离的站点可配置）
	Initialed bool                   `json:"initialed"`  // 站点是否已初始化完成
}

// WebsiteMysqlSafe 是**不含密码**的数据库配置，专门用于读接口。
//
// 为什么必须脱敏：MysqlConfig.Password 是数据库账号密码，而
// GetWebsiteInfo 此前直接返回整个 MysqlConfig——实测 MCP 侧
// siteops_website/detail 能读到明文密码（如 "password": "root"）。
// 任何能调后台接口的人都因此拿到了数据库凭据，可直接连库读写一切。
//
// 保留 database/user/host/port 是必要的：调用方要靠这些判断
// 「这个站点连的是哪个库」，对诊断配置问题有用；只有 password 该藏。
type WebsiteMysqlSafe struct {
	Database string `json:"database"`    // 数据库名称
	User     string `json:"user"`        // 数据库用户名
	Host     string `json:"host"`        // 数据库地址
	Port     int    `json:"port"`        // 数据库端口
	// Password 固定输出掩码而非真实值，语义与 pluginSendmail 的授权码掩码一致。
	// 注意：仅脱敏不够，写端点还须识别该掩码并沿用内存真值（回写保护），
	// 否则有人用读到的详情原样保存会把掩码写进库、导致站点连不上数据库。
	Password string `json:"password"` // 数据库密码（已脱敏）
	// UseDefault 表示复用主站点的数据库账号密码，为 true 时本就无独立密码可返回。
	UseDefault bool `json:"use_default"`
}

// MysqlPasswordMask 是数据库密码的掩码占位符。
const MysqlPasswordMask = "********"

// SafeMysql 从完整配置构造脱敏副本。
//
// 密码**恒**输出掩码，包括 UseDefault=true 与本来就空密码的情况：
//   - UseDefault=true：密码复用主站点的，本就没有独立值可返回，
//     但输出空串会被误读成「这个库没设密码」；
//   - 本来就是空：输出空串同样会误导（读方分不清「没密码」与「被脱敏了」）。
// 统一用固定的掩码串，读方一眼就知道「这里本该有值但被藏了」。
func SafeMysql(m config.MysqlConfig) WebsiteMysqlSafe {
	return WebsiteMysqlSafe{
		Database:   m.Database,
		User:       m.User,
		Host:       m.Host,
		Port:       m.Port,
		Password:   MysqlPasswordMask,
		UseDefault: m.UseDefault,
	}
}
