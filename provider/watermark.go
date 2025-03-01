package provider

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/webp"
	"image"
	"image/color"
	"image/draw"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

type Watermark struct {
	PublicPath string
	config     *config.PluginWatermark
	useWebp    int
	font       *truetype.Font
	fontSize   int
	watermark  image.Image
}

func (w *Website) NewWatermark(cfg *config.PluginWatermark) *Watermark {
	if !cfg.Open {
		// 未开启，不做处理
		return nil
	}
	rand.Seed(time.Now().UnixNano())
	f := loadLocalFont(w.PublicPath + cfg.FontPath)
	fontSize := cfg.Size
	if fontSize == 0 {
		fontSize = 20
	}
	t := Watermark{
		PublicPath: w.PublicPath,
		config:     cfg,
		useWebp:    w.Content.UseWebp,
		font:       f,
		fontSize:   fontSize,
	}
	if t.config.Type == 0 {
		// image
		watermarkFile := t.PublicPath + t.config.ImagePath
		wf, err := os.Open(watermarkFile)
		if err != nil {
			return nil
		}
		defer wf.Close()
		img, _, err := image.Decode(wf)
		if err != nil {
			// 尝试用webp解析
			wf.Seek(0, 0)
			img, err = webp.Decode(wf)
			if err != nil {
				return nil
			}
		}
		t.watermark = img
	}

	return &t
}

func (w *Website) GenerateAllWatermark() {
	// 根据attachment表读取每一张图片
	lastId := uint(0)
	limit := 500
	wm := w.NewWatermark(w.PluginWatermark)
	if wm == nil {
		return
	}
	for {
		var attaches []model.Attachment
		w.DB.Where("`id` > ?", lastId).Order("id asc").Limit(limit).Find(&attaches)
		if len(attaches) == 0 {
			break
		}
		lastId = attaches[len(attaches)-1].Id
		for _, v := range attaches {
			if v.IsImage == 0 || v.Watermark == 1 {
				continue
			}
			_ = w.addWatermark(wm, &v)
		}

	}

	log.Println("finished generate watermark")
}

func (w *Website) addWatermark(wm *Watermark, attachment *model.Attachment) error {
	// 先处理原图
	originPath := w.PublicPath + attachment.FileLocation

	wf, err := os.Open(originPath)
	if err != nil {
		return nil
	}
	defer wf.Close()
	img, imgType, err := image.Decode(wf)
	if err != nil {
		// 尝试用webp解析
		wf.Seek(0, 0)
		img, err = webp.Decode(wf)
		if err != nil {
			return errors.New(w.Tr("UnableToObtainImageSize"))
		}
		imgType = "webp"
	}
	if imgType == "jpeg" {
		imgType = "jpg"
	}
	quality := w.Content.Quality
	if quality == 0 {
		// 默认质量是90
		quality = config.DefaultQuality
	}

	// gif 不处理
	if imgType == "gif" {
		return errors.New(w.Tr("NotProcessingGif"))
	}
	img, err = wm.DrawWatermark(img)
	if err != nil {
		return err
	}
	buf, _, _ := encodeImage(img, imgType, quality)

	err = os.WriteFile(originPath, buf, os.ModePerm)
	if err != nil {
		return err
	}
	_, err = w.Storage.UploadFile(attachment.FileLocation, buf)
	if err != nil {
		return err
	}
	// 修改数据库
	attachment.Watermark = 1
	w.DB.Model(attachment).UpdateColumn("watermark", attachment.Watermark)

	// 缩略图
	paths, fileName := filepath.Split(attachment.FileLocation)
	thumbPath := w.PublicPath + paths + "thumb_" + fileName

	newImg := library.ThumbnailCrop(w.Content.ThumbWidth, w.Content.ThumbHeight, img, w.Content.ThumbCrop)
	buf, _, _ = encodeImage(newImg, imgType, quality)

	err = os.WriteFile(thumbPath, buf, os.ModePerm)
	if err != nil {
		return err
	}
	_, err = w.Storage.UploadFile(paths+"thumb_"+fileName, buf)
	if err != nil {
		return err
	}

	return nil
}

func (t *Watermark) DrawWatermark(m image.Image) (image.Image, error) {
	// 小于一定尺寸的不添加水印
	if m.Bounds().Dy() < t.config.MinSize && m.Bounds().Dx() < t.config.MinSize {
		return m, errors.New("小尺寸不处理")
	}
	dst := image.NewNRGBA(m.Bounds())
	draw.Draw(dst, dst.Bounds(), m, image.Point{}, draw.Src)
	maxHeight := m.Bounds().Dy() * t.fontSize / 100
	maxWidth := m.Bounds().Dx() * t.fontSize / 100
	edge := m.Bounds().Dx() / 15
	if edge > 50 {
		edge = 50
	}
	// position = 5 居中，1 左上角，3 右上角 7 左下角 9 右下角
	// 开始绘制图片或文字
	var markImg draw.Image
	if t.config.Type == 0 {
		markImg = imaging.Fit(t.watermark, maxWidth, maxHeight, imaging.Lanczos)
	} else {
		// 开始绘制文字
		// 先计算文字大小，中文算一个，英文算0.5个，约等于字体大小
		maxWidth = t.getLettersLen([]rune(t.config.Text), t.fontSize)
		maxHeight = t.fontSize

		nc := freetype.NewContext()
		nc.SetDPI(72)
		markImg = image.NewNRGBA(image.Rect(0, 0, maxWidth+int(float64(maxWidth)*0.1), maxHeight+int(float64(maxHeight)*0.1)))
		nc.SetClip(markImg.Bounds())
		nc.SetDst(markImg)
		nc.SetHinting(font.HintingFull)
		nc.SetFont(t.font)
		nc.SetFontSize(float64(t.fontSize))
		nc.SetSrc(image.NewUniform(library.HEXToRGB(t.config.Color)))

		pt := freetype.Pt(0, maxHeight)
		_, _ = nc.DrawString(t.config.Text, pt)
	}
	// 开始存放位置，默认 右下角
	x := m.Bounds().Dx() - edge - markImg.Bounds().Dx()
	y := m.Bounds().Dy() - edge - markImg.Bounds().Dy()
	if t.config.Position == 5 {
		// 居中
		x = m.Bounds().Dx()/2 - markImg.Bounds().Dx()/2
		y = m.Bounds().Dy()/2 - markImg.Bounds().Dy()/2
	} else if t.config.Position == 1 {
		// 左上角
		x = edge
		y = edge
	} else if t.config.Position == 3 {
		// 右上角
		x = m.Bounds().Dx() - edge - markImg.Bounds().Dx()
		y = edge
	} else if t.config.Position == 7 {
		// 左下角
		x = edge
		y = m.Bounds().Dy() - edge - markImg.Bounds().Dy()
	}
	if t.config.Opacity < 100 {
		newRgba := image.NewNRGBA(markImg.Bounds())
		dx := markImg.Bounds().Dx()
		dy := markImg.Bounds().Dy()
		for i := 0; i < dx; i++ {
			for j := 0; j < dy; j++ {
				colorRgb := markImg.At(i, j)
				r, g, b, a := colorRgb.RGBA()
				if a > 0 {
					opacity := uint16(float64(a) * float64(t.config.Opacity) / 100)
					//颜色模型转换，至关重要！
					v := newRgba.ColorModel().Convert(color.NRGBA64{R: uint16(r), G: uint16(g), B: uint16(b), A: opacity})
					//Alpha = 0: Full transparent
					rr, gg, bb, aa := v.RGBA()
					newRgba.SetRGBA64(i, j, color.RGBA64{R: uint16(rr), G: uint16(gg), B: uint16(bb), A: uint16(aa)})
				}
			}
		}
		markImg = newRgba
	}

	draw.Draw(dst, image.Rect(x, y, x+markImg.Bounds().Dx(), y+markImg.Bounds().Dy()), markImg, image.Point{}, draw.Over)

	return dst, nil
}

func (t *Watermark) DrawWatermarkPreview() string {
	// 生成一个纯色预览图
	m := image.NewRGBA(image.Rect(0, 0, 800, 600))
	bgColor := color.RGBA{R: 96, G: 125, B: 139, A: 255}
	draw.Draw(m, m.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)
	img, _ := t.DrawWatermark(m)

	return t.EncodeB64string(img)
}

func (t *Watermark) EncodeB64string(img image.Image) string {
	buf, _, _ := encodeImage(img, "webp", config.DefaultQuality)
	return fmt.Sprintf("data:%s;base64,%s", "image/webp", base64.StdEncoding.EncodeToString(buf))
}

// countLetter 计算字体宽度
func (t *Watermark) getLettersLen(ss []rune, fontSize int) int {
	var width int
	for _, s := range ss {
		hm := t.font.HMetric(fixed.Int26_6(fontSize), t.font.Index(s))
		width += int(hm.AdvanceWidth)
	}
	return width
}
