package imagecache

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	maxBytes = 5 * 1024 * 1024 // Anthropic per-image limit
	timeout  = 30 * time.Second
)

// Image holds bytes ready for Anthropic-style content block.
type Image struct {
	MediaType string // image/png, image/jpeg, image/webp, image/gif
	Base64    string
}

// Fetch parses an OpenAI-style image_url. Accepts:
//   - data:<mime>;base64,<data>
//   - http(s):// URL (downloaded, capped at 5MB)
func Fetch(url string) (*Image, error) {
	if strings.HasPrefix(url, "data:") {
		return parseDataURL(url)
	}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return download(url)
	}
	return nil, errors.New("imagecache: unsupported image url scheme")
}

func parseDataURL(s string) (*Image, error) {
	// data:image/png;base64,iVBOR...
	rest := strings.TrimPrefix(s, "data:")
	semi := strings.Index(rest, ";")
	comma := strings.Index(rest, ",")
	if semi < 0 || comma < 0 || comma < semi {
		return nil, errors.New("imagecache: malformed data url")
	}
	media := rest[:semi]
	enc := rest[semi+1 : comma]
	body := rest[comma+1:]

	if enc != "base64" {
		return nil, errors.New("imagecache: only base64-encoded data urls supported")
	}
	if !validMedia(media) {
		return nil, errors.New("imagecache: unsupported media type: " + media)
	}
	if base64.StdEncoding.DecodedLen(len(body)) > maxBytes {
		return nil, errors.New("imagecache: image exceeds 5MB")
	}
	return &Image{MediaType: media, Base64: body}, nil
}

func download(url string) (*Image, error) {
	cli := &http.Client{Timeout: timeout}
	resp, err := cli.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("imagecache: http " + resp.Status)
	}
	media := resp.Header.Get("Content-Type")
	if i := strings.Index(media, ";"); i > 0 {
		media = strings.TrimSpace(media[:i])
	}
	if !validMedia(media) {
		return nil, errors.New("imagecache: unsupported media type: " + media)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBytes {
		return nil, errors.New("imagecache: image exceeds 5MB")
	}
	return &Image{MediaType: media, Base64: base64.StdEncoding.EncodeToString(body)}, nil
}

func validMedia(m string) bool {
	switch m {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return true
	}
	return false
}
