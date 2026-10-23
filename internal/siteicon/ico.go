package siteicon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"sort"
)

var errICO = errors.New("unsupported ICO image")

// IsICO tells an icon file by its header
func IsICO(data []byte) bool {
	return len(data) >= 6 && binary.LittleEndian.Uint16(data) == 0 && (binary.LittleEndian.Uint16(data[2:]) == 1 || binary.LittleEndian.Uint16(data[2:]) == 2)
}

// DecodeICO decodes the largest image of an icon file it can, PNG or a 1, 4, 8, 24 or 32 bit bitmap with its mask
func DecodeICO(data []byte) (image.Image, error) {
	if !IsICO(data) {
		return nil, errICO
	}
	type entry struct {
		w, h, bpp    int
		offset, size int
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	entries := []entry{}
	for i := 0; i < count && 6+16*(i+1) <= len(data); i++ {
		e := data[6+16*i:]
		w, h := int(e[0]), int(e[1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		entries = append(entries, entry{w, h, int(binary.LittleEndian.Uint16(e[6:])), int(binary.LittleEndian.Uint32(e[12:])), int(binary.LittleEndian.Uint32(e[8:]))})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.w*a.h != b.w*b.h {
			return a.w*a.h > b.w*b.h
		}
		return a.bpp > b.bpp
	})
	for _, e := range entries {
		if e.offset < 0 || e.size <= 0 || e.offset+e.size > len(data) || e.offset+e.size < e.offset {
			continue
		}
		raw := data[e.offset : e.offset+e.size]
		if bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
			if img, err := png.Decode(bytes.NewReader(raw)); err == nil {
				return img, nil
			}
			continue
		}
		if img, err := decodeDIB(raw); err == nil {
			return img, nil
		}
	}
	return nil, errICO
}

func decodeDIB(raw []byte) (image.Image, error) {
	if len(raw) < 40 {
		return nil, errICO
	}
	header := int(binary.LittleEndian.Uint32(raw))
	w := int(int32(binary.LittleEndian.Uint32(raw[4:])))
	h := int(int32(binary.LittleEndian.Uint32(raw[8:]))) / 2
	bpp := int(binary.LittleEndian.Uint16(raw[14:]))
	compression := binary.LittleEndian.Uint32(raw[16:])
	colors := int(binary.LittleEndian.Uint32(raw[32:]))
	if header < 40 || header > len(raw) || w <= 0 || h <= 0 || w > 1024 || h > 1024 || (compression != 0 && compression != 3) {
		return nil, errICO
	}
	var palette []color.NRGBA
	if bpp <= 8 {
		if colors == 0 || colors > 1<<bpp {
			colors = 1 << bpp
		}
		if header+4*colors > len(raw) {
			return nil, errICO
		}
		for i := 0; i < colors; i++ {
			p := raw[header+4*i:]
			palette = append(palette, color.NRGBA{p[2], p[1], p[0], 255})
		}
	}
	switch bpp {
	case 1, 4, 8, 24, 32:
	default:
		return nil, errICO
	}
	pixels := header + 4*len(palette)
	stride := (w*bpp + 31) / 32 * 4
	maskStride := (w + 31) / 32 * 4
	if pixels+stride*h > len(raw) {
		return nil, errICO
	}
	mask := pixels + stride*h
	hasMask := mask+maskStride*h <= len(raw)
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	alpha := false
	for y := 0; y < h; y++ {
		row := raw[pixels+(h-1-y)*stride:]
		for x := 0; x < w; x++ {
			var c color.NRGBA
			switch bpp {
			case 32:
				c = color.NRGBA{row[4*x+2], row[4*x+1], row[4*x], row[4*x+3]}
				alpha = alpha || c.A != 0
			case 24:
				c = color.NRGBA{row[3*x+2], row[3*x+1], row[3*x], 255}
			default:
				bit := x * bpp
				i := int(row[bit/8]>>(8-bpp-bit%8)) & (1<<bpp - 1)
				if i >= len(palette) {
					return nil, errICO
				}
				c = palette[i]
			}
			img.SetNRGBA(x, y, c)
		}
	}
	// 32 bit images carry their own alpha; the others, and 32 bit ones without any, use the AND mask
	if bpp != 32 || !alpha {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := img.NRGBAAt(x, y)
				c.A = 255
				if hasMask && raw[mask+(h-1-y)*maskStride+x/8]>>(7-x%8)&1 == 1 {
					c.A = 0
				}
				img.SetNRGBA(x, y, c)
			}
		}
	}
	return img, nil
}
