// Package generation talks to provider image/video APIs and returns raw bytes.
package generation

import (
	"context"
	"fmt"
	"strings"
)

const (
	KindImage = "image"
	KindVideo = "video"

	DefaultImageModel = "grok-imagine-image-2.0"
	DefaultVideoModel = "grok-imagine-video-1.5"
	DefaultProviderID = "xai"
)

// ImageRequest is a provider-agnostic still generation request.
type ImageRequest struct {
	Prompt      string
	Model       string
	AspectRatio string
}

// VideoRequest is a provider-agnostic clip generation request.
type VideoRequest struct {
	Prompt      string
	Model       string
	AspectRatio string
	Duration    int
	Resolution  string
	Image       []byte
	ImageType   string
}

// Result is downloaded media ready to persist locally.
type Result struct {
	MediaType string
	Data      []byte
	Model     string
}

// ImageGenerator creates stills from a prompt.
type ImageGenerator interface {
	GenerateImage(ctx context.Context, req ImageRequest) (Result, error)
}

// VideoGenerator creates clips from a prompt or first frame.
type VideoGenerator interface {
	GenerateVideo(ctx context.Context, req VideoRequest) (Result, error)
}

// Binding is the Settings selection for one generation kind.
type Binding struct {
	ProviderID string
	Model      string
	Method     string
}

// Resolve picks an implemented adapter for the binding.
func Resolve(kind string, binding Binding) (any, error) {
	method := strings.ToLower(strings.TrimSpace(binding.Method))
	if method == "" {
		method = strings.ToLower(strings.TrimSpace(binding.ProviderID))
	}
	switch method {
	case "xai":
		client := NewXAIClient(nil)
		switch kind {
		case KindImage:
			return ImageGenerator(client), nil
		case KindVideo:
			return VideoGenerator(client), nil
		default:
			return nil, fmt.Errorf("unsupported generation kind %q", kind)
		}
	default:
		if strings.TrimSpace(binding.ProviderID) == "" && strings.TrimSpace(binding.Model) == "" {
			return nil, fmt.Errorf("no %s generation model is configured", kind)
		}
		return nil, fmt.Errorf("%s generation is not implemented for provider %q", kind, firstNonEmpty(binding.ProviderID, method))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
