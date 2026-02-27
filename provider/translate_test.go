package provider

import "testing"

func TestNewBaiduTranslate(t *testing.T) {
	tr := NewBaiduTranslate("xxx", "xxxx")

	content, err := tr.Translate("安企CMS自推出以来，已经逐步扩展到多站点功能，用户群体也在不断扩大。在这个过程中，用户们对于多语言功能的呼声越来越高。然而，当前版本的安企CMS虽然支持每个站点设置不同语言，但由于无法实现内容之间的语言切换，这种多语言功能显得不够完善。尤其是在国际化环境中，内容的多语言切换和自动翻译的需求变得尤为迫切。因此，作为开发者，我们决定改进多语言功能，以满足用户需求，并让安企CMS更好地服务于全球市场。", "auto", "en")

	if err != nil {
		t.Fatal(err)
	}

	t.Log(content)
}

func TestNewYoudaoTranslate(t *testing.T) {
	tr := NewYoudaoTranslate("xxx", "xxxx")

	content, err := tr.Translate("安企CMS自推出以来，已经逐步扩展到多站点功能，用户群体也在不断扩大。在这个过程中，用户们对于多语言功能的呼声越来越高。然而，当前版本的安企CMS虽然支持每个站点设置不同语言，但由于无法实现内容之间的语言切换，这种多语言功能显得不够完善。尤其是在国际化环境中，内容的多语言切换和自动翻译的需求变得尤为迫切。因此，作为开发者，我们决定改进多语言功能，以满足用户需求，并让安企CMS更好地服务于全球市场。", "auto", "en")

	if err != nil {
		t.Fatal(err)
	}

	t.Log(content)
}
