package provider

import (
	"fmt"
	"io/ioutil"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"mime/multipart"
	"net/url"
	"strings"
)

func GetRedirectList(keyword string, currentPage, pageSize int) ([]*model.Redirect, int64, error) {
	var redirects []*model.Redirect
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := dao.DB.Model(&model.Redirect{}).Order("id desc")
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

func GetRedirectById(id uint) (*model.Redirect, error) {
	var redirect model.Redirect

	err := dao.DB.Where("`id` = ?", id).First(&redirect).Error
	if err != nil {
		return nil, err
	}

	return &redirect, nil
}

func GetRedirectByFromUrl(fromUrl string) (*model.Redirect, error) {
	var redirect model.Redirect

	err := dao.DB.Where("`from_url` = ?", fromUrl).First(&redirect).Error
	if err != nil {
		return nil, err
	}

	return &redirect, nil
}

func ImportRedirects(file multipart.File, info *multipart.FileHeader) (string, error) {
	buff, err := ioutil.ReadAll(file)
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
		redirect, err := GetRedirectByFromUrl(fromUrl)
		if err != nil {
			//表示不存在
			redirect = &model.Redirect{
				FromUrl: fromUrl,
			}
			total++
		}
		redirect.ToUrl = toUrl
		dao.DB.Save(redirect)
	}

	return fmt.Sprintf(config.Lang("成功导入了%d个链接"), total), nil
}

func DeleteRedirect(redirect *model.Redirect) error {
	err := dao.DB.Delete(redirect).Error
	if err != nil {
		return err
	}

	return nil
}

func DeleteCacheRedirects() {
	library.MemCache.Delete("redirects")
}

func GetCacheRedirects() map[string]string {
	if dao.DB == nil {
		return nil
	}
	var redirects = map[string]string{}
	result := library.MemCache.Get("redirects")
	if result != nil {
		var ok bool
		redirects, ok = result.(map[string]string)
		if ok {
			return redirects
		}
	}

	baseUrl, err := url.Parse(config.JsonData.System.BaseUrl)
	if err == nil {
		baseUrl.Host = "127.0.0.1:8001"
	}

	var tmpData []model.Redirect
	dao.DB.Where(model.Redirect{}).Find(&tmpData)
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

	library.MemCache.Set("redirects", redirects, 0)

	return redirects
}

func GetRedirectFromCache(fromUrl string) string {
	redirects := GetCacheRedirects()

	if val, ok := redirects[fromUrl]; ok {
		return val
	}
	return ""
}
