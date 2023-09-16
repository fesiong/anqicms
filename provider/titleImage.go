package provider

import (
	"encoding/base64"
	"fmt"
	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"math"
	"math/rand"
	"mime/multipart"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/flopp/go-findfont"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

type TitleImage struct {
	PublicPath string
	config     *config.PluginTitleImageConfig
	useWebp    int
	title      string
	sum        string
	font       *truetype.Font
	fontSize   int
	img        image.Image
}

func (w *Website) NewTitleImage(title string) *TitleImage {
	rand.Seed(time.Now().UnixNano())
	f := loadLocalFont(w.PublicPath + w.PluginTitleImage.FontPath)
	fontSize := w.PluginTitleImage.FontSize
	if fontSize < 16 {
		fontSize = 32
	}
	t := TitleImage{
		PublicPath: w.PublicPath,
		config:     &w.PluginTitleImage,
		useWebp:    w.Content.UseWebp,
		title:      title,
		sum:        sumTitle(title),
		font:       f,
		fontSize:   fontSize,
	}

	t.makeBackground()

	t.drawTitle()

	return &t
}

func sumTitle(title string) string {
	str := library.Md5(title)
	var newStr = make([]byte, 0, 32)
	for i := range str {
		if str[i] > 57 {
			newStr = append(newStr, str[i]-49)
		} else {
			newStr = append(newStr, str[i])
		}
	}
	return string(newStr)
}

func (t *TitleImage) makeBackground() {
	if len(t.config.BgImage) > 0 {
		file, err := os.Open(t.PublicPath + t.config.BgImage)
		defer file.Close()
		if err == nil {
			img, _, err := image.Decode(file)
			if err == nil {
				t.img = imaging.Resize(img, t.config.Width, t.config.Height, imaging.Lanczos)
				return
			} else {
				file.Seek(0, 0)
				if strings.HasSuffix(t.config.BgImage, "webp") {
					img, err = webp.Decode(file)
					if err == nil {
						t.img = imaging.Resize(img, t.config.Width, t.config.Height, imaging.Lanczos)
						return
					}
				}
			}
		}
	}
	// auto generate
	tmpH := 6
	tmpW := int(float64(t.config.Width) / float64(t.config.Height) * float64(tmpH))
	bgColor := t.RandDeepColor(0)
	m := image.NewRGBA(image.Rect(0, 0, tmpW, tmpH))

	draw.Draw(m, m.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)
	if t.config.Noise {
		n := 0
		for i := 0; i < tmpH; i++ {
			if t.sum[i]%2 != 0 {
				continue
			}
			for j := 0; j < tmpW; j++ {
				n = (n + 1) % 32
				if t.sum[n]%3 == 0 {
					m.Set(j, i, t.RandDeepColor(int(t.sum[n])%22))
				}
			}
		}
	}
	t.img = imaging.Resize(m, t.config.Width, t.config.Height, imaging.Gaussian)
}

func (t *TitleImage) Save(w *Website) (string, error) {
	imgType := "png"
	if t.useWebp == 1 {
		imgType = "webp"
	}
	buf, _ := encodeImage(t.img, imgType, 100)

	fileHeader := &multipart.FileHeader{
		Filename: library.Md5(t.title) + "." + imgType,
		Header:   nil,
		Size:     int64(len(buf)),
	}

	tmpfile, _ := os.CreateTemp("", fileHeader.Filename)
	defer os.Remove(tmpfile.Name()) // clean up
	tmpfile.Write(buf)

	attachment, err := w.AttachmentUpload(tmpfile, fileHeader, 0, 0)
	if err != nil {
		return "", err
	}

	return attachment.FileLocation, nil
}

func (t *TitleImage) EncodeB64string() string {
	buf, _ := encodeImage(t.img, "png", 85)

	return fmt.Sprintf("data:%s;base64,%s", "image/png", base64.StdEncoding.EncodeToString(buf))
}

func (t *TitleImage) drawTitle() {
	if len(t.title) == 0 {
		return
	}
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetClip(t.img.Bounds())
	m := image.NewNRGBA(t.img.Bounds())
	draw.Draw(m, t.img.Bounds(), t.img, image.Point{}, draw.Src)
	c.SetDst(m)
	c.SetHinting(font.HintingFull)
	c.SetFont(t.font)
	// 文字最小，最大
	minSize := t.fontSize
	maxSize := 100
	gap := 100

	realSize := (t.config.Width - gap) / utf8.RuneCountInString(t.title)
	if realSize < minSize {
		realSize = minSize
	} else if realSize > maxSize {
		realSize = maxSize
	}
	c.SetFontSize(float64(realSize))
	c.SetSrc(image.NewUniform(library.HEXToRGB(t.config.FontColor)))

	textWidth := t.getLettersLen([]rune(t.title), realSize)
	lineLen := int(math.Ceil(float64(textWidth) / float64(t.config.Width-gap)))
	// 行高 size * 1.6
	runeText := []rune(t.title)
	start := 0
	startY := t.config.Height/2 - (int(float64(realSize)/1.5) + int(float64((lineLen-1)*realSize)*1.6)/2) + realSize
	for i := 0; i < lineLen; i++ {
		tmpWidth := int(0)
		tmpText := string(runeText[start:])
		for j := start; j < len(runeText); j++ {
			tmpWidth += t.getLettersLen([]rune{runeText[j]}, realSize)
			if tmpWidth > (t.config.Width - gap) {
				// 退回一个
				tmpText = string(runeText[start:j])
				start = j
				break
			}
		}
		if len(tmpText) > 0 {
			tmpWidth = t.getLettersLen([]rune(tmpText), realSize)

			startX := (t.config.Width - int(tmpWidth)) / 2
			if i > 0 {
				startY += int(float64(realSize) * 1.6)
			}
			pt := freetype.Pt(startX, startY)
			_, _ = c.DrawString(tmpText, pt)
		}
	}

	// replace the img source to new source
	t.img = m
}

// countLetter 计算字体宽度
func (t *TitleImage) getLettersLen(ss []rune, fontSize int) int {
	var width int
	for _, s := range ss {
		hm := t.font.HMetric(fixed.Int26_6(fontSize), t.font.Index(s))
		width += int(hm.AdvanceWidth)
	}
	return width
}

func (t *TitleImage) RandDeepColor(addon int) color.RGBA {
	randColor := t.RandColor(addon)
	num, _ := strconv.Atoi(t.sum[22-addon : 22-addon+9])
	increase := float64(30 + num%255)

	red := math.Abs(math.Min(float64(randColor.R)-increase, 255))

	green := math.Abs(math.Min(float64(randColor.G)-increase, 255))
	blue := math.Abs(math.Min(float64(randColor.B)-increase, 255))

	return color.RGBA{R: uint8(red), G: uint8(green), B: uint8(blue), A: uint8(255)}
}

// RandColor get random color. 生成随机颜色.
func (t *TitleImage) RandColor(addon int) color.RGBA {
	num, _ := strconv.Atoi(t.sum[addon : addon+9])
	red := num % 255
	green := num / 1000 % 255
	var blue int
	if (red + green) > 400 {
		blue = 0
	} else {
		blue = 400 - green - red
	}
	if blue > 255 {
		blue = 255
	}
	return color.RGBA{R: uint8(red), G: uint8(green), B: uint8(blue), A: uint8(255)}
}

func loadLocalFont(diyPath string) *truetype.Font {
	// if exist diy font file, then use diy font file
	info, err := os.Stat(diyPath)
	if err == nil && !info.IsDir() {
		phtf, err := os.ReadFile(diyPath)
		phtft, err := freetype.ParseFont(phtf)
		if err == nil {
			return phtft
		}
	}

	fontPaths := findfont.List()
	for _, path := range fontPaths {
		//楷体:simkai.ttf
		//黑体:simhei.ttf
		if strings.Contains(path, "yahei") ||
			strings.Contains(path, "simhei") ||
			strings.Contains(path, "simkai.ttf") ||
			strings.Contains(path, "PingFang.ttc") ||
			strings.Contains(path, "Heiti") ||
			strings.Contains(path, "simsun.ttc") {
			phtf, err := os.ReadFile(path)
			phtft, err := freetype.ParseFont(phtf)
			if err != nil {
				continue
			}

			return phtft
		}
	}
	for _, path := range fontPaths {
		info, err = os.Stat(path)
		if err == nil {
			if info.Size() > 1024*1024*2 {
				phtf, err := os.ReadFile(path)
				phtft, err := freetype.ParseFont(phtf)
				if err != nil {
					continue
				}

				return phtft
			}
		}
	}
	// 英文状态下的默认字体
	for _, path := range fontPaths {
		// 选择解析成功的第一个
		phtf, err := os.ReadFile(path)
		phtft, err := freetype.ParseFont(phtf)
		if err != nil {
			continue
		}

		return phtft
	}

	return nil
}
