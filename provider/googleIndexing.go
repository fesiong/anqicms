package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2/google"
)

func (w *Website) GetGoogleIndexingAccess() (*http.Client, error) {
	content := w.PluginPush.GoogleJson
	if len(content) == 0 {
		return nil, errors.New(w.Tr("AccountError"))
	}

	ctx := context.Background()
	conf, err := google.JWTConfigFromJSON([]byte(content), "https://www.googleapis.com/auth/indexing")
	if err != nil {
		return nil, err
	}
	return conf.Client(ctx), nil
}

func (w *Website) PushGoogleIndexing(client *http.Client, domain string) (int, error) {
	body := map[string]string{
		"url":  domain,
		"type": "URL_UPDATED",
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", "https://indexing.googleapis.com/v3/urlNotifications:publish", bytes.NewReader(bodyBytes))
	if err != nil {
		return -1, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	w.logPushResult("google", fmt.Sprintf("%s, %d: %s", domain, resp.StatusCode, string(respBody)))

	return resp.StatusCode, nil
}

func (w *Website) PushGoogle(list []string) error {
	client, err := w.GetGoogleIndexingAccess()
	if err != nil {
		return err
	}
	for _, domain := range list {
		_, err := w.PushGoogleIndexing(client, domain)
		if err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}
