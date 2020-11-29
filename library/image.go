package library

import (
	"fmt"
	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
	"image"
)

func ThumbnailCrop(minWidth, minHeight uint, img image.Image) image.Image {
	origBounds := img.Bounds()
	origWidth := uint(origBounds.Dx())
	origHeight := uint(origBounds.Dy())
	newWidth, newHeight := origWidth, origHeight

	// Return original image if it have same or smaller size as constraints
	if minWidth >= origWidth && minHeight >= origHeight {
		return img
	}

	if minWidth > origWidth {
		minWidth = origWidth
	}

	if minHeight > origHeight {
		minHeight = origHeight
	}

	// Preserve aspect ratio
	if origWidth > minWidth {
		newHeight = uint(origHeight * minWidth / origWidth)
		if newHeight < 1 {
			newHeight = 1
		}
		//newWidth = minWidth
	}

	if newHeight < minHeight {
		newWidth = uint(newWidth * minHeight / newHeight)
		if newWidth < 1 {
			newWidth = 1
		}
		//newHeight = minHeight
	}

	if origWidth > origHeight {
		newWidth = minWidth
		newHeight = 0
	}else {
		newWidth = 0
		newHeight = minHeight
	}

	thumbImg := resize.Resize(newWidth, newHeight, img, resize.Lanczos3)
	//return CropImg(thumbImg, int(minWidth), int(minHeight))
	return thumbImg
}

func Resize(width, height uint, img image.Image, interp resize.InterpolationFunction) image.Image {
	return resize.Resize(width, height, img, interp)
}

func Thumbnail(width, height uint, img image.Image, interp resize.InterpolationFunction) image.Image {
	return resize.Thumbnail(width, height, img, interp)
}

func CropImg(srcImg image.Image, dstWidth, dstHeight int) image.Image {
	//origBounds := srcImg.Bounds()
	//origWidth := origBounds.Dx()
	//origHeight := origBounds.Dy()

	dstImg, err := cutter.Crop(srcImg, cutter.Config{
		Height: dstHeight,      // height in pixel or Y ratio(see Ratio Option below)
		Width:  dstWidth,       // width in pixel or X ratio
		Mode:   cutter.Centered, // Accepted Mode: TopLeft, Centered
		//Anchor: image.Point{
		//	origWidth / 12,
		//	origHeight / 8}, // Position of the top left point
		Options: 0, // Accepted Option: Ratio
	})
	fmt.Println()
	if err != nil {
		fmt.Println("[GIN] Cannot crop image:" + err.Error())
		return srcImg
	}
	return dstImg
}