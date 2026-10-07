package request

type DesignInfoRequest struct {
	Name         string `json:"name"`          // 模板名称
	Package      string `json:"package"`       // 模板包名
	Version      string `json:"version"`       // 模板版本
	Description  string `json:"description"`   // 模板描述
	Author       string `json:"author"`        // 模板作者
	Homepage     string `json:"homepage"`      // 模板主页
	Created      string `json:"created"`       // 模板创建时间
	TemplateType int    `json:"template_type"` // 模板类型：0=自适应，1=代码适配，2=电脑+手机
}

type UseDesignRequest struct {
	Package string `json:"package"` // 模板包名
}

type RestoreDesignFileRequest struct {
	Hash     string `json:"hash"`    // 文件哈希
	Package  string `json:"package"` // 模板包名
	Filepath string `json:"path"`    // 文件路径
	Type     string `json:"type"`    // 文件类型：static|template。
}

type SaveDesignFileRequest struct {
	Package       string `json:"package"`        // 模板包名
	Path          string `json:"path"`           // 文件路径
	Type          string `json:"type"`           // 文件类型：static|template。
	RenamePath    string `json:"rename_path"`    // 重命名路径
	Content       string `json:"content"`        // 文件内容
	Remark        string `json:"remark"`         // 文件备注
	UpdateContent bool   `json:"update_content"` // 是否更新文件内容，如果为 true 则仅更新内容
}

type DeleteDesignFileRequest struct {
	Package string `json:"package"` // 模板包名
	Path    string `json:"path"`    // 文件路径
	Type    string `json:"type"`    // 文件类型：static|template。
}

type CopyDesignFileRequest struct {
	Package string `json:"package"`  // 模板包名
	Path    string `json:"path"`     // 文件路径
	NewPath string `json:"new_path"` // 新文件路径
	Type    string `json:"type"`     // 文件类型：static|template。
	Remark  string `json:"remark"`   // 文件备注
}

type DesignDataRequest struct {
	Package     string `json:"package"`      // 模板包名
	AutoBackup  bool   `json:"auto_backup"`  // 是否自动备份网站数据。
	AutoCleanup bool   `json:"auto_cleanup"` // 是否一键清空网站数据（为 true 时会先执行备份）。
}
