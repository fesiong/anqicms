package provider

import (
	"log"
	"testing"
)

func TestNewDeepl(t *testing.T) {
	text := "欢迎使用AnQiCMS"
	key := "xxxxxx:fx"
	client := NewDeepl(key)
	glossariesId, err := client.GetGlossaries()
	if err != nil {
		t.Fatal(err)
	}
	log.Println("glossariesId", glossariesId)
	translation, _, err := client.Translate(
		text,
		"zh",
		"en",
		glossariesId,
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%#v", translation)
}
