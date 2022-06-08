package provider

import (
	"errors"
	"github.com/PuerkitoBio/goquery"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"strings"
	"time"
)

func GetLinkList() ([]*model.Link, error) {
	var links []*model.Link
	db := dao.DB
	err := db.Order("sort asc").Find(&links).Error
	if err != nil {
		return nil, err
	}

	return links, nil
}

func GetLinkById(id uint) (*model.Link, error) {
	var link model.Link
	if err := dao.DB.Where("id = ?", id).First(&link).Error; err != nil {
		return nil, err
	}

	return &link, nil
}

func GetLinkByLink(link string) (*model.Link, error) {
	if link == "" {
		return nil, errors.New("link必填")
	}

	var friendLink model.Link
	var err error
	err = dao.DB.Where("`link` = ?", link).First(&friendLink).Error
	if err != nil {
		// 增加兼容模式查找
		if strings.HasPrefix(link, "https") {
			link = strings.ReplaceAll(link, "https://", "http://")
		} else {
			link = strings.ReplaceAll(link, "http://", "https://")
		}
		err = dao.DB.Where("`link` = ?", link).First(&friendLink).Error
	}


	if err != nil {
		return nil, err
	}

	return &friendLink, nil
}

func PluginLinkCheck(link *model.Link) (*model.Link, error) {
	remoteLink := link.BackLink
	if remoteLink == "" {
		remoteLink = link.Link
	}

	//获取内容
	resp, err := library.Request(remoteLink, nil)
	if err != nil {
		return nil, err
	}

	//检查内容
	htmlR := strings.NewReader(resp.Body)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err != nil {
		return nil, err
	}

	myLink := link.MyLink
	if myLink == "" {
		myLink = config.JsonData.System.BaseUrl
	}

	linkStatus := model.LinkStatusNotMatch
	//获取所有link
	aLinks := doc.Find("a")
	for i := range aLinks.Nodes {
		href, exists := aLinks.Eq(i).Attr("href")
		title := strings.TrimSpace(aLinks.Eq(i).Text())
		rel, relExists := aLinks.Eq(i).Attr("rel")
		if exists {
			if href == myLink || href == myLink + "/" {
				linkStatus = model.LinkStatusOk
				if link.MyTitle != "" && title != link.MyTitle {
					linkStatus = model.LinkStatusNotTitle
				}
				if relExists && rel == "nofollow" {
					linkStatus = model.LinkStatusNofollow
				}

				break
			}
		}
	}

	link.CheckedTime = time.Now().Unix()
	link.Status = linkStatus
	link.Save(dao.DB)

	return link, nil
}
