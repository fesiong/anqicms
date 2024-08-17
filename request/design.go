package request

type DesignInfoRequest struct {
	Name         string `json:"name"`
	Package      string `json:"package"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	Author       string `json:"author"`
	Homepage     string `json:"homepage"`
	Created      string `json:"created"`
	TemplateType int    `json:"template_type"`
}

type RestoreDesignFileRequest struct {
	Hash     string `json:"hash"`
	Package  string `json:"package"`
	Filepath string `json:"path"`
	Type     string `json:"type"`
}

type SaveDesignFileRequest struct {
	Package       string `json:"package"`
	Path          string `json:"path"`
	Type          string `json:"type"`
	RenamePath    string `json:"rename_path"`
	Content       string `json:"content"`
	Remark        string `json:"remark"`
	UpdateContent bool   `json:"update_content"`
}

type CopyDesignFileRequest struct {
	Package string `json:"package"`
	Path    string `json:"path"`
	NewPath string `json:"new_path"`
	Type    string `json:"type"`
	Remark  string `json:"remark"`
}

type DesignDataRequest struct {
	Package     string `json:"package"`
	AutoBackup  bool   `json:"auto_backup"`
	AutoCleanup bool   `json:"auto_cleanup"`
}
