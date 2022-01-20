package provider

import (
	"context"
	"fmt"
	"github.com/chromedp/chromedp"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var articleLang = [2]string{"zh", "en"}

type translateSources struct {
	sync.Mutex
	List []func(content string, to string) string
	index int
}

func (t *translateSources) getSource() func(content string, to string) string {
	t.Lock()
	defer t.Unlock()
	t.index = (t.index + 1) % len(t.List)
	return t.List[t.index]
}

var TranslateSources *translateSources

func init() {
	TranslateSources = &translateSources{
		List: []func(content string, to string) string{
			TranslateFromSogou,
			TranslateFromQQ,
		},
		index: 0,
	}
}

// TranslateFromQQ 单次限制2000字
func TranslateFromQQ(content string, to string) string {
	log.Println("from QQ")
	// https://fanyi.qq.com/

	var contents []string
	tmpContents := strings.Split(content, "\n")
	curCount := 0
	curIndex := 0
	for i := 0; i < len(tmpContents); i++ {
		curCount += utf8.RuneCountInString(tmpContents[i])
		if curCount > 2000 {
			contents = append(contents, strings.Join(tmpContents[curIndex:i], "\n"))
			curIndex = i
			curCount = utf8.RuneCountInString(tmpContents[i])
		} else if i == len(tmpContents)-1 {
			//最后一个
			contents = append(contents, strings.Join(tmpContents[curIndex:], "\n"))
		}
	}

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), GetAllocOpts()...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	timeCtx, cancel := context.WithTimeout(ctx, time.Duration(len(contents))*5*time.Second)
	defer cancel()

	for i := range contents {
		var res string
		err := chromedp.Run(timeCtx,
			chromedp.Navigate("https://fanyi.qq.com/"),
			chromedp.SetValue(`//textarea[@class="textinput"]`, contents[i]),
			chromedp.WaitVisible(`//span[@class="text-dst"]`),
			chromedp.Text(`//div[@class="textpanel-target-textblock"]`, &res, chromedp.NodeVisible),
		)
		if err != nil {
			log.Println("qq翻译失败：", err.Error())
			return ""
		}
		time.Sleep(time.Duration(2 + rand.Intn(5)) * time.Second)

		contents[i] = TrimContents(res)
	}

	return strings.Join(contents, "\n")
}

// TranslateFromSogou 单次限制5000字
func TranslateFromSogou(content string, to string) string {
	log.Println("from sogou")
	// https://fanyi.sogou.com/text?fr=common_index_nav_pc&ie=utf8&keyword=&p=40051205
	from := "zh-CHS"
	if to == "zh" {
		from = "en"
		to = "zh-CHS"
	}

	var contents []string
	tmpContents := strings.Split(content, "\n")
	curCount := 0
	curIndex := 0
	for i := 0; i < len(tmpContents); i++ {
		curCount += utf8.RuneCountInString(tmpContents[i])
		if curCount > 5000 {
			contents = append(contents, strings.Join(tmpContents[curIndex:i], "\n"))
			curIndex = i
			curCount = utf8.RuneCountInString(tmpContents[i])
		} else if i == len(tmpContents)-1 {
			//最后一个
			contents = append(contents, strings.Join(tmpContents[curIndex:], "\n"))
		}
	}

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), GetAllocOpts()...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	timeCtx, cancel := context.WithTimeout(ctx, time.Duration(len(contents))*5*time.Second)
	defer cancel()

	for i := range contents {
		var res string
		err := chromedp.Run(timeCtx,
			chromedp.Navigate(fmt.Sprintf("https://fanyi.sogou.com/text?keyword=&transfrom=%s&transto=%s", from, to)),
			chromedp.SetValue(`#trans-input`, contents[i]),
			chromedp.WaitVisible(`#trans-result`),
			chromedp.Text(`#trans-result`, &res, chromedp.NodeVisible),
		)
		if err != nil {
			log.Println("搜狗翻译失败：", err.Error())
			return ""
		}
		time.Sleep(time.Duration(2 + rand.Intn(5)) * time.Second)

		contents[i] = TrimContents(res)
	}

	return strings.Join(contents, "\n")
}

// ChromeCtx 使用一个实例
var allocOpts []chromedp.ExecAllocatorOption
func GetAllocOpts() []chromedp.ExecAllocatorOption {
	if allocOpts == nil {
		allocOpts = chromedp.DefaultExecAllocatorOptions[:]
		allocOpts = append(allocOpts,
			//chromedp.Flag("headless", false),
			chromedp.Flag("blink-settings", "imagesEnabled=false"),
			chromedp.UserAgent(`Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.55 Safari/537.36`),
			chromedp.Flag("accept-language", `zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7,zh-TW;q=0.6`),
		)
	}

	return allocOpts
}
