package siteicon

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginStripsPathAndQuery(t *testing.T) {
	for raw, want := range map[string]string{
		"https://api.example.com/v1/chat":     "https://api.example.com",
		"https://api.example.com:8443/openai": "https://api.example.com:8443",
		"HTTPS://API.Example.com:443/v1?x=1":  "https://api.example.com",
		" http://127.0.0.1:8317/v1 ":          "http://127.0.0.1:8317",
	} {
		if got, ok := Origin(raw); !ok || got != want {
			t.Fatalf("Origin(%q) = %q %v, want %q", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"not a url", "ftp://example.com", "https://"} {
		if _, ok := Origin(raw); ok {
			t.Fatalf("Origin(%q) accepted", raw)
		}
	}
}

func TestParseIconHrefsRanksAndResolves(t *testing.T) {
	page := `
      <html><head>
        <link rel="stylesheet" href="/app.css" />
        <link rel="icon" href="/assets/mark.png" sizes="32x32" />
        <link rel="apple-touch-icon" href="/apple.png" />
        <link rel="shortcut icon" href="https://cdn.example.com/fav.ico" />
        <link rel="mask-icon" href="/mask.svg" />
        <LINK REL='icon' HREF='/q.png?a=1&amp;b=2'>
      </head></html>`
	hrefs := ParseIconHrefs(page, "https://api.example.com/docs")
	if len(hrefs) == 0 || hrefs[0] != "https://api.example.com/assets/mark.png" {
		t.Fatalf("best icon first: %v", hrefs)
	}
	want := map[string]bool{"https://cdn.example.com/fav.ico": true, "https://api.example.com/apple.png": true, "https://api.example.com/q.png?a=1&b=2": true}
	for _, h := range hrefs {
		delete(want, h)
		if h == "https://api.example.com/app.css" || h == "https://api.example.com/mask.svg" {
			t.Fatalf("non icon listed: %v", hrefs)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing %v in %v", want, hrefs)
	}
}

func TestIsHTMLAndLooksImage(t *testing.T) {
	if !IsHTML("text/html; charset=utf-8", "") || IsHTML("application/json", "<html>") || !IsHTML("", "<!DOCTYPE html><html>") || IsHTML("text/plain", "ok") {
		t.Fatal("html detection")
	}
	if !LooksImage(200, "image/png", []byte{1}) || !LooksImage(200, "", []byte{1}) || LooksImage(200, "text/html", []byte{1}) || LooksImage(404, "image/png", []byte{1}) || LooksImage(200, "image/png", nil) {
		t.Fatal("image detection")
	}
}

func pngBytes(t *testing.T, size int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	img.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func ico(images ...[]byte) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, [3]uint16{0, 1, uint16(len(images))})
	offset := 6 + 16*len(images)
	for _, img := range images {
		w := byte(0)
		if bytes.HasPrefix(img, []byte("\x89PNG")) {
			w = 64
		} else {
			w = byte(binary.LittleEndian.Uint32(img[4:]))
		}
		b.Write([]byte{w, w, 0, 0})
		binary.Write(&b, binary.LittleEndian, [2]uint16{1, 32})
		binary.Write(&b, binary.LittleEndian, [2]uint32{uint32(len(img)), uint32(offset)})
		offset += len(img)
	}
	for _, img := range images {
		b.Write(img)
	}
	return b.Bytes()
}

// dib is a 2×2 bitmap whose top left pixel is blue and opaque
func dib(bpp int) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, struct {
		Size          uint32
		W, H          int32
		Planes, Bpp   uint16
		Compression   uint32
		Image         uint32
		XPPM, YPPM    int32
		Used, Needful uint32
	}{40, 2, 4, 1, uint16(bpp), 0, 0, 0, 0, 0, 0})
	switch bpp {
	case 32:
		// rows bottom up; alpha 0 except the top left pixel
		b.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0})
		b.Write([]byte{255, 0, 0, 255, 0, 0, 0, 0})
	case 1:
		b.Write([]byte{0, 0, 0, 0, 255, 0, 0, 0})
		b.Write([]byte{0, 0, 0, 0})
		b.Write([]byte{0x80, 0, 0, 0})
	}
	// AND mask, bottom up: everything transparent but the top left pixel
	b.Write([]byte{0xC0, 0, 0, 0})
	b.Write([]byte{0x40, 0, 0, 0})
	return b.Bytes()
}

func TestDecodeICO(t *testing.T) {
	img, err := DecodeICO(ico(dib(32), pngBytes(t, 64)))
	if err != nil || img.Bounds().Dx() != 64 {
		t.Fatalf("largest PNG entry not chosen: %v %v", err, img)
	}
	for _, bpp := range []int{32, 1} {
		img, err = DecodeICO(ico(dib(bpp)))
		if err != nil {
			t.Fatal(bpp, err)
		}
		top := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
		other := color.NRGBAModel.Convert(img.At(1, 1)).(color.NRGBA)
		if top != (color.NRGBA{0, 0, 255, 255}) || other.A != 0 {
			t.Fatalf("%d bit pixels: %v %v", bpp, top, other)
		}
	}
	if _, err = DecodeICO([]byte("not an icon")); err == nil {
		t.Fatal("garbage decoded")
	}
}

func TestResolveFollowsLinkedIconsThenFavicon(t *testing.T) {
	good := pngBytes(t, 16)
	usable := func(b []byte) bool { _, _, err := image.Decode(bytes.NewReader(b)); return err == nil }
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><link rel="icon" href="/broken.png" sizes="64x64"><link rel="icon" href="/good.png"></head></html>`))
	})
	mux.HandleFunc("/broken.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("not a png"))
	})
	mux.HandleFunc("/good.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(good)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	got := Resolve(context.Background(), server.Client(), server.URL, usable)
	if !got.Reached || !bytes.Equal(got.Icon, good) {
		t.Fatalf("linked icon not used: %+v", got.Reached)
	}

	api := http.NewServeMux()
	api.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.Write(ico(pngBytes(t, 32)))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	apiServer := httptest.NewServer(api)
	defer apiServer.Close()
	got = Resolve(context.Background(), apiServer.Client(), apiServer.URL, func(b []byte) bool { return IsICO(b) })
	if !got.Reached || !IsICO(got.Icon) {
		t.Fatal("favicon fallback missing")
	}

	apiServer.Close()
	got = Resolve(context.Background(), apiServer.Client(), apiServer.URL, usable)
	if got.Reached || got.Icon != nil {
		t.Fatal("unreachable origin reported as final")
	}
}

func TestCacheKeyedByOrigin(t *testing.T) {
	c := Cache{Dir: t.TempDir()}
	if err := c.Save("site-1", "https://a.test", []byte{1, 2}); err != nil {
		t.Fatal(err)
	}
	if icon, hit := c.Load("site-1", "https://a.test"); !hit || !bytes.Equal(icon, []byte{1, 2}) {
		t.Fatal("cached icon lost")
	}
	if _, hit := c.Load("site-1", "https://b.test"); hit {
		t.Fatal("icon reused across origins")
	}
	if err := c.Save("site-2", "https://a.test", nil); err != nil {
		t.Fatal(err)
	}
	if icon, hit := c.Load("site-2", "https://a.test"); !hit || icon != nil {
		t.Fatal("missing icon not remembered")
	}
	if err := c.Invalidate("site-1"); err != nil {
		t.Fatal(err)
	}
	if _, hit := c.Load("site-1", "https://a.test"); hit {
		t.Fatal("invalidated icon kept")
	}
	if err := c.Save("../escape", "https://a.test", []byte{1}); err != nil {
		t.Fatal(err)
	}
	if _, hit := c.Load("../escape", "https://a.test"); hit {
		t.Fatal("unsafe id cached")
	}
}
