package provider

import (
	"fmt"
	"net/http"
	"time"

	"kandaoni.com/anqicms/library"
)

var tplHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	// 允许跟随重定向 (download 端点 302 重定向到 COS)
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// 最多跟随 2 次重定向
		if len(via) >= 2 {
			return fmt.Errorf("stopped after 2 redirects")
		}
		return nil
	},
}

// ResolveTemplateFromURL 根据 iframe 当前加载的 URL 反查对应的主模板文件。
func (w *Website) ResolveTemplateFromURL(rawURL string, adminToken string) (string, error) {
	var templateName string
	var tplKey = "template:" + library.Md5(rawURL)
	err := w.Cache.Get(tplKey, &templateName)
	if err == nil {
		return templateName, nil
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", library.GetUserAgent(false))
	req.Header.Set("Admin", adminToken)
	resp, err := tplHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析 header，获取 X-Template-Name
	templateName = resp.Header.Get("X-Template-Name")
	if templateName == "" {
		return "", fmt.Errorf("无法获取模板名称")
	}
	// 将结果缓存 5 分钟
	w.Cache.Set(tplKey, templateName, 5*60)

	return templateName, nil
}
