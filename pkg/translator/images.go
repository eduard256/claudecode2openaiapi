package translator

import (
	"github.com/eduard256/claudecode2openaiapi/pkg/imagecache"
)

// imageBlock builds an Anthropic-native image content block from any
// OpenAI image_url value (data: or http(s)://).
func imageBlock(url string) (map[string]any, error) {
	img, err := imagecache.Fetch(url)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type": "image",
		"source": map[string]any{
			"type":       "base64",
			"media_type": img.MediaType,
			"data":       img.Base64,
		},
	}, nil
}
