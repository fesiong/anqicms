package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
)

func (w *Website) DeleteCacheIndex() {
	w.RemoveHtmlCache("/")
}

func init() {
	// check what if this server can visit google
	go func() {
		resp, err := library.GetURLData("https://www.google.com", "", 5)
		if err != nil {
			config.GoogleValid = false
		} else {
			config.GoogleValid = true
			log.Println("google-status", resp.StatusCode)
		}
	}()
}
