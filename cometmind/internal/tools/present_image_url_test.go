package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/session"
)

type imageRoundTripper func(*http.Request) (*http.Response, error)

func (f imageRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type urlMediaAppenderStub struct {
	last []session.ContentBlock
}

func (m *urlMediaAppenderStub) AppendAssistantMedia(
	_ context.Context,
	_ string,
	images []session.ContentBlock,
) (session.Message, error) {
	m.last = images
	return session.Message{ID: "msg1"}, nil
}

func TestPresentImageURLRegistersRemoteImage(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46}
	stub := &urlMediaAppenderStub{}
	tool := PresentImageURL{
		Media: stub,
		Client: &http.Client{Transport: imageRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://1.1.1.1/image.jpg" {
				t.Fatalf("URL = %q", req.URL)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
				Body:       io.NopCloser(strings.NewReader(string(jpeg))),
			}, nil
		})},
	}
	ctx := WithToolSession(context.Background(), "sess-url")

	res, err := tool.Execute(ctx, json.RawMessage(`{"url":"https://1.1.1.1/image.jpg","alt":"remote"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not ok: %s", res.Output)
	}
	if len(stub.last) != 1 || stub.last[0].MediaType != "image/jpeg" {
		t.Fatalf("AppendAssistantMedia blocks = %#v", stub.last)
	}
	if !strings.Contains(res.Output, "downloaded image") {
		t.Fatalf("output = %q", res.Output)
	}
}

func TestPresentImageURLRejectsNonImageResponse(t *testing.T) {
	tool := PresentImageURL{
		Media: &urlMediaAppenderStub{},
		Client: &http.Client{Transport: imageRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Body:       io.NopCloser(strings.NewReader("not an image")),
			}, nil
		})},
	}
	ctx := WithToolSession(context.Background(), "sess-url")

	res, err := tool.Execute(ctx, json.RawMessage(`{"url":"https://1.1.1.1/not-image"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "content type") {
		t.Fatalf("result = %#v", res)
	}
}

func TestPresentImageURLRejectsLocalAddress(t *testing.T) {
	tool := PresentImageURL{Media: &urlMediaAppenderStub{}}
	ctx := WithToolSession(context.Background(), "sess-url")

	res, err := tool.Execute(ctx, json.RawMessage(`{"url":"http://localhost:7700/image.png"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "local address") {
		t.Fatalf("result = %#v", res)
	}
}
