package provider

import (
	"log"
	"testing"
)

func TestWordSplit(t *testing.T) {
	s := "Golang 在线教程"

	result := WordSplit(s, false)

	log.Printf("%#v", result)
}
