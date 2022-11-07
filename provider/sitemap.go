package provider

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

func UpdateSitemapTime() error {
	path := fmt.Sprintf("%scache/sitemap-time.log", config.ExecPath)

	nowTime := fmt.Sprintf("%d", time.Now().Unix())
	err := os.WriteFile(path, []byte(nowTime), 0666)

	if err != nil {
		return err
	}

	return nil
}

func GetSitemapTime() int64 {
	path := fmt.Sprintf("%scache/sitemap-time.log", config.ExecPath)
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
func BuildSitemap() error {
	//每一个sitemap包含50000条记录
	//当所有数量少于50000的时候，生成到sitemap.txt文件中
	//如果所有数量多于50000，则按种类生成。
	//sitemap将包含首页、分类首页、文章页、产品页
	basePath := fmt.Sprintf("%spublic/", config.ExecPath)
	baseUrl := config.JsonData.System.BaseUrl
	var categoryCount int64
	var archiveCount int64
	var tagCount int64
	categoryBuilder := dao.DB.Model(&model.Category{}).Where("`status` = 1").Order("id asc").Count(&categoryCount)
	archiveBuilder := dao.DB.Model(&model.Archive{}).Where("`status` = 1").Order("id asc").Count(&archiveCount)
	tagBuilder := dao.DB.Model(&model.Tag{}).Where("`status` = 1").Order("id asc").Count(&tagCount)

	//index 和 category 存放在同一个文件，文章单独一个文件
	indexFile := NewSitemapIndexGenerator(config.JsonData.PluginSitemap.Type, fmt.Sprintf("%ssitemap.%s", basePath, config.JsonData.PluginSitemap.Type), false)
	defer indexFile.Save()

	indexFile.AddIndex(fmt.Sprintf("%s/category.%s", baseUrl, config.JsonData.PluginSitemap.Type))

	categoryFile := NewSitemapGenerator(config.JsonData.PluginSitemap.Type, fmt.Sprintf("%scategory.%s", basePath, config.JsonData.PluginSitemap.Type), false)
	defer categoryFile.Save()
	//写入首页
	categoryFile.AddLoc(baseUrl, time.Now().Format("2006-01-02"))
	//写入分类页
	var categories []*model.Category
	categoryBuilder.Find(&categories)
	for _, v := range categories {
		categoryFile.AddLoc(GetUrl("category", v, 0), time.Unix(v.UpdatedTime, 0).Format("2006-01-02"))
	}
	//写入文章
	pager := int(math.Ceil(float64(archiveCount) / float64(SitemapLimit)))
	var archives []*model.Archive
	for i := 1; i <= pager; i++ {
		//写入index
		indexFile.AddIndex(fmt.Sprintf("%s/archive-%d.%s", baseUrl, i, config.JsonData.PluginSitemap.Type))

		//写入archive-sitemap
		archiveFile := NewSitemapGenerator(config.JsonData.PluginSitemap.Type, fmt.Sprintf("%sarchive-%d.%s", basePath, i, config.JsonData.PluginSitemap.Type), false)
		err := archiveBuilder.Limit(SitemapLimit).Offset((i - 1) * SitemapLimit).Find(&archives).Error
		if err == nil {
			for _, v := range archives {
				archiveFile.AddLoc(GetUrl("archive", v, 0), time.Unix(v.UpdatedTime, 0).Format("2006-01-02"))
			}
		}
		archiveFile.Save()
	}
	//写入tag
	pager = int(math.Ceil(float64(tagCount) / float64(SitemapLimit)))
	var tags []*model.Tag
	for i := 1; i <= pager; i++ {
		//写入index
		indexFile.AddIndex(fmt.Sprintf("%s/tag-%d.%s", baseUrl, i, config.JsonData.PluginSitemap.Type))

		//写入tag-sitemap
		tagFile := NewSitemapGenerator(config.JsonData.PluginSitemap.Type, fmt.Sprintf("%stag-%d.%s", basePath, i, config.JsonData.PluginSitemap.Type), false)
		err := tagBuilder.Limit(SitemapLimit).Offset((i - 1) * SitemapLimit).Find(&tags).Error
		if err == nil {
			for _, v := range tags {
				tagFile.AddLoc(GetUrl("tag", v, 0), time.Unix(v.UpdatedTime, 0).Format("2006-01-02"))
			}
		}
		tagFile.Save()
	}

	_ = UpdateSitemapTime()

	return nil
}

// AddonSitemap 追加sitemap
func AddonSitemap(itemType string, link string, lastmod string) error {
	basePath := fmt.Sprintf("%spublic/", config.ExecPath)

	//index 和 category 存放在同一个文件，文章单独一个文件
	if itemType == "category" {
		categoryPath := fmt.Sprintf("%scategory.%s", basePath, config.JsonData.PluginSitemap.Type)
		_, err := os.Stat(categoryPath)
		if err != nil {
			if os.IsNotExist(err) {
				return BuildSitemap()
			} else {
				return err
			}
		}

		categoryFile := NewSitemapGenerator(config.JsonData.PluginSitemap.Type, categoryPath, true)
		if err != nil {
			return err
		}
		defer categoryFile.Save()
		//写入分类页
		categoryFile.AddLoc(link, lastmod)

		if err == nil {
			_ = UpdateSitemapTime()
		}
	} else if itemType == "archive" {
		var archiveCount int64
		dao.DB.Model(&model.Archive{}).Where("`status` = 1").Count(&archiveCount)
		//文章，由于本次统计的时候，这个文章已经存在，可以直接使用统计数量
		pager := int(math.Ceil(float64(archiveCount) / float64(SitemapLimit)))
		archivePath := fmt.Sprintf("%sarchive-%d.%s", basePath, pager, config.JsonData.PluginSitemap.Type)
		_, err := os.Stat(archivePath)
		if err != nil {
			if os.IsNotExist(err) {
				return BuildSitemap()
			} else {
				return err
			}
		}
		archiveFile := NewSitemapGenerator(config.JsonData.PluginSitemap.Type, archivePath, true)
		defer archiveFile.Save()
		archiveFile.AddLoc(link, lastmod)

		if err == nil {
			_ = UpdateSitemapTime()
		}
	} else if itemType == "tag" {
		var tagCount int64
		dao.DB.Model(&model.Tag{}).Where("`status` = 1").Count(&tagCount)
		//tag
		pager := int(math.Ceil(float64(tagCount) / float64(SitemapLimit)))
		tagPath := fmt.Sprintf("%stag-%d.%s", basePath, pager, config.JsonData.PluginSitemap.Type)
		_, err := os.Stat(tagPath)
		if err != nil {
			if os.IsNotExist(err) {
				return BuildSitemap()
			} else {
				return err
			}
		}
		tagFile := NewSitemapGenerator(config.JsonData.PluginSitemap.Type, tagPath, true)
		defer tagFile.Save()
		tagFile.AddLoc(link, lastmod)

		_ = UpdateSitemapTime()
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
}

type SitemapIndexGenerator struct {
	XMLName  xml.Name     `xml:"sitemapindex"`
	Xmlns    string       `xml:"xmlns,attr"`
	Type     string       `xml:"-"`
	Sitemaps []SitemapUrl `xml:"sitemap"`
	FilePath string       `xml:"-"`
}

func NewSitemapGenerator(sitemapType string, filePath string, load bool) *SitemapGenerator {
	generator := &SitemapGenerator{
		Type:     sitemapType,
		FilePath: filePath,
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
			_, err = f.WriteString("<?xml-stylesheet type=\"text/xsl\" href=\"" + config.JsonData.System.BaseUrl + "/anqi-style.xsl\" ?>\n")
			_, err = f.Write(output)
			if err1 := f.Close(); err1 != nil && err == nil {
				err = err1
			}
			return err
		}

		return err
	} else {
		var links = make([]string, 0, len(g.Urls))
		for i := range g.Urls {
			links = append(links, g.Urls[i].Loc)
		}
		err := os.WriteFile(g.FilePath, []byte(strings.Join(links, "\r\n")), os.ModePerm)

		return err
	}
}

func NewSitemapIndexGenerator(sitemapType string, filePath string, load bool) *SitemapIndexGenerator {
	generator := &SitemapIndexGenerator{
		Type:     sitemapType,
		Xmlns:    "http://www.sitemaps.org/schemas/sitemap/0.9",
		FilePath: filePath,
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
			_, err = f.WriteString("<?xml-stylesheet type=\"text/xsl\" href=\"" + config.JsonData.System.BaseUrl + "/anqi-index.xsl\" ?>\n")
			_, err = f.Write(output)
			if err1 := f.Close(); err1 != nil && err == nil {
				err = err1
			}
			return err
		}

		return err
	} else {
		var links = make([]string, 0, len(s.Sitemaps))
		for i := range s.Sitemaps {
			links = append(links, s.Sitemaps[i].Loc)
		}
		err := os.WriteFile(s.FilePath, []byte(strings.Join(links, "\r\n")), os.ModePerm)

		return err
	}
}
