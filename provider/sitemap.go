package provider

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
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

func (w *Website) DeleteSitemap(sitemapType string) {
	sitemapIndex := NewSitemapIndexGenerator(w, sitemapType, fmt.Sprintf("%ssitemap.%s", w.PublicPath, sitemapType), w.System.BaseUrl, true)
	defer func() {
		_ = os.Remove(sitemapIndex.FilePath)
	}()
	if len(sitemapIndex.Sitemaps) == 0 {
		return
	}
	for _, sitemapUrl := range sitemapIndex.Sitemaps {
		parsed, err := url.Parse(sitemapUrl.Loc)
		if err != nil {
			continue
		}
		sitemapFile := w.PublicPath + strings.TrimPrefix(parsed.Path, "/")
		if err = os.Remove(sitemapFile); err != nil {
			continue
		}
	}
}

// BuildSitemap 手动生成sitemap
func (w *Website) BuildSitemap() error {
	//每一个sitemap包含50000条记录
	//当所有数量少于50000的时候，生成到sitemap.txt文件中
	//如果所有数量多于50000，则按种类生成。
	//sitemap将包含首页、分类首页、文章页、产品页
	baseUrl := w.System.BaseUrl
	multiLang := false
	var multiLangSiteType string
	var multiLangSites []config.MultiLangSite
	if w.PluginSitemap.Type == "xml" {
		mainId := w.ParentId
		if mainId == 0 {
			mainId = w.Id
		}
		mainSite := GetWebsite(mainId)
		if mainSite.MultiLanguage.Open == true {
			multiLang = true
			multiLangSiteType = mainSite.MultiLanguage.SiteType
			multiLangSites = w.GetMultiLangSites(mainId, false)
		}
	}

	categoryBuilder := w.DB.Model(&model.Category{}).Where("`status` = 1").Order("id asc").Select("id", "updated_time", "type", "module_id", "url_token")
	archiveBuilder := w.DB.Model(&model.Archive{}).Order("id asc").Select("id", "created_time", "updated_time", "url_token", "module_id", "category_id", "fixed_link")
	tagBuilder := w.DB.Model(&model.Tag{}).Where("`status` = 1").Order("id asc").Select("id", "updated_time", "url_token")
	if len(w.PluginSitemap.ExcludeCategoryIds) > 0 || len(w.PluginSitemap.ExcludePageIds) > 0 {
		excludeIds := make([]uint, 0)
		excludeIds = append(excludeIds, w.PluginSitemap.ExcludeCategoryIds...)
		excludeIds = append(excludeIds, w.PluginSitemap.ExcludePageIds...)
		categoryBuilder = categoryBuilder.Where("id not in (?)", excludeIds)
		archiveBuilder = archiveBuilder.Where("category_id not in (?)", excludeIds)
	}
	if len(w.PluginSitemap.ExcludeModuleIds) > 0 {
		categoryBuilder = categoryBuilder.Where("module_id not in (?)", w.PluginSitemap.ExcludeModuleIds)
		archiveBuilder = archiveBuilder.Where("module_id not in (?)", w.PluginSitemap.ExcludeModuleIds)
	}

	//index 和 category 存放在同一个文件，文章单独一个文件
	indexFile := NewSitemapIndexGenerator(w, w.PluginSitemap.Type, fmt.Sprintf("%ssitemap.%s", w.PublicPath, w.PluginSitemap.Type), w.System.BaseUrl, false)
	defer indexFile.Save()

	indexFile.AddIndex(fmt.Sprintf("%s/category.%s", baseUrl, w.PluginSitemap.Type))

	categoryFile := NewSitemapGenerator(w, fmt.Sprintf("%scategory.%s", w.PublicPath, w.PluginSitemap.Type), w.System.BaseUrl, false)
	defer categoryFile.Save()
	//写入首页
	var alternates []AlternateLink
	if multiLang {
		for idx := range multiLangSites {
			var alternate AlternateLink
			if multiLangSiteType == config.MultiLangSiteTypeMulti {
				site := GetWebsite(multiLangSites[idx].Id)
				if site == nil {
					continue
				}
				if site.Id == w.Id {
					continue
				}
				alternate = AlternateLink{
					Href:     site.System.BaseUrl,
					Hreflang: site.System.Language,
				}
			} else {
				// single
				link := w.System.BaseUrl + "/"
				link = w.MultiLanguage.GetUrl(link, w.System.BaseUrl, &multiLangSites[idx])
				alternate = AlternateLink{
					Href:     link,
					Hreflang: multiLangSites[idx].Language,
				}
			}
			alternates = append(alternates, alternate)
		}
	}
	categoryFile.AddLoc(baseUrl, time.Now().Format("2006-01-02"), alternates)
	//写入分类页
	var categories []*model.Category
	categoryBuilder.Find(&categories)
	for _, v := range categories {
		defaultLink := w.GetUrl("category", v, 0)
		alternates = alternates[:0]
		if multiLang {
			for idx := range multiLangSites {
				var alternate AlternateLink
				if multiLangSiteType == config.MultiLangSiteTypeMulti {
					site := GetWebsite(multiLangSites[idx].Id)
					if site == nil {
						continue
					}
					if site.Id == w.Id {
						continue
					}
					alternate = AlternateLink{
						Href:     site.GetUrl("category", v, 0),
						Hreflang: site.System.Language,
					}
				} else {
					// single
					alternate = AlternateLink{
						Href:     w.MultiLanguage.GetUrl(defaultLink, w.System.BaseUrl, &multiLangSites[idx]),
						Hreflang: multiLangSites[idx].Language,
					}
				}
				alternates = append(alternates, alternate)
			}
		}
		categoryFile.AddLoc(defaultLink, time.Unix(v.UpdatedTime, 0).Format("2006-01-02"), alternates)
	}
	//写入文章
	var archives []*model.Archive
	lastId := int64(0)
	page := 0
	for {
		// 每次加1，累计将生成的页码
		page++
		//写入index
		indexFile.AddIndex(fmt.Sprintf("%s/archive-%d.%s", baseUrl, page, w.PluginSitemap.Type))

		//写入archive-sitemap
		archiveFile := NewSitemapGenerator(w, fmt.Sprintf("%sarchive-%d.%s", w.PublicPath, page, w.PluginSitemap.Type), w.System.BaseUrl, false)
		remainNum := SitemapLimit
		finished := false
		for remainNum > 0 {
			// 单次查询2000条
			archiveBuilder.WithContext(context.Background()).Where("id > ?", lastId).Limit(2000).Find(&archives)
			if len(archives) == 0 {
				finished = true
				break
			}
			for _, v := range archives {
				defaultLink := w.GetUrl("archive", v, 0)
				alternates = alternates[:0]
				if multiLang {
					for idx := range multiLangSites {
						var alternate AlternateLink
						if multiLangSiteType == config.MultiLangSiteTypeMulti {
							site := GetWebsite(multiLangSites[idx].Id)
							if site == nil {
								continue
							}
							if site.Id == w.Id {
								continue
							}
							alternate = AlternateLink{
								Href:     site.GetUrl("archive", v, 0),
								Hreflang: site.System.Language,
							}
						} else {
							// single
							alternate = AlternateLink{
								Href:     w.MultiLanguage.GetUrl(defaultLink, w.System.BaseUrl, &multiLangSites[idx]),
								Hreflang: multiLangSites[idx].Language,
							}
						}
						alternates = append(alternates, alternate)
					}
				}
				archiveFile.AddLoc(defaultLink, time.Unix(v.UpdatedTime, 0).Format("2006-01-02"), alternates)
			}
			remainNum -= len(archives)
			lastId = archives[len(archives)-1].Id
		}
		_ = archiveFile.Save()
		if finished {
			break
		}
	}
	//写入tag
	if w.PluginSitemap.ExcludeTag == false {
		page = 0
		var tags []*model.Tag
		lastId = int64(0)
		for {
			page++
			//写入index
			indexFile.AddIndex(fmt.Sprintf("%s/tag-%d.%s", baseUrl, page, w.PluginSitemap.Type))

			//写入tag-sitemap
			tagFile := NewSitemapGenerator(w, fmt.Sprintf("%stag-%d.%s", w.PublicPath, page, w.PluginSitemap.Type), w.System.BaseUrl, false)
			remainNum := SitemapLimit
			finished := false
			for remainNum > 0 {
				// 单次查询5000条
				tagBuilder.WithContext(context.Background()).Where("id > ?", lastId).Limit(5000).Find(&tags)
				if len(tags) == 0 {
					finished = true
					break
				}
				for _, v := range tags {
					defaultLink := w.GetUrl("tag", v, 0)
					alternates = alternates[:0]
					if multiLang {
						for idx := range multiLangSites {
							var alternate AlternateLink
							if multiLangSiteType == config.MultiLangSiteTypeMulti {
								site := GetWebsite(multiLangSites[idx].Id)
								if site == nil {
									continue
								}
								if site.Id == w.Id {
									continue
								}
								alternate = AlternateLink{
									Href:     site.GetUrl("tag", v, 0),
									Hreflang: site.System.Language,
								}
							} else {
								// single
								alternate = AlternateLink{
									Href:     w.MultiLanguage.GetUrl(defaultLink, w.System.BaseUrl, &multiLangSites[idx]),
									Hreflang: multiLangSites[idx].Language,
								}
							}
							alternates = append(alternates, alternate)
						}
					}
					tagFile.AddLoc(defaultLink, time.Unix(v.UpdatedTime, 0).Format("2006-01-02"), alternates)
				}
				remainNum -= len(tags)
				lastId = int64(tags[len(tags)-1].Id)
			}
			_ = tagFile.Save()
			if finished {
				break
			}
		}
	}

	_ = w.UpdateSitemapTime()

	return nil
}

// AddonSitemap 追加sitemap
func (w *Website) AddonSitemap(itemType string, link string, lastmod string, data interface{}) error {
	multiLang := false
	var multiLangSiteType string
	var multiLangSites []config.MultiLangSite
	if w.PluginSitemap.Type == "xml" {
		mainId := w.ParentId
		if mainId == 0 {
			mainId = w.Id
		}
		mainSite := GetWebsite(mainId)
		if mainSite.MultiLanguage.Open == true {
			multiLang = true
			multiLangSiteType = mainSite.MultiLanguage.SiteType
			multiLangSites = w.GetMultiLangSites(mainId, false)
		}
	}

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
		defer categoryFile.Save()
		//写入分类页
		var alternates []AlternateLink
		if multiLang {
			item, ok := data.(*model.Category)
			if ok {
				for idx := range multiLangSites {
					var alternate AlternateLink
					if multiLangSiteType == config.MultiLangSiteTypeMulti {
						site := GetWebsite(multiLangSites[idx].Id)
						if site == nil {
							continue
						}
						if site.Id == w.Id {
							continue
						}
						alternate = AlternateLink{
							Href:     site.GetUrl("category", item, 0),
							Hreflang: site.System.Language,
						}
					} else {
						// single
						alternate = AlternateLink{
							Href:     w.MultiLanguage.GetUrl(link, w.System.BaseUrl, &multiLangSites[idx]),
							Hreflang: multiLangSites[idx].Language,
						}
					}
					alternates = append(alternates, alternate)
				}
			}
		}
		categoryFile.AddLoc(link, lastmod, alternates)
		_ = w.UpdateSitemapTime()
	} else if itemType == "archive" {
		// 读取SitemapIndex，并找到最后一个
		indexFile := NewSitemapIndexGenerator(w, w.PluginSitemap.Type, fmt.Sprintf("%ssitemap.%s", w.PublicPath, w.PluginSitemap.Type), w.System.BaseUrl, true)
		var latestSitemap string
		for _, v := range indexFile.Sitemaps {
			if strings.Contains(v.Loc, "archive-") {
				latestSitemap = v.Loc
			}
		}
		re, _ := regexp.Compile(`archive-(\d+)`)
		match := re.FindStringSubmatch(latestSitemap)
		if len(match) < 2 {
			// Sitemap不存在。生成一份
			return w.BuildSitemap()
		}
		latestSitemapId, _ := strconv.Atoi(match[1])
		archivePath := fmt.Sprintf("%sarchive-%d.%s", w.PublicPath, latestSitemapId, w.PluginSitemap.Type)
		archiveFile := NewSitemapGenerator(w, archivePath, w.System.BaseUrl, true)
		var alternates []AlternateLink
		if multiLang {
			item, ok := data.(*model.Archive)
			if ok {
				for idx := range multiLangSites {
					var alternate AlternateLink
					if multiLangSiteType == config.MultiLangSiteTypeMulti {
						site := GetWebsite(multiLangSites[idx].Id)
						if site == nil {
							continue
						}
						if site.Id == w.Id {
							continue
						}
						alternate = AlternateLink{
							Href:     site.GetUrl("archive", item, 0),
							Hreflang: site.System.Language,
						}
					} else {
						// single
						alternate = AlternateLink{
							Href:     w.MultiLanguage.GetUrl(link, w.System.BaseUrl, &multiLangSites[idx]),
							Hreflang: multiLangSites[idx].Language,
						}
					}
					alternates = append(alternates, alternate)
				}
			}
		}
		if len(archiveFile.Urls) >= SitemapLimit {
			// 生成新文件
			latestSitemapId++
			//写入index
			indexFile.AddIndex(fmt.Sprintf("%s/archive-%d.%s", w.System.BaseUrl, latestSitemapId, w.PluginSitemap.Type))
			_ = indexFile.Save()
			archivePathNew := fmt.Sprintf("%sarchive-%d.%s", w.PublicPath, latestSitemapId+1, w.PluginSitemap.Type)
			archiveFile2 := NewSitemapGenerator(w, archivePathNew, w.System.BaseUrl, false)
			archiveFile2.AddLoc(link, lastmod, alternates)
			_ = archiveFile2.Save()
		} else {
			archiveFile.AddLoc(link, lastmod, alternates)
			_ = archiveFile.Save()
		}
		_ = w.UpdateSitemapTime()
	} else if itemType == "tag" {
		// 读取SitemapIndex，并找到最后一个
		indexFile := NewSitemapIndexGenerator(w, w.PluginSitemap.Type, fmt.Sprintf("%ssitemap.%s", w.PublicPath, w.PluginSitemap.Type), w.System.BaseUrl, true)
		var latestSitemap string
		for _, v := range indexFile.Sitemaps {
			if strings.Contains(v.Loc, "tag-") {
				latestSitemap = v.Loc
			}
		}
		re, _ := regexp.Compile(`tag-(\d+)`)
		match := re.FindStringSubmatch(latestSitemap)
		var latestSitemapId int
		if len(match) == 2 {
			latestSitemapId, _ = strconv.Atoi(match[1])
		}
		tagPath := fmt.Sprintf("%stag-%d.%s", w.PublicPath, latestSitemapId, w.PluginSitemap.Type)
		tagFile := NewSitemapGenerator(w, tagPath, w.System.BaseUrl, true)
		var alternates []AlternateLink
		if multiLang {
			item, ok := data.(*model.Tag)
			if ok {
				for idx := range multiLangSites {
					var alternate AlternateLink
					if multiLangSiteType == config.MultiLangSiteTypeMulti {
						site := GetWebsite(multiLangSites[idx].Id)
						if site == nil {
							continue
						}
						if site.Id == w.Id {
							continue
						}
						alternate = AlternateLink{
							Href:     site.GetUrl("tag", item, 0),
							Hreflang: site.System.Language,
						}
					} else {
						// single
						alternate = AlternateLink{
							Href:     w.MultiLanguage.GetUrl(link, w.System.BaseUrl, &multiLangSites[idx]),
							Hreflang: multiLangSites[idx].Language,
						}
					}
					alternates = append(alternates, alternate)
				}
			}
		}
		if len(tagFile.Urls) >= SitemapLimit {
			latestSitemapId++
			indexFile.AddIndex(fmt.Sprintf("%s/tag-%d.%s", w.System.BaseUrl, latestSitemapId, w.PluginSitemap.Type))
			tagPathNew := fmt.Sprintf("%stag-%d.%s", w.PublicPath, latestSitemapId, w.PluginSitemap.Type)
			tagFile2 := NewSitemapGenerator(w, tagPathNew, w.System.BaseUrl, false)
			tagFile2.AddLoc(link, lastmod, alternates)
			_ = tagFile2.Save()
		} else {
			tagFile.AddLoc(link, lastmod, alternates)
			_ = tagFile.Save()
		}

		_ = w.UpdateSitemapTime()
	}

	return nil
}

// AlternateLink 用于存储 hreflang 信息
type AlternateLink struct {
	Hreflang string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}

type SitemapUrl struct {
	Loc        string          `xml:"loc"`
	Lastmod    string          `xml:"lastmod,omitempty"`
	ChangeFreq string          `xml:"changefreq,omitempty"`
	Priority   string          `xml:"priority,omitempty"`
	Alternates []AlternateLink `xml:"xhtml:link,omitempty"`
}

type SitemapGenerator struct {
	XMLName  xml.Name     `xml:"urlset"`
	Xmlns    string       `xml:"xmlns,attr"`
	XmlnsX   string       `xml:"xmlns:xhtml,attr"`
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
		XmlnsX:   "http://www.w3.org/1999/xhtml",
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

func (g *SitemapGenerator) AddLoc(loc string, lastMod string, alternates []AlternateLink) {
	g.Urls = append(g.Urls, SitemapUrl{
		Loc:     loc,
		Lastmod: lastMod,
		//ChangeFreq: "daily",
		//Priority:   "0.8",
		Alternates: alternates,
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

func NewSitemapIndexGenerator(w *Website, sitemapType, filePath, baseUrl string, load bool) *SitemapIndexGenerator {
	generator := &SitemapIndexGenerator{
		w:        w,
		Type:     sitemapType,
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
