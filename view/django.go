package view

import (
	"bytes"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	stdPath "path"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"github.com/kataras/iris/v12/i18n"
	view2 "github.com/kataras/iris/v12/view"
	"golang.org/x/net/html"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"

	"github.com/fatih/structs"
	"github.com/flosch/pongo2/v6"
)

type (
	// Value type alias for pongo2.Value
	Value = pongo2.Value
	// Error type alias for pongo2.Error
	Error = pongo2.Error
	// FilterFunction type alias for pongo2.FilterFunction
	FilterFunction = pongo2.FilterFunction

	// Parser type alias for pongo2.Parser
	Parser = pongo2.Parser
	// Token type alias for pongo2.Token
	Token = pongo2.Token
	// INodeTag type alias for pongo2.InodeTag
	INodeTag = pongo2.INodeTag
	// TagParser the function signature of the tag's parser you will have
	// to implement in order to create a new tag.
	//
	// 'doc' is providing access to the whole document while 'arguments'
	// is providing access to the user's arguments to the tag:
	//
	//     {% your_tag_name some "arguments" 123 %}
	//
	// start_token will be the *Token with the tag's name in it (here: your_tag_name).
	//
	// Please see the Parser documentation on how to use the parser.
	// See `RegisterTag` for more information about writing a tag as well.
	TagParser = pongo2.TagParser
)

// AsValue converts any given value to a pongo2.Value
// Usually being used within own functions passed to a template
// through a Context or within filter functions.
//
// Example:
//
//	AsValue("my string")
//
// Shortcut for `pongo2.AsValue`.
var AsValue = pongo2.AsValue

// AsSafeValue works like AsValue, but does not apply the 'escape' filter.
// Shortcut for `pongo2.AsSafeValue`.
var AsSafeValue = pongo2.AsSafeValue

type tDjangoAssetLoader struct {
	rootDir string
	fs      fs.FS
}

// Abs calculates the path to a given template. Whenever a path must be resolved
// due to an import from another template, the base equals the parent template's path.
func (l *tDjangoAssetLoader) Abs(base, name string) string {
	if stdPath.IsAbs(name) {
		return name
	}

	return stdPath.Join(l.rootDir, name)
}

// Get returns an io.Reader where the template's content can be read from.
func (l *tDjangoAssetLoader) Get(path string) (io.Reader, error) {
	if stdPath.IsAbs(path) {
		path = path[1:]
	}

	res, err := asset(l.fs, path)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(res), nil
}

// DjangoEngine contains the django view engine structure.
type DjangoEngine struct {
	extension string
	reload    bool
	//
	rmu sync.RWMutex // locks for filters, globals and `ExecuteWiter` when `reload` is true.
	// filters for pongo2, map[name of the filter] the filter function . The filters are auto register
	filters map[string]FilterFunction
	// globals share context fields between templates.
	globals map[string]interface{}
	Set     map[uint]*pongo2.TemplateSet
	// multiple sites support
	templateCache map[uint]map[string]*pongo2.Template
}

var (
	_ view2.Engine       = (*DjangoEngine)(nil)
	_ view2.EngineFuncer = (*DjangoEngine)(nil)
)

// Django creates and returns a new django view engine.
// The given "extension" MUST begin with a dot.
//
// Usage:
// Django("./views", ".html") or
// Django(iris.Dir("./views"), ".html") or
// Django(embed.FS, ".html") or Django(AssetFile(), ".html") for embedded data.
func Django(extension string) *DjangoEngine {
	s := &DjangoEngine{
		extension:     extension,
		globals:       make(map[string]interface{}),
		filters:       make(map[string]FilterFunction),
		Set:           make(map[uint]*pongo2.TemplateSet),
		templateCache: make(map[uint]map[string]*pongo2.Template),
	}

	return s
}

// Name returns the django engine's name.
func (s *DjangoEngine) Name() string {
	return "Django"
}

// Ext returns the file extension which this view engine is responsible to render.
// If the filename extension on ExecuteWriter is empty then this is appended.
func (s *DjangoEngine) Ext() string {
	return s.extension
}

// Reload if set to true the templates are reloading on each render,
// use it when you're in development and you're boring of restarting
// the whole app when you edit a template file.
//
// Note that if `true` is passed then only one `View -> ExecuteWriter` will be render each time,
// no concurrent access across clients, use it only on development status.
// It's good to be used side by side with the https://github.com/kataras/rizla reloader for go source files.
func (s *DjangoEngine) Reload(developmentMode bool) *DjangoEngine {
	s.reload = developmentMode
	return s
}

// AddFunc adds the function to the template's Globals.
// It is legal to overwrite elements of the default actions:
// - url func(routeName string, args ...string) string
// - urlpath func(routeName string, args ...string) string
// - render func(fullPartialName string) (template.HTML, error).
func (s *DjangoEngine) AddFunc(funcName string, funcBody interface{}) {
	s.rmu.Lock()
	s.globals[funcName] = funcBody
	s.rmu.Unlock()
}

// AddFilter registers a new filter. If there's already a filter with the same
// name, RegisterFilter will panic. You usually want to call this
// function in the filter's init() function:
// http://golang.org/doc/effective_go.html#init
//
// Same as `RegisterFilter`.
func (s *DjangoEngine) AddFilter(filterName string, filterBody FilterFunction) *DjangoEngine {
	return s.registerFilter(filterName, filterBody)
}

// RegisterFilter registers a new filter. If there's already a filter with the same
// name, RegisterFilter will panic. You usually want to call this
// function in the filter's init() function:
// http://golang.org/doc/effective_go.html#init
//
// See http://www.florian-schlachter.de/post/pongo2/ for more about
// writing filters and tags.
func (s *DjangoEngine) RegisterFilter(filterName string, filterBody FilterFunction) *DjangoEngine {
	return s.registerFilter(filterName, filterBody)
}

func (s *DjangoEngine) registerFilter(filterName string, fn FilterFunction) *DjangoEngine {
	pongo2.RegisterFilter(filterName, fn)
	return s
}

// RegisterTag registers a new tag. You usually want to call this
// function in the tag's init() function:
// http://golang.org/doc/effective_go.html#init
//
// See http://www.florian-schlachter.de/post/pongo2/ for more about
// writing filters and tags.
func (s *DjangoEngine) RegisterTag(tagName string, fn TagParser) error {
	return pongo2.RegisterTag(tagName, fn)
}

// Load parses the templates to the engine.
// It is responsible to add the necessary global functions.
//
// Returns an error if something bad happens, user is responsible to catch it.
func (s *DjangoEngine) Load() error {
	_ = s.LoadStart(false)
	// 不返回错误，否则出错了程序无法启动
	return nil
}

func (s *DjangoEngine) LoadStart(throw bool) error {
	// multiple sites support
	websites := provider.GetWebsites()
	var err error
	for _, site := range websites {
		if !throw {
			s.Set[site.Id] = nil
			s.templateCache[site.Id] = nil
		}
		if !site.Initialed {
			continue
		}
		// 检查模板是否有多语言
		var mapLocales = map[string]struct{}{}
		sfs := getFS(site.GetTemplateDir())
		rootDirName := getRootDirName(sfs)
		var tplFiles = make(map[string]int64, 100)
		err = walk(sfs, "", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if throw {
					return err
				}
				return nil
			}

			if info == nil || info.IsDir() {
				return nil
			}
			// 判断是否有多语言
			if strings.HasPrefix(path, "locales") {
				pathSplit := strings.Split(path, "/")
				if len(pathSplit) > 2 {
					mapLocales[pathSplit[1]] = struct{}{}
				}
			}

			if s.extension != "" {
				if !strings.HasSuffix(path, s.extension) {
					return nil
				}
			}

			if site.RootPath == rootDirName {
				path = strings.TrimPrefix(path, rootDirName)
				path = strings.TrimPrefix(path, "/")
			}

			contents, err := asset(sfs, path)
			if err != nil {
				if throw {
					return err
				}
				return nil
			}
			tplFiles[path] = info.Size()
			err = s.ParseTemplate(site, path, contents)
			if err != nil && throw {
				return err
			}
			return nil
		})
		site.SetTemplates(tplFiles)
		if len(mapLocales) > 0 {
			var locales = make([]string, 0, len(mapLocales))
			for k := range mapLocales {
				locales = append(locales, k)
			}
			tplI18n := i18n.New()
			err = tplI18n.LoadFS(sfs, "./locales/*/*.yml", locales...)
			if err == nil {
				site.TplI18n = tplI18n
			}
		}
	}

	return err
}

// ParseTemplate adds a custom template from text.
// This parser does not support funcs per template. Use the `AddFunc` instead.
func (s *DjangoEngine) ParseTemplate(site *provider.Website, name string, contents []byte) error {
	s.rmu.Lock()
	defer s.rmu.Unlock()

	s.initSet(site)

	name = strings.TrimPrefix(name, "/")
	tmpl, err := s.Set[site.Id].FromBytes(contents)
	if s.templateCache[site.Id] == nil {
		s.templateCache[site.Id] = make(map[string]*pongo2.Template)
	}
	if err == nil {
		s.templateCache[site.Id][name] = tmpl
	} else {
		s.templateCache[site.Id][name], _ = s.Set[site.Id].FromBytes([]byte(err.Error() + "<br/> on file " + name))
	}

	return nil
}

func (s *DjangoEngine) initSet(site *provider.Website) { // protected by the caller.
	if s.Set[site.Id] == nil {
		s.Set[site.Id] = pongo2.NewSet("", &tDjangoAssetLoader{fs: getFS(site.GetTemplateDir()), rootDir: "./"})
		s.Set[site.Id].Globals = getPongoContext(s.globals)
	}
}

// getPongoContext returns the pongo2.Context from map[string]interface{} or from pongo2.Context, used internaly
func getPongoContext(templateData interface{}) pongo2.Context {
	if templateData == nil {
		return nil
	}

	switch data := templateData.(type) {
	case pongo2.Context:
		return data
	case context.Map:
		return pongo2.Context(data)
	default:
		// if struct, convert it to map[string]interface{}
		if structs.IsStruct(data) {
			return pongo2.Context(structs.Map(data))
		}

		panic("django: template data: should be a map or struct")
	}
}

func (s *DjangoEngine) fromCache(siteId uint, relativeName string) *pongo2.Template {
	if s.reload {
		s.rmu.RLock()
		defer s.rmu.RUnlock()
	}

	if tmpl, ok := s.templateCache[siteId][relativeName]; ok {
		return tmpl
	}
	return nil
}

// ExecuteWriter executes a templates and write its results to the w writer
// layout here is useless.
func (s *DjangoEngine) ExecuteWriter(w io.Writer, filename string, _ string, bindingData interface{}) error {
	// reparse the templates if reload is enabled.
	if s.reload {
		if err := s.LoadStart(true); err != nil {
			return err
		}
	}
	ctx := w.(iris.Context)
	// 检查是否已经超时
	if err := ctx.Request().Context().Err(); err != nil {
		return err
	}
	currentSite := provider.CurrentSite(ctx)
	if tmpl := s.fromCache(currentSite.Id, filename); tmpl != nil {
		// 在执行模板渲染前再次检查超时状态
		if err := ctx.Request().Context().Err(); err != nil {
			return err
		}
		data, err := tmpl.ExecuteBytes(getPongoContext(bindingData))
		if err != nil {
			return err
		}
		// 再次检查是否超时
		if err := ctx.Request().Context().Err(); err != nil {
			return err
		}
		// 如果启用了防采集
		if currentSite.PluginInterference.Open {
			if currentSite.PluginInterference.DisableSelection ||
				currentSite.PluginInterference.DisableCopy ||
				currentSite.PluginInterference.DisableRightClick {
				addonText := "<script type=\"text/javascript\">\nwindow.onload = function() {\n"
				if currentSite.PluginInterference.DisableSelection {
					addonText += "document.onselectstart = function (e) {e.preventDefault();};\n"
				}
				if currentSite.PluginInterference.DisableCopy {
					addonText += "document.oncopy = function(e) {e.preventDefault();}\n"
				}
				if currentSite.PluginInterference.DisableRightClick {
					addonText += "document.oncontextmenu = function(e){e.preventDefault();}\n"
				}
				addonText += "}</script>"
				if index := bytes.LastIndex(data, []byte("</body>")); index != -1 {
					index = index + 7
					tmpData := make([]byte, len(data)+len(addonText))
					copy(tmpData, data[:index])
					copy(tmpData[index:], addonText)
					copy(tmpData[index+len(addonText):], data[index:])
					data = tmpData
				} else {
					data = append(data, addonText...)
				}
			}
			// 基于每个页面独立不变，则这里需要根据页面URL确定唯一值
			if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok && len(webInfo.CanonicalUrl) > 0 {
				if currentSite.PluginInterference.Mode == config.InterferenceModeClass {
					// 添加随机class
					re, _ := regexp.Compile(`(?i)<(a|article|div|h1|h2|h3|h4|h5|h6|img|p|span|table|section)[\S\s]*?>`)
					classRe, _ := regexp.Compile(`(?i)class="(.*?)"`)
					index := 0
					data = re.ReplaceAllFunc(data, func(b []byte) []byte {
						index++
						randClass := crc32.ChecksumIEEE([]byte(webInfo.CanonicalUrl + strconv.Itoa(index)))
						match := classRe.FindSubmatch(b)
						if len(match) == 2 {
							tmpB := make([]byte, len(match[0]))
							copy(tmpB, match[0])
							tmpB = append(tmpB[:len(tmpB)-1], " "+library.DecimalToLetter(int64(randClass))+"\""...)
							b = bytes.Replace(b, match[0], tmpB, 1)
						} else {
							b = bytes.Replace(b, []byte{'>'}, []byte(" class=\""+library.DecimalToLetter(int64(randClass))+"\">"), 1)
						}
						return b
					})
				} else if currentSite.PluginInterference.Mode == config.InterferenceModeText {
					// 生成10个隐藏的class
					addonStyle := "<style type=\"text/css\">\n"
					hiddenStyles := []string{
						"{display: inline-block;width: .1px;height: .1px;overflow: hidden;visibility: hidden;}\n",
						"{display: inline-block;font-size: 0!important;width: 1em;height: 1em;visibility: hidden;line-height: 0;}\n",
						"{display: none!important;}\n",
					}
					for i := 0; i < 5; i++ {
						tmpClass := library.DecimalToLetter(int64(crc32.ChecksumIEEE([]byte(webInfo.CanonicalUrl + strconv.Itoa(i)))))
						addonStyle += "    ." + tmpClass + hiddenStyles[i%len(hiddenStyles)]
					}
					addonStyle += "</style>\n"
					if index := bytes.Index(data, []byte("</head>")); index != -1 {
						tmpData := make([]byte, len(data)+len(addonStyle))
						copy(tmpData, data[:index])
						copy(tmpData[index:], addonStyle)
						copy(tmpData[index+len(addonStyle):], data[index:])
						data = tmpData
					} else {
						data = append(data, addonStyle...)
					}
				}
			}
		}
		// 对data进行敏感词替换
		data = currentSite.ReplaceSensitiveWords(data)
		pjax := ctx.GetHeader("X-Pjax")
		var pjaxContainer string
		if pjax == "true" {
			pjaxContainer = ctx.GetHeader("X-Pjax-Container")
			if pjaxContainer == "" {
				pjaxContainer = ctx.URLParam("_pjax")
			}
			if pjaxContainer == "" {
				pjaxContainer = "pjax-container"
			} else {
				pjaxContainer = strings.TrimLeft(pjaxContainer, "#")
			}
			doc, err := html.Parse(bytes.NewBuffer(data))
			if err == nil {
				// 查找 #pjax-container 节点
				node := findNodeByID(doc, pjaxContainer)
				if node != nil {
					data = getInnerHTML(node)
				}
			}
		}
		// 对于模板是pc+mobile的域名，需要做替换
		if len(currentSite.System.MobileUrl) > 0 {
			mobileTemplate := ctx.Values().GetBoolDefault("mobileTemplate", false)
			if mobileTemplate {
				data = bytes.ReplaceAll(data, []byte(currentSite.System.BaseUrl), []byte(currentSite.System.MobileUrl))
			}
		}
		// 添加json-ld
		if currentSite.PluginJsonLd.Open {
			jsonLd := currentSite.GetJsonLd(ctx)
			if len(jsonLd) > 0 {
				jsonLdBuf := []byte("\n<script type=\"application/ld+json\">\n" + jsonLd + "\n</script>\n")
				if index := bytes.LastIndex(data, []byte("</body>")); index != -1 {
					index = index + 7
					tmpData := make([]byte, len(data)+len(jsonLdBuf))
					copy(tmpData, data[:index])
					copy(tmpData[index:], jsonLdBuf)
					copy(tmpData[index+len(jsonLdBuf):], data[index:])
					data = tmpData
				} else {
					data = append(data, jsonLdBuf...)
				}
			}
		}

		buf := bytes.NewBuffer(data)
		_, err = buf.WriteTo(w)
		return err
	}

	return view2.ErrNotExist{Name: filename, IsLayout: false, Data: bindingData}
}
func findNodeByID(n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "id" && attr.Val == id {
				return n
			}
		}
	}

	// 递归查找子节点
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if result := findNodeByID(child, id); result != nil {
			return result
		}
	}

	return nil
}

func getInnerHTML(n *html.Node) []byte {
	var buf bytes.Buffer
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		html.Render(&buf, child)
	}
	return buf.Bytes()
}
