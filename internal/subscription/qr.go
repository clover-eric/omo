package subscription

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/boombuler/barcode/qr"
)

const qrQuietZone = 4

func qrSVG(payload string) ([]byte, error) {
	code, err := qr.Encode(payload, qr.M, qr.Auto)
	if err != nil {
		return nil, err
	}
	bounds := code.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	var b strings.Builder
	size := width + qrQuietZone*2
	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, size, size))
	b.WriteString(`<rect width="100%" height="100%" fill="#fff"/>`)
	b.WriteString(`<path fill="#000" d="`)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if isDark(code.At(bounds.Min.X+x, bounds.Min.Y+y)) {
				b.WriteString(fmt.Sprintf("M%d %dh1v1h-1z", x+qrQuietZone, y+qrQuietZone))
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return []byte(b.String()), nil
}

func isDark(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r+g+b < 0x8000*3
}
