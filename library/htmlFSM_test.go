package library

import (
	"fmt"
	"testing"
	"time"
)

func TestHtmlFSM(t *testing.T) {
	html := `<div class="container">
        <!-- 示例内容与提问中的HTML结构一致 -->
        <div class="languages">
            <a href="http://cn.anqi.com/ji-chuang/42.html" class="active" data-pjax="false">
				<div class="language-item">
					<img class="language-icon" src="http://cn.anqi.com/uploads/202309/14/a9bb82ab686ed21e.webp"/>
					<span>简体中文</span>
				</div>
			</a>
			
			<a href="http://en.anqi.com/ji-chuang/42.html" class="" data-pjax="false">
				<div class="language-item">
					<span class="language-icon">🇺🇸</span>
					
					<span>English</span>
				</div>
			</a>
        </div>
    </div>`

	expect := `<div class="languages">
            <a href="http://cn.anqi.com/ji-chuang/42.html" class="active" data-pjax="false">
				<div class="language-item">
					<img class="language-icon" src="http://cn.anqi.com/uploads/202309/14/a9bb82ab686ed21e.webp"/>
					<span>简体中文</span>
				</div>
			</a>
			
			<a href="http://en.anqi.com/ji-chuang/42.html" class="" data-pjax="false">
				<div class="language-item">
					<span class="language-icon">🇺🇸</span>
					
					<span>English</span>
				</div>
			</a>
        </div>`

	st2 := time.Now().UnixMicro()
	locator := NewDivLocator("div", "languages")
	result2 := locator.FindDiv(html)
	ed2 := time.Now().UnixMicro()
	fmt.Printf("耗时：%d 微秒\n", ed2-st2)
	fmt.Println("提取结果：")
	fmt.Println(result2)
	if result2 != expect {
		t.Errorf("提取结果不匹配，期望：%s，实际：%s", expect, result2)
	}
}
