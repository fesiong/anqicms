package provider

import (
	"kandaoni.com/anqicms/request"
	"log"
	"testing"
)

func TestCreateTransferTask(t *testing.T) {
	website := request.TransferWebsite{
		Name:     "ncwordpress",
		BaseUrl:  "https://www.nokiipx.com/",
		Token:    "anqicms",
		Provider: "wordpress",
	}
	task, err := CreateTransferTask(&website)
	if err != nil {
		t.Fatal(err)
	}

	task.TransferWebData()
	log.Println(task.Current, task.ErrorMsg)
}
