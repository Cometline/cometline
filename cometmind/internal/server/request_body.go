package server

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// stagedBody serves a stand-in JSON object to the generated strict binder,
// then the original bytes once the adapter calls useOriginalBody. Handlers
// keep their existing bind and error text.
type stagedBody struct {
	original []byte
	cur      *bytes.Reader
}

func (b *stagedBody) Read(p []byte) (int, error) {
	if b.cur == nil {
		return 0, io.EOF
	}
	return b.cur.Read(p)
}

func (b *stagedBody) Close() error { return nil }

func stageRequestBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		original := readOriginalBody(c.Request)
		if c.Request != nil {
			c.Request.Body = &stagedBody{
				original: original,
				cur:      bytes.NewReader([]byte("{}")),
			}
		}
		c.Next()
	}
}

func readOriginalBody(r *http.Request) []byte {
	if r == nil || r.Body == nil {
		return nil
	}
	original, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return nil
	}
	return original
}

func useOriginalBody(c *gin.Context) {
	if c == nil || c.Request == nil {
		return
	}
	staged, ok := c.Request.Body.(*stagedBody)
	if !ok {
		return
	}
	staged.cur = bytes.NewReader(staged.original)
}
