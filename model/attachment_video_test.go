package model

import (
	"encoding/json"
	"testing"
)

func TestAttachmentExposesVideoTaskAndPlayURL(t *testing.T) {
	attachment := Attachment{
		Id:        7,
		IsImage:   2,
		PlayURL:   "https://cdn.example/master.m3u8",
		VideoTask: &VideoTask{Id: 9, Status: VideoTaskStatusSuccess},
	}
	encoded, err := json.Marshal(attachment)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["play_url"] != attachment.PlayURL {
		t.Fatalf("play_url = %#v", payload["play_url"])
	}
	task, ok := payload["video_task"].(map[string]any)
	if !ok || task["status"] != float64(VideoTaskStatusSuccess) {
		t.Fatalf("video_task = %#v", payload["video_task"])
	}
}
