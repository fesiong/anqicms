package library

import (
	"github.com/disintegration/imaging"
	"image"
	"image/draw"
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
	} else if thumbCrop == 2 {
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