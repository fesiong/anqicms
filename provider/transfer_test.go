package provider

import (
	"kandaoni.com/anqicms/request"
	"log"
	"testing"
)

func (w *Website) TestCreateTransferTask(t *testing.T) {
	website := request.TransferWebsite{
		Name:     "ncwordpress",
		BaseUrl:  "https://www.nokiipx.com/",
		Token:    "anqicms",
		Provider: "wordpress",
	}
	task, err := w.CreateTransferTask(&website)
	if err != nil {
		t.Fatal(err)
	}

	task.TransferWebData(&request.TransferTypes{})
	log.Println(task.Current, task.ErrorMsg)
}

func (w *Website) TestParseContent(t *testing.T) {
	conten := `<code>func(){ echo 'aaa';}</code><!-- wp:image --><figure class="wp-block-image"><img src="http://www.ytyxqj.com/Upload/5cb420751572e.jpg" alt=""/></figure><!-- /wp:image -->`

	result := ParseContent(conten)

	log.Println(result)
}
