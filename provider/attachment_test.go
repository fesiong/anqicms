package provider

import (
	"fmt"
	"image"
	"log"
	"net/http"
	"os"
	"testing"
)

func (w *Website) TestDownloadRemoteImage(t *testing.T) {
	link := "https://mmbiz.qpic.cn/mmbiz_jpg/YNoY3yGicTIRicbeSpTCnzxK1icJ0vBLlnMwibl9icyZcNnL4ml0ic3YI1Yp3RyeK8FicBu9OFVvmibRuK89ky5u2faCnw/640?wx_fmt=jpeg"
	alt := ""

	result, err := w.DownloadRemoteImage(link, alt)
	if err != nil {
		t.Fatal(err)
	}

	log.Printf("%#v", result)
}

func TestEncodeImage(t *testing.T) {
	imgUrl := "https://mmbiz.qpic.cn/mmbiz_jpg/YNoY3yGicTIRicbeSpTCnzxK1icJ0vBLlnMwibl9icyZcNnL4ml0ic3YI1Yp3RyeK8FicBu9OFVvmibRuK89ky5u2faCnw/640?wx_fmt=jpeg"
	res, err := http.Get(imgUrl)
	if err != nil {
		fmt.Println("A error occurred!")
		return
	}
	defer res.Body.Close()

	imageData, _, err := image.Decode(res.Body)
	if err != nil {
		fmt.Println("err decode", err)
		return
	}
	if data, imgType, err := encodeImage(imageData, "png", 90); err != nil {
		fmt.Println("err", err)
	} else {
		log.Println(imgType)
		os.WriteFile("1.png", data, os.ModePerm)
	}
}
