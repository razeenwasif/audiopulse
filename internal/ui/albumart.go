package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // register JPEG decoder (Spotify covers)
	_ "image/png"  // register PNG decoder
	"net/http"
	"strings"
	"time"
)

// artCellW is the album-art width in terminal cells. The height is derived from
// the terminal's cell aspect ratio (see artDims) so a square cover looks square.
const artCellW = 22

// fetchAlbumArt downloads the cover at url and renders it as half-block cells.
func fetchAlbumArt(ctx context.Context, url string, cellW, cellH int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return "", err
	}
	return halfBlocks(img, cellW, cellH), nil
}

// halfBlocks renders img as cellW×cellH terminal cells. Each cell uses the
// upper-half block "▀": its foreground is the top pixel and its background the
// bottom pixel, doubling vertical resolution. Colors use 24-bit ANSI.
func halfBlocks(img image.Image, cellW, cellH int) string {
	bnd := img.Bounds()
	iw, ih := bnd.Dx(), bnd.Dy()
	if iw == 0 || ih == 0 || cellW < 1 || cellH < 1 {
		return ""
	}
	pxH := cellH * 2

	var sb strings.Builder
	for cy := 0; cy < cellH; cy++ {
		for cx := 0; cx < cellW; cx++ {
			sx := bnd.Min.X + cx*iw/cellW
			tyTop := bnd.Min.Y + (cy*2)*ih/pxH
			tyBot := bnd.Min.Y + (cy*2+1)*ih/pxH
			tr, tg, tb := rgb(img.At(sx, tyTop))
			br, bg, bb := rgb(img.At(sx, tyBot))
			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", tr, tg, tb, br, bg, bb)
		}
		sb.WriteString("\x1b[0m")
		if cy < cellH-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func rgb(c color.Color) (int, int, int) {
	r, g, b, _ := c.RGBA()
	return int(r >> 8), int(g >> 8), int(b >> 8)
}
