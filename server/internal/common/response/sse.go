package response

import (
	"encoding/json"
	"io"

	"github.com/gin-gonic/gin"
)

func StreamHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
}

func WriteSSE(w io.Writer, event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	message := make([]byte, 0, len(event)+len(payload)+16)
	message = append(message, "event: "...)
	message = append(message, event...)
	message = append(message, "\ndata: "...)
	message = append(message, payload...)
	message = append(message, '\n', '\n')
	n, err := w.Write(message)
	if err != nil {
		return err
	}
	if n < len(message) {
		return io.ErrShortWrite
	}
	return nil
}
