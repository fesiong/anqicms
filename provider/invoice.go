package provider

import (
	"errors"
	"fmt"
	"github.com/signintech/gopdf"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/webp"
	"image"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (w *Website) GenerateInvoice(order *model.Order) (string, error) {
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()
	currentY := float64(60)

	fontPath := config.ExecPath + "source/Arial.ttf"
	err := pdf.AddTTFFont("Arial", fontPath)
	if err != nil {
		return "", err
	}
	fontPath = config.ExecPath + "source/Arial Bold.ttf"
	err = pdf.AddTTFFontWithOption("Arial", fontPath, gopdf.TtfOption{
		Style: gopdf.Bold,
	})
	if err != nil {
		return "", err
	}
	err = pdf.SetFont("Arial", "B", 32)
	if err != nil {
		return "", err
	}
	pdf.SetY(currentY)
	pdf.SetX(gopdf.PageSizeA4.W/2 - 50)
	pdf.Text("INVOICE") // 标题居中

	// set unpay
	pdf.SetXY(gopdf.PageSizeA4.W-250, 60)
	pdf.SetFontSize(24)
	pdf.SetTextColor(255, 255, 255)
	pdf.Rotate(-40, gopdf.PageSizeA4.W-70, 150)
	pdf.SetLineWidth(2)
	if order.PaidTime == 0 {
		// unpaid
		pdf.SetStrokeColor(171, 50, 44)
		pdf.SetFillColor(224, 85, 74)
		pdf.Rectangle(gopdf.PageSizeA4.W-250, 50, gopdf.PageSizeA4.W, 90, "DF", 0, 0)
		pdf.CellWithOption(&gopdf.Rect{W: 250, H: 40}, "UNPAID", gopdf.CellOption{
			Align: gopdf.Center,
		})
	} else {
		// paid
		pdf.SetStrokeColor(110, 192, 71)
		pdf.SetFillColor(151, 223, 75)
		pdf.Rectangle(gopdf.PageSizeA4.W-250, 50, gopdf.PageSizeA4.W, 90, "DF", 0, 0)
		pdf.CellWithOption(&gopdf.Rect{W: 250, H: 40}, "PAID", gopdf.CellOption{
			Align: gopdf.Center,
		})
	}
	pdf.RotateReset() //reset
	pdf.SetTextColor(0, 0, 0)
	//end
	currentY += 30
	logo, _ := w.GetLogoPath()
	if logo != "" {
		logof, err := os.Open(logo)
		if err != nil {
			log.Print(err.Error())
			return "", err
		}
		defer logof.Close()
		img, _, err := image.Decode(logof)
		if err != nil {
			logof.Seek(0, 0)
			img, err = webp.Decode(logof)
		}
		if err == nil {
			img = library.Resize(img, 0, 80)
			pdf.ImageFrom(img, 40, currentY, nil)
		} else if strings.HasSuffix(logo, ".svg") {
			icon, err := oksvg.ReadIcon(logo)
			if err != nil {
				panic(err)
			}
			ww := 160
			hh := 80
			icon.SetTarget(0, 0, float64(ww), float64(hh))

			// 创建RGBA图像
			rgba := image.NewRGBA(image.Rect(0, 0, ww, hh))

			// 绘制到图像上
			scanner := rasterx.NewScannerGV(ww, hh, rgba, rgba.Bounds())
			dasher := rasterx.NewDasher(ww, hh, scanner)
			icon.Draw(dasher, 1.0) // 1.0是不透明度
			// other format
			pdf.ImageFrom(rgba, 40, currentY, nil)
		}
	}
	userInfo, err := w.GetUserInfoById(order.UserId)
	if err == nil {
		currentY += 80
		pdf.SetY(currentY)
		pdf.SetX(40)
		pdf.SetFont("Arial", "B", 16)
		if userInfo.RealName != "" {
			pdf.Text(userInfo.RealName)
		} else {
			pdf.Text(userInfo.UserName)
		}
	}
	currentY += 20
	pdf.SetY(currentY)
	pdf.SetX(40)
	pdf.SetFont("Arial", "", 12)
	pdf.Text("Date created: " + time.Unix(order.CreatedTime, 10).Format("Jan 1,2006"))
	currentY += 20
	pdf.SetY(currentY)
	pdf.SetX(40)
	pdf.Text("Invoice No: #" + strconv.FormatInt(int64(order.Id), 10))
	currentY += 20
	pdf.SetY(currentY)
	pdf.SetX(40)
	pdf.Text("Order No: #" + order.OrderId)
	if order.ExpressCompany != "" {
		currentY += 20
		pdf.SetY(currentY)
		pdf.SetX(40)
		pdf.Text("Shipping Method: " + order.ExpressCompany)
	}

	if order.OrderAddress != nil {
		// Address
		pdf.SetFont("Arial", "B", 12)
		currentY += 40
		pdf.SetY(currentY)
		pdf.SetX(40)
		pdf.Text("From Address")
		pdf.SetX(gopdf.PageSizeA4.W / 2)
		pdf.Text("Shipping Address")

		pdf.SetFont("Arial", "", 10)
		currentY += 10
		pdf.SetY(currentY)
		pdf.SetX(40)
		pdf.Cell(&gopdf.Rect{W: (gopdf.PageSizeA4.W-80)/2 - 30, H: 20}, w.Contact.UserName)
		pdf.SetX(gopdf.PageSizeA4.W / 2)
		pdf.Cell(&gopdf.Rect{W: (gopdf.PageSizeA4.W-80)/2 - 30, H: 20}, order.OrderAddress.Name)

		currentY += 18
		pdf.SetXY(40, currentY)
		pdf.MultiCell(&gopdf.Rect{W: (gopdf.PageSizeA4.W-80)/2 - 30, H: 60}, w.Contact.Address+" "+w.Contact.Email)
		pdf.SetXY((gopdf.PageSizeA4.W-80)/3+40, currentY)
		pdf.MultiCell(&gopdf.Rect{W: (gopdf.PageSizeA4.W-80)/2 - 30, H: 60}, order.OrderAddress.AddressInfo+", "+order.OrderAddress.Country+", "+order.OrderAddress.City+", "+order.OrderAddress.Province+", "+order.OrderAddress.Postcode+", "+order.OrderAddress.Phone)
	}

	// table
	currentY += 50
	table := pdf.NewTableLayout(40, currentY, 40, len(order.Details))
	table.AddColumn("QTY.", 40, "left")
	table.AddColumn("NAME", 260, "left")
	table.AddColumn("DATE", 100, "center")
	table.AddColumn("DISCOUNT", 60, "center")
	table.AddColumn("PRICE", 60, "right")

	for _, detail := range order.Details {
		var goodsName string
		if detail.Goods != nil {
			goodsName = detail.Goods.Title
		} else if detail.Group != nil {
			goodsName = detail.Group.Title
		}
		if detail.GoodsSku != nil {
			goodsName += "\n" + detail.GoodsSku.Title
		}
		discount := float64(detail.OriginPrice-detail.Price) / 100
		table.AddRow([]string{fmt.Sprintf("%dx", detail.Quantity), goodsName, time.Unix(detail.CreatedTime, 0).Format("Jan 1, 2006"), fmt.Sprintf("%.2f", discount), fmt.Sprintf("%.2f", float64(detail.Price)/100)})
	}
	table.AddRow([]string{"", "", "", "Shipping", fmt.Sprintf("%.2f", float64(order.ShippingFee)/100)})
	table.AddRow([]string{"", "", "", "Total", fmt.Sprintf("%.2f", float64(order.Amount)/100)})

	table.SetTableStyle(gopdf.CellStyle{
		BorderStyle: gopdf.BorderStyle{
			Top:      false,
			Left:     false,
			Bottom:   true,
			Right:    false,
			Width:    2,
			RGBColor: gopdf.RGBColor{R: 0, G: 0, B: 0},
		},
		FillColor: gopdf.RGBColor{R: 255, G: 255, B: 255},
		TextColor: gopdf.RGBColor{R: 0, G: 0, B: 0},
		FontSize:  14,
	})
	table.SetHeaderStyle(gopdf.CellStyle{
		BorderStyle: gopdf.BorderStyle{
			Top:      true,
			Left:     false,
			Bottom:   true,
			Right:    false,
			Width:    2.0,
			RGBColor: gopdf.RGBColor{R: 0, G: 0, B: 0},
		},
		FillColor: gopdf.RGBColor{R: 255, G: 255, B: 255},
		TextColor: gopdf.RGBColor{R: 0, G: 0, B: 0},
		FontSize:  16,
	})

	table.SetCellStyle(gopdf.CellStyle{
		BorderStyle: gopdf.BorderStyle{
			Right:    false,
			Bottom:   true,
			Width:    0.5,
			RGBColor: gopdf.RGBColor{R: 0, G: 0, B: 0},
		},
		FillColor: gopdf.RGBColor{R: 255, G: 255, B: 255},
		TextColor: gopdf.RGBColor{R: 0, G: 0, B: 0},
		FontSize:  14,
	})
	table.DrawTable()

	payment, err := w.GetPaymentInfoByOrderId(order.OrderId)
	if err == nil {
		currentY += 40 * (5 + 4)
		pdf.SetFont("Arial", "B", 12)
		pdf.SetXY(40, currentY)
		pdf.Text("Payment Method: ")
		pdf.SetFont("Arial", "", 12)
		pdf.Text(strings.Title(payment.PayWay))
	}
	// output pdf to data/invoices/orderId.pdf
	pdfPath := fmt.Sprintf("%sinvoices/%s.pdf", w.DataPath, order.OrderId)
	_, err = os.Stat(filepath.Dir(pdfPath))
	if err != nil && os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(pdfPath), os.ModePerm)
	}

	err = pdf.WritePdf(pdfPath)
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(pdfPath, w.DataPath), nil
}

func (w *Website) GetLogoPath() (string, error) {
	logo := w.System.SiteLogo
	tmpPath := w.CachePath + "cache/tmp-logo.png"
	info, err := os.Stat(tmpPath)
	// cache 1 day
	if err == nil && info.ModTime().After(time.Now().AddDate(0, 0, -1)) {
		return tmpPath, nil
	}

	if logo != "" {
		var logoPath string
		logo = strings.TrimPrefix(logo, w.PluginStorage.StorageUrl)
		if strings.HasPrefix(logo, "http") {
			resp, err := library.GetURLData(logo, "", 10)
			if err != nil {
				log.Print(err.Error())
				return "", err
			}
			// write logo
			err = os.WriteFile(tmpPath, []byte(resp.Body), os.ModePerm)
			if err != nil {
				log.Print(err.Error())
				return "", err
			}
			logoPath = tmpPath
		} else {
			_, err := os.Stat(w.PublicPath + logo)
			if err == nil {
				logoPath = w.PublicPath + logo
			} else {
				// remote file
				logoUrl := w.PluginStorage.StorageUrl + logo
				resp, err := library.GetURLData(logoUrl, "", 10)
				if err != nil {
					log.Print(err.Error())
					return "", err
				}
				err = os.WriteFile(tmpPath, []byte(resp.Body), os.ModePerm)
				if err != nil {
					log.Print(err.Error())
					return "", err
				}
				logoPath = tmpPath
			}
		}

		return logoPath, nil
	}

	return "", errors.New("empty logo")
}
