package provider

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"kandaoni.com/anqicms/model"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

func (w *Website) UpdateSitemapTime() error {
	path := w.CachePath + "sitemap-time.log"

	nowTime := fmt.Sprintf("%d", time.Now().Unix())
	err := os.WriteFile(path, []byte(nowTime), 0666)

	if err != nil {
		return err
	}

	return nil
}

func (w *Website) GetSitemapTime() int64 {
	path := w.CachePath + "sitemap-time.log"
	timeBytes, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	timeInt, err := strconv.Atoi(string(timeBytes))
	if err != nil {
		return 0
	}

	return int64(timeInt)
}

// BuildSitemap 手动生成sitemap
func (w *Website) BuildSitemap() error {
	//每一个sitemap包含50000条记录
	//当所有数量少于50000的时候，生成到sitemap.txt文件中
	//如果所有数量多于50000，则按种类生成。
	//sitemap将包含首页、分类首页、文章页、产品页
	baseUrl := w.System.BaseUrl
	var categoryCount int64
	var archiveCount int64
	var tagCount int64
	categoryBuilder := w.DB.Model(&model.Category{}).Where("`status` = 1").Order("id asc").Count(&categoryCount)
	archiveBuilder := w.DB.Model(&model.Archive{}).Order("id asc").Count(&archiveCount)
	tagBuilder := w.DB.Model(&model.Tag{}).Where("`status` = 1").Order("id asc").Count(&tagCount)

	//index 和 category 存放在同一个文件，文章单独一个文件
	indexFile := NewSitemapIndexGenerator(w, fmt.Sprintf("%ssitemap.%s", w.PublicPath, w.PluginSitemap.Type), w.System.BaseUrl, false)
	defer indexFile.Save()

	indexFile.AddIndex(fmt.Sprintf("%s/category.%s", baseUrl, w.PluginSitemap.Type))

	categoryFile := NewSitemapGenerator(w, fmt.Sprintf("%scategory.%s", w.PublicPath, w.PluginSitemap.Type), w.System.BaseUrl, false)
	defer categoryFile.Save()
	//写入首页
	categoryFile.AddLoc(baseUrl, time.Now().Format("2006-01-02"))
	//写入分类页
	var categories []*model.Category
	categoryBuilder.Find(&categories)
	for _, v := range categories {
		categoryFile.AddLoc(w.GetUrl("category", v, 0), time.Unix(v.UpdatedTime, 0).Format("2006-01-02"))
	}
	//写入文章
	pager := int(math.Ceil(float64(archiveCount) / float64(SitemapLimit)))
	var archives []*model.Archive
	lastId := uint(0)
	for i := 1; i <= pager; i++ {
		//写入index
		indexFile.AddIndex(fmt.Sprintf("%s/archive-%d.%s", baseUrl, i, w.PluginSitemap.Type))

		//写入archive-sitemap
		archiveFile := NewSitemapGenerator(w, fmt.Sprintf("%sarchive-%d.%s", w.PublicPath, i, w.PluginSitemap.Type), w.System.BaseUrl, false)
		remainNum := SitemapLimit
		for remainNum > 0 {
			// 单次查询2000条
			archiveBuilder.WithContext(context.Background()).Where("id > ?", lastId).Limit(2000).Find(&archives)
			if len(archives) == 0 {
				break
			}
			for _, v := range archives {
				archiveFile.AddLoc(w.GetUrl("archive", v, 0), time.Unix(v.UpdatedTime, 0).Format("2006-01-02"))
			}
			remainNum -= len(archives)
			lastId = archives[len(archives)-1].Id
		}
		archiveFile.Save()
	}
	//写入tag
	pager = int(math.Ceil(float64(tagCount) / float64(SitemapLimit)))
	var tags []*model.Tag
	lastId = uint(0)
	for i := 1; i <= pager; i++ {
		//写入index
		indexFile.AddIndex(fmt.Sprintf("%s/tag-%d.%s", baseUrl, i, w.PluginSitemap.Type))

		//写入tag-sitemap
		tagFile := NewSitemapGenerator(w, fmt.Sprintf("%stag-%d.%s", w.PublicPath, i, w.PluginSitemap.Type), w.System.BaseUrl, false)
		remainNum := SitemapLimit
		for remainNum > 0 {
			// 单次查询2000条
			tagBuilder.WithContext(context.Background()).Where("id > ?", lastId).Limit(2000).Find(&tags)
			if len(tags) == 0 {
				break
			}
			for _, v := range tags {
				tagFile.AddLoc(w.GetUrl("tag", v, 0), time.Unix(v.UpdatedTime, 0).Format("2006-01-02"))
			}
			remainNum -= len(tags)
			lastId = tags[len(tags)-1].Id
		}
		tagFile.Save()
	}

	_ = w.UpdateSitemapTime()

	return nil
}

// AddonSitemap 追加sitemap
func (w *Website) AddonSitemap(itemType string, link string, lastmod string) error {
	//index 和 category 存放在同一个文件，文章单独一个文件
	if itemType == "category" {
		categoryPath := fmt.Sprintf("%scategory.%s", w.PublicPath, w.PluginSitemap.Type)
		_, err := os.Stat(categoryPath)
		if err != nil {
			if os.IsNotExist(err) {
				return w.BuildSitemap()
			} else {
				return err
			}
		}

		categoryFile := NewSitemapGenerator(w, categoryPath, w.System.BaseUrl, true)
		if err != nil {
			return err
		}
		defer categoryFile.Save()
		//写入分类页
		categoryFile.AddLoc(link, lastmod)

		if err == nil {
			_ = w.UpdateSitemapTime()
		}
	} else if itemType == "archive" {
		var archiveCount int64
		w.DB.Model(&model.Archive{}).Count(&archiveCount)
		//文章，由于本次统计的时候，这个文章已经存在，可以直接使用统计数量
		pager := int(math.Ceil(float64(archiveCount) / float64(SitemapLimit)))
		archivePath := fmt.Sprintf("%sarchive-%d.%s", w.PublicPath, pager, w.PluginSitemap.Type)
		_, err := os.Stat(archivePath)
		if err != nil {
			if os.IsNotExist(err) {
				return w.BuildSitemap()
			} else {
				return err
			}
		}
		archiveFile := NewSitemapGenerator(w, archivePath, w.System.BaseUrl, true)
		defer archiveFile.Save()
		archiveFile.AddLoc(link, lastmod)

		if err == nil {
			_ = w.UpdateSitemapTime()
		}
	} else if itemType == "tag" {
		var tagCount int64
		w.DB.Model(&model.Tag{}).Where("`status` = 1").Count(&tagCount)
		//tag
		pager := int(math.Ceil(float64(tagCount) / float64(SitemapLimit)))
		tagPath := fmt.Sprintf("%stag-%d.%s", w.PublicPath, pager, w.PluginSitemap.Type)
		_, err := os.Stat(tagPath)
		if err != nil {
			if os.IsNotExist(err) {
				return w.BuildSitemap()
			} else {
				return err
			}
		}
		tagFile := NewSitemapGenerator(w, tagPath, w.System.BaseUrl, true)
		defer tagFile.Save()
		tagFile.AddLoc(link, lastmod)

		_ = w.UpdateSitemapTime()
	}

	return nil
}

type SitemapUrl struct {
	Loc        string `xml:"loc"`
	Lastmod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type SitemapGenerator struct {
	XMLName  xml.Name     `xml:"urlset"`
	Xmlns    string       `xml:"xmlns,attr"`
	Urls     []SitemapUrl `xml:"url"`
	Type     string       `xml:"-"`
	FilePath string       `xml:"-"`
	BaseUrl  string       `xml:"-"`
	w        *Website
}

type SitemapIndexGenerator struct {
	XMLName  xml.Name     `xml:"sitemapindex"`
	Xmlns    string       `xml:"xmlns,attr"`
	Type     string       `xml:"-"`
	Sitemaps []SitemapUrl `xml:"sitemap"`
	FilePath string       `xml:"-"`
	BaseUrl  string       `xml:"-"`
	w        *Website
}

func NewSitemapGenerator(w *Website, filePath, baseUrl string, load bool) *SitemapGenerator {
	generator := &SitemapGenerator{
		w:        w,
		Type:     w.PluginSitemap.Type,
		FilePath: filePath,
		BaseUrl:  baseUrl,
		Xmlns:    "http://www.sitemaps.org/schemas/sitemap/0.9",
	}
	if load {
		generator.Load()
	}

	return generator
}

func (g *SitemapGenerator) Load() {
	if g.Type == "xml" {
		data, err := os.ReadFile(g.FilePath)
		if err == nil {
			err = xml.Unmarshal(data, g)
			if err == nil {
				// nothing to do
			}
		}
	} else {
		data, err := os.ReadFile(g.FilePath)
		if err == nil {
			links := strings.Split(string(bytes.TrimSpace(data)), "\r\n")
			g.Urls = make([]SitemapUrl, 0, len(links))
			for i := range links {
				g.Urls = append(g.Urls, SitemapUrl{Loc: links[i]})
			}
		}
	}
}

func (g *SitemapGenerator) AddLoc(loc string, lastMod string) {
	g.Urls = append(g.Urls, SitemapUrl{
		Loc:     loc,
		Lastmod: lastMod,
		//ChangeFreq: "daily",
		//Priority:   "0.8",
	})
}

func (g *SitemapGenerator) Exists(link string) bool {
	for i := range g.Urls {
		if g.Urls[i].Loc == link {
			return true
		}
	}

	return false
}

func (g *SitemapGenerator) Save() error {
	if g.Type == "xml" {
		output, err := xml.Marshal(g)
		if err == nil {
			f, err := os.OpenFile(g.FilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			if err != nil {
				return err
			}
			_, err = f.WriteString(xml.Header)
			_, err = f.WriteString("<?xml-stylesheet type=\"text/xsl\" href=\"" + g.BaseUrl + "/anqi-style.xsl\" ?>\n")
			_, err = f.Write(output)
			if err1 := f.Close(); err1 != nil && err == nil {
				err = err1
			}
			// 上传到静态服务器
			remotePath := strings.TrimPrefix(g.FilePath, g.w.PublicPath)
			_ = g.w.SyncHtmlCacheToStorage(g.FilePath, remotePath)
			return err
		}

		return err
	} else {
		var links = make([]string, 0, len(g.Urls))
		for i := range g.Urls {
			links = append(links, g.Urls[i].Loc)
		}
		err := os.WriteFile(g.FilePath, []byte(strings.Join(links, "\r\n")), os.ModePerm)
		// 上传到静态服务器
		remotePath := strings.TrimPrefix(g.FilePath, g.w.PublicPath)
		_ = g.w.SyncHtmlCacheToStorage(g.FilePath, remotePath)
		return err
	}
}

func NewSitemapIndexGenerator(w *Website, filePath, baseUrl string, load bool) *SitemapIndexGenerator {
	generator := &SitemapIndexGenerator{
		w:        w,
		Type:     w.PluginSitemap.Type,
		Xmlns:    "http://www.sitemaps.org/schemas/sitemap/0.9",
		FilePath: filePath,
		BaseUrl:  baseUrl,
	}
	if load {
		generator.Load()
	}

	return generator
}

func (s *SitemapIndexGenerator) Load() {
	if s.Type == "xml" {
		data, err := os.ReadFile(s.FilePath)
		if err == nil {
			err = xml.Unmarshal(data, s)
			if err == nil {
				// nothing to do
			}
		}
	} else {
		data, err := os.ReadFile(s.FilePath)
		if err == nil {
			links := strings.Split(string(bytes.TrimSpace(data)), "\r\n")
			s.Sitemaps = make([]SitemapUrl, 0, len(links))
			for i := range links {
				s.Sitemaps = append(s.Sitemaps, SitemapUrl{Loc: links[i]})
			}
		}
	}
}

func (s *SitemapIndexGenerator) AddIndex(loc string) {
	s.Sitemaps = append(s.Sitemaps, SitemapUrl{
		Loc: loc,
	})
}

func (s *SitemapIndexGenerator) Exists(link string) bool {
	for i := range s.Sitemaps {
		if s.Sitemaps[i].Loc == link {
			return true
		}
	}

	return false
}

func (s *SitemapIndexGenerator) Save() error {
	if s.Type == "xml" {
		output, err := xml.Marshal(s)
		if err == nil {
			f, err := os.OpenFile(s.FilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			if err != nil {
				return err
			}
			_, err = f.WriteString(xml.Header)
			_, err = f.WriteString("<?xml-stylesheet type=\"text/xsl\" href=\"" + s.BaseUrl + "/anqi-index.xsl\" ?>\n")
			_, err = f.Write(output)
			if err1 := f.Close(); err1 != nil && err == nil {
				err = err1
			}
			// 上传到静态服务器
			remotePath := strings.TrimPrefix(s.FilePath, s.w.PublicPath)
			_ = s.w.SyncHtmlCacheToStorage(s.FilePath, remotePath)
			return err
		}

		return err
	} else {
		var links = make([]string, 0, len(s.Sitemaps))
		for i := range s.Sitemaps {
			links = append(links, s.Sitemaps[i].Loc)
		}
		err := os.WriteFile(s.FilePath, []byte(strings.Join(links, "\r\n")), os.ModePerm)
		// 上传到静态服务器
		remotePath := strings.TrimPrefix(s.FilePath, s.w.PublicPath)
		_ = s.w.SyncHtmlCacheToStorage(s.FilePath, remotePath)
		return err
	}
}
