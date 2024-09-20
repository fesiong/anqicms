package provider

import (
	"log"
	"testing"
)

func (w *Website) TestGetDesignList(t *testing.T) {
	designList := w.GetDesignList()

	log.Printf("%#v", designList)
}
