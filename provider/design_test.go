package provider

import (
	"log"
	"testing"
)

func TestGetDesignList(t *testing.T) {
	designList := GetDesignList()

	log.Printf("%#v", designList)
}
