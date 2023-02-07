package library

import (
	"encoding/hex"
	"github.com/disintegration/imaging"
	"image"
	"image/color"
	"image/draw"
	"strings"
)

func ThumbnailCrop(minWidth, minHeight int, img image.Image, thumbCrop int) image.Image {
	if minWidth == 0 {
		//默认值
		minWidth = 250
	}
	if minHeight == 0 {
		//默认值
		minHeight = 250
	}

	var thumbImg image.Image
	if thumbCrop == 0 {
		//等比缩放
		thumbImg = imaging.Fit(img, minWidth, minHeight, imaging.Lanczos)
	} else if thumbCrop == 1 {
		//补白
		thumbImg = imaging.Fit(img, minWidth, minHeight, imaging.Lanczos)
		thumbImg = ResizeFill(thumbImg, minWidth, minHeight)
	} else {
		//裁剪
		thumbImg = imaging.Thumbnail(img, minWidth, minHeight, imaging.Lanczos)
	}

	return thumbImg
}

func Resize(img image.Image, dstWidth, dstHeight int) image.Image {
	return imaging.Resize(img, dstWidth, dstHeight, imaging.Lanczos)
}

func ResizeFill(img image.Image, width, height int) image.Image {
	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(rgba, rgba.Bounds(), image.White, image.Point{}, draw.Src)
	return imaging.PasteCenter(rgba, img)
}

func HEXToRGB(h string) color.Color {
	h = strings.Trim(h, "#")
	if h == "" {
		return color.RGBA{}
	}
	if len(h) == 3 {
		h = string(h[0] + h[0] + h[1] + h[1] + h[2] + h[2])
	}
	bs, err := hex.DecodeString(h)
	if err != nil {
		return color.RGBA{}
	}
	if len(bs) != 3 {
		return color.RGBA{}
	}
	return color.RGBA{R: bs[0], G: bs[1], B: bs[2], A: 255}
}
