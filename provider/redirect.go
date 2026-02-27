package provider

import (
	"fmt"
	"io"
	"kandaoni.com/anqicms/model"
	"mime/multipart"
	"net/url"
	"strings"
)

func (w *Website) GetRedirectList(keyword string, currentPage, pageSize int) ([]*model.Redirect, int64, error) {
	var redirects []*model.Redirect
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := w.DB.Model(&model.Redirect{}).Order("id desc")
	if keyword != "" {
		//模糊搜索
		builder = builder.Where("(`from_url` like ?)", "%"+keyword+"%")
	}

	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&redirects).Error
	if err != nil {
		return nil, 0, err
	}

	return redirects, total, nil
}

func (w *Website) GetRedirectById(id uint) (*model.Redirect, error) {
	var redirect model.Redirect

	err := w.DB.Where("`id` = ?", id).First(&redirect).Error
	if err != nil {
		return nil, err
	}

	return &redirect, nil
}

func (w *Website) GetRedirectByFromUrl(fromUrl string) (*model.Redirect, error) {
	var redirect model.Redirect

	err := w.DB.Where("`from_url` = ?", fromUrl).First(&redirect).Error
	if err != nil {
		return nil, err
	}

	return &redirect, nil
}

func (w *Website) ImportRedirects(file multipart.File, info *multipart.FileHeader) (string, error) {
	buff, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(buff), "\n")
	var total int
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// 格式：from_url, to_url
		if i == 0 {
			continue
		}
		values := strings.Split(line, ",")
		if len(values) < 2 {
			continue
		}
		fromUrl := strings.TrimSpace(values[0])
		toUrl := strings.TrimSpace(values[1])
		if fromUrl == "" || fromUrl == "/" {
			continue
		}
		if !strings.HasPrefix(fromUrl, "http") && !strings.HasPrefix(fromUrl, "/") {
			fromUrl = "/" + fromUrl
		}
		if !strings.HasPrefix(toUrl, "http") && !strings.HasPrefix(toUrl, "/") {
			toUrl = "/" + toUrl
		}
		if fromUrl == toUrl {
			continue
		}
		redirect, err := w.GetRedirectByFromUrl(fromUrl)
		if err != nil {
			//表示不存在
			redirect = &model.Redirect{
				FromUrl: fromUrl,
			}
			total++
		}
		redirect.ToUrl = toUrl
		w.DB.Save(redirect)
	}

	return fmt.Sprintf(w.Tr("SuccessfullyImportedLinks"), total), nil
}

func (w *Website) DeleteRedirect(redirect *model.Redirect) error {
	err := w.DB.Delete(redirect).Error
	if err != nil {
		return err
	}

	return nil
}

func (w *Website) DeleteCacheRedirects() {
	w.Cache.Delete("redirects")
}

func (w *Website) GetCacheRedirects() map[string]string {
	if w.DB == nil {
		return nil
	}
	var redirects = map[string]string{}
	err := w.Cache.Get("redirects", &redirects)
	if err == nil {
		return redirects
	}

	baseUrl, err := url.Parse(w.System.BaseUrl)
	if err != nil {
		baseUrl, _ = url.Parse("http://127.0.0.1:8001")
	}

	var tmpData []model.Redirect
	w.DB.Where(model.Redirect{}).Find(&tmpData)
	for i := range tmpData {
		fromUrl := tmpData[i].FromUrl
		toUrl := tmpData[i].ToUrl
		if strings.HasPrefix(fromUrl, "http") {
			urlParsed, err := url.Parse(fromUrl)
			if err == nil {
				fromUrl = urlParsed.RequestURI()
			}
		}
		if strings.HasPrefix(toUrl, "http") {
			urlParsed, err := url.Parse(toUrl)
			if err == nil && urlParsed.Host == baseUrl.Host {
				fromUrl = urlParsed.RequestURI()
			}
		}
		redirects[fromUrl] = toUrl
	}

	_ = w.Cache.Set("redirects", redirects, 0)

	return redirects
}

func (w *Website) GetRedirectFromCache(fromUrl string) string {
	redirects := w.GetCacheRedirects()

	if val, ok := redirects[fromUrl]; ok {
		return val
	}
	return ""
}
