// +build ignore

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	const size = 1024
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	lerp := func(a, b uint8, t float64) uint8 {
		return uint8(float64(a)*(1-t) + float64(b)*t)
	}

	blendColor := func(c1, c2 color.RGBA, t float64) color.RGBA {
		t = math.Max(0, math.Min(1, t))
		return color.RGBA{lerp(c1.R, c2.R, t), lerp(c1.G, c2.G, t), lerp(c1.B, c2.B, t), 255}
	}

	blendOver := func(bg, fg color.RGBA, alpha float64) color.RGBA {
		alpha = math.Max(0, math.Min(1, alpha))
		return color.RGBA{lerp(bg.R, fg.R, alpha), lerp(bg.G, fg.G, alpha), lerp(bg.B, fg.B, alpha), 255}
	}

	dist := func(x1, y1, x2, y2 float64) float64 {
		dx := x2 - x1
		dy := y2 - y1
		return math.Sqrt(dx*dx + dy*dy)
	}

	inRoundedRect := func(x, y, rx1, ry1, rx2, ry2, r float64) bool {
		cx := math.Max(rx1+r, math.Min(x, rx2-r))
		cy := math.Max(ry1+r, math.Min(y, ry2-r))
		dx := x - cx
		dy := y - cy
		return dx*dx+dy*dy <= r*r
	}

	smoothstep := func(edge0, edge1, x float64) float64 {
		t := (x - edge0) / (edge1 - edge0)
		t = math.Max(0, math.Min(1, t))
		return t * t * (3 - 2*t)
	}

	distToSegment := func(px, py, x1, y1, x2, y2 float64) float64 {
		dx := x2 - x1
		dy := y2 - y1
		lenSq := dx*dx + dy*dy
		if lenSq == 0 {
			return dist(px, py, x1, y1)
		}
		t := ((px-x1)*dx + (py-y1)*dy) / lenSq
		t = math.Max(0, math.Min(1, t))
		return dist(px, py, x1+t*dx, y1+t*dy)
	}

	bgDeep := color.RGBA{8, 8, 18, 255}
	bgMid := color.RGBA{18, 18, 36, 255}

	margin := 24.0
	radius := 200.0

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx := float64(x)
			fy := float64(y)

			if !inRoundedRect(fx, fy, margin, margin, float64(size)-margin, float64(size)-margin, radius) {
				img.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
				continue
			}

			t := (fx + fy) / float64(2*size)
			bg := blendColor(bgDeep, bgMid, t)

			// --- GLASS CARD ---
			cardX1, cardY1 := 140.0, 200.0
			cardX2, cardY2 := 884.0, 824.0
			cardR := 52.0

			if inRoundedRect(fx, fy, cardX1, cardY1, cardX2, cardY2, cardR) {
				bg = blendOver(bg, color.RGBA{255, 255, 255, 255}, 0.035)

				if !inRoundedRect(fx, fy, cardX1+1.5, cardY1+1.5, cardX2-1.5, cardY2-1.5, cardR-1.5) {
					bg = blendOver(bg, color.RGBA{255, 255, 255, 255}, 0.10)
				}

				// ======================
				// LEFT SIDE: Bold "T"
				// ======================

				// T horizontal bar
				tBarX1, tBarY1 := 180.0, 330.0
				tBarX2, tBarY2 := 450.0, 400.0
				tBarR := 35.0

				if inRoundedRect(fx, fy, tBarX1, tBarY1, tBarX2, tBarY2, tBarR) {
					gt := (fx - tBarX1) / (tBarX2 - tBarX1)
					letterColor := blendColor(
						color.RGBA{100, 60, 255, 255},
						color.RGBA{0, 200, 255, 255},
						gt,
					)
					edgeDist := math.Min(math.Min(fx-tBarX1, tBarX2-fx), math.Min(fy-tBarY1, tBarY2-fy))
					alpha := smoothstep(0, 2.5, edgeDist)
					bg = blendOver(bg, letterColor, alpha)
				}

				// T vertical stem
				tStemX1, tStemY1 := 275.0, 385.0
				tStemX2, tStemY2 := 355.0, 700.0
				tStemR := 35.0

				if inRoundedRect(fx, fy, tStemX1, tStemY1, tStemX2, tStemY2, tStemR) {
					gt := (fy - tStemY1) / (tStemY2 - tStemY1)
					letterColor := blendColor(
						color.RGBA{60, 140, 255, 255},
						color.RGBA{120, 40, 255, 255},
						gt,
					)
					edgeDist := math.Min(math.Min(fx-tStemX1, tStemX2-fx), math.Min(fy-tStemY1, tStemY2-fy))
					alpha := smoothstep(0, 2.5, edgeDist)
					bg = blendOver(bg, letterColor, alpha)
				}

				// Glow behind T
				tGlowDist := dist(fx, fy, 315, 520)
				if tGlowDist < 200 {
					bg = blendOver(bg, color.RGBA{80, 40, 255, 255}, (1-tGlowDist/200)*0.06)
				}

				// --- Separator line ---
				sepX := 514.0
				sepDist := math.Abs(fx - sepX)
				if sepDist < 1.2 && fy > 360 && fy < 680 {
					sepFade := 1.0
					if fy < 400 {
						sepFade = (fy - 360) / 40
					} else if fy > 640 {
						sepFade = (680 - fy) / 40
					}
					bg = blendOver(bg, color.RGBA{255, 255, 255, 255}, 0.08*sepFade)
				}

				// ======================
				// RIGHT SIDE: CHEVRON >_
				// ======================

				chevX1, chevY1 := 560.0, 360.0
				chevX2, chevY2 := 700.0, 512.0
				chevX3, chevY3 := 560.0, 664.0
				chevStroke := 42.0

				cd1 := distToSegment(fx, fy, chevX1, chevY1, chevX2, chevY2)
				cd2 := distToSegment(fx, fy, chevX2, chevY2, chevX3, chevY3)
				chevDist := math.Min(cd1, cd2)

				if chevDist < chevStroke {
					gt := (fx - 540) / 200
					gt = math.Max(0, math.Min(1, gt))
					chevColor := blendColor(
						color.RGBA{0, 212, 255, 255},
						color.RGBA{0, 255, 200, 255},
						gt,
					)
					alpha := smoothstep(0, 2.5, chevStroke-chevDist)
					bg = blendOver(bg, chevColor, alpha)
				}

				// Glow around chevron
				if chevDist >= chevStroke && chevDist < chevStroke+50 {
					glowA := (1 - (chevDist-chevStroke)/50) * 0.12
					bg = blendOver(bg, color.RGBA{0, 212, 255, 255}, glowA)
				}

				// Cursor underscore
				curX1, curY1 := 730.0, 640.0
				curX2, curY2 := 840.0, 680.0
				curR := 10.0
				if inRoundedRect(fx, fy, curX1, curY1, curX2, curY2, curR) {
					gt := (fx - curX1) / (curX2 - curX1)
					curColor := blendColor(
						color.RGBA{0, 212, 255, 255},
						color.RGBA{0, 255, 200, 255},
						gt,
					)
					edgeDist := math.Min(math.Min(fx-curX1, curX2-fx), math.Min(fy-curY1, curY2-fy))
					alpha := smoothstep(0, 2, edgeDist)
					bg = blendOver(bg, curColor, alpha)
				}

			}

			// Outer icon border
			if !inRoundedRect(fx, fy, margin+2, margin+2, float64(size)-margin-2, float64(size)-margin-2, radius-2) {
				bg = blendOver(bg, color.RGBA{255, 255, 255, 255}, 0.05)
			}

			img.SetRGBA(x, y, bg)
		}
	}

	f, _ := os.Create("build/appicon.png")
	defer f.Close()
	png.Encode(f, img)

	f2, _ := os.Create("assets/icon.png")
	defer f2.Close()
	png.Encode(f2, img)

	// Generate .ico with multiple sizes
	icoSizes := []int{256, 128, 64, 48, 32, 16}
	var pngDatas [][]byte

	for _, s := range icoSizes {
		resized := resizeImage(img, s)
		var buf bytes.Buffer
		png.Encode(&buf, resized)
		pngDatas = append(pngDatas, buf.Bytes())
	}

	writeICO("build/windows/icon.ico", icoSizes, pngDatas)
}

func resizeImage(src *image.RGBA, newSize int) *image.RGBA {
	srcSize := src.Bounds().Dx()
	dst := image.NewRGBA(image.Rect(0, 0, newSize, newSize))
	scale := float64(srcSize) / float64(newSize)

	for y := 0; y < newSize; y++ {
		for x := 0; x < newSize; x++ {
			// Bilinear sampling
			srcX := (float64(x) + 0.5) * scale - 0.5
			srcY := (float64(y) + 0.5) * scale - 0.5

			x0 := int(math.Floor(srcX))
			y0 := int(math.Floor(srcY))
			x1 := x0 + 1
			y1 := y0 + 1

			if x0 < 0 { x0 = 0 }
			if y0 < 0 { y0 = 0 }
			if x1 >= srcSize { x1 = srcSize - 1 }
			if y1 >= srcSize { y1 = srcSize - 1 }

			fx := srcX - float64(x0)
			fy := srcY - float64(y0)

			c00 := src.RGBAAt(x0, y0)
			c10 := src.RGBAAt(x1, y0)
			c01 := src.RGBAAt(x0, y1)
			c11 := src.RGBAAt(x1, y1)

			mix := func(a, b uint8, t float64) uint8 {
				return uint8(float64(a)*(1-t) + float64(b)*t)
			}

			top := color.RGBA{mix(c00.R, c10.R, fx), mix(c00.G, c10.G, fx), mix(c00.B, c10.B, fx), mix(c00.A, c10.A, fx)}
			bot := color.RGBA{mix(c01.R, c11.R, fx), mix(c01.G, c11.G, fx), mix(c01.B, c11.B, fx), mix(c01.A, c11.A, fx)}
			final := color.RGBA{mix(top.R, bot.R, fy), mix(top.G, bot.G, fy), mix(top.B, bot.B, fy), mix(top.A, bot.A, fy)}

			dst.SetRGBA(x, y, final)
		}
	}
	return dst
}

func writeICO(path string, sizes []int, pngDatas [][]byte) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	numImages := len(sizes)

	// ICO header: 6 bytes
	writeU16 := func(v uint16) { f.Write([]byte{byte(v), byte(v >> 8)}) }
	writeU32 := func(v uint32) { f.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}) }

	writeU16(0)                // reserved
	writeU16(1)                // type: ICO
	writeU16(uint16(numImages)) // count

	// Directory entries: 16 bytes each
	offset := uint32(6 + 16*numImages)
	for i, s := range sizes {
		w := byte(s)
		h := byte(s)
		if s == 256 {
			w = 0 // 0 means 256 in ICO format
			h = 0
		}
		f.Write([]byte{w, h, 0, 0}) // width, height, palette, reserved
		writeU16(1)                  // color planes
		writeU16(32)                 // bits per pixel
		writeU32(uint32(len(pngDatas[i]))) // size of PNG data
		writeU32(offset)             // offset to PNG data
		offset += uint32(len(pngDatas[i]))
	}

	// PNG data
	for _, data := range pngDatas {
		f.Write(data)
	}
}
