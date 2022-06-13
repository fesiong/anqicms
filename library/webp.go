package library

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/chai2010/webp"
	"golang.org/x/image/bmp"
	"image"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
)

const webpMax = 16383

func ConvertImage(raw, optimized string) error {
	//we need to create dir first
	err := os.MkdirAll(path.Dir(optimized), 0755)
	if err != nil {
		return err
	}

	err = webpEncoder(raw, optimized, 80)

	return err
}

func readRawImage(imgPath string, maxPixel int) (img image.Image, err error) {
	data, err := ioutil.ReadFile(imgPath)
	if err != nil {
		return
	}

	imgExtension := strings.ToLower(path.Ext(imgPath))
	if strings.Contains(imgExtension, "jpeg") || strings.Contains(imgExtension, "jpg") {
		img, err = jpeg.Decode(bytes.NewReader(data))
	} else if strings.Contains(imgExtension, "png") {
		img, err = png.Decode(bytes.NewReader(data))
	} else if strings.Contains(imgExtension, "bmp") {
		img, err = bmp.Decode(bytes.NewReader(data))
	}
	if err != nil || img == nil {
		errInfo := fmt.Sprintf("image file %s is corrupted: %v", imgPath, err)
		return nil, errors.New(errInfo)
	}

	x, y := img.Bounds().Max.X, img.Bounds().Max.Y
	if x > maxPixel || y > maxPixel {
		errInfo := fmt.Sprintf("WebP: %s(%dx%d) is too large", imgPath, x, y)
		return nil, errors.New(errInfo)
	}

	return img, nil
}

func webpEncoder(p1, p2 string, quality float32) error {
	// if convert fails, return error; success nil
	var buf bytes.Buffer
	var img image.Image
	// The maximum pixel dimensions of a WebP image is 16383 x 16383.
	img, err := readRawImage(p1, webpMax)
	if err != nil {
		return err
	}

	err = webp.Encode(&buf, img, &webp.Options{Lossless: false, Quality: quality})
	if err != nil {
		log.Printf("Can't encode source image: %v to WebP", err)
		return err
	}

	if err = ioutil.WriteFile(p2, buf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}
