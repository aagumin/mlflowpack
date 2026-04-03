// Package logger provides a compact slog handler for buildpack output.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// CompactHandler outputs slog records as:
//
//	LEVEL message key=value ...
//
// For example:
//
//	INFO Downloading metadata source=s3://bucket/path
//	WARN failed to close file path=/tmp/foo error=details
type CompactHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
}

// NewCompactHandler creates a CompactHandler that writes to w at the given level.
func NewCompactHandler(w io.Writer, level slog.Level) *CompactHandler {
	return &CompactHandler{w: w, level: level}
}

// SetupDefault configures the global slog default to use CompactHandler.
func SetupDefault(w io.Writer, level slog.Level) {
	slog.SetDefault(slog.New(NewCompactHandler(w, level)))
}

func (h *CompactHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *CompactHandler) Handle(_ context.Context, r slog.Record) error {
	var b []byte
	b = append(b, r.Level.String()...)
	b = append(b, ' ')
	b = append(b, r.Message...)
	for _, a := range h.attrs {
		b = appendAttr(b, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		b = appendAttr(b, a)
		return true
	})
	b = append(b, '\n')
	_, err := h.w.Write(b)
	return err
}

func (h *CompactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	combined = append(combined, h.attrs...)
	combined = append(combined, attrs...)
	return &CompactHandler{w: h.w, level: h.level, attrs: combined}
}

func (h *CompactHandler) WithGroup(string) slog.Handler { return h }

func appendAttr(b []byte, a slog.Attr) []byte {
	b = append(b, ' ')
	b = append(b, a.Key...)
	b = append(b, '=')
	return appendValue(b, a.Value)
}

func appendValue(b []byte, v slog.Value) []byte {
	if v.Kind() != slog.KindString {
		return append(b, fmt.Sprintf("%v", v.Any())...)
	}
	s := v.String()
	if !needsQuote(s) {
		return append(b, s...)
	}
	b = append(b, '"')
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			b = append(b, '\\')
		}
		b = append(b, s[i])
	}
	b = append(b, '"')
	return b
}

func needsQuote(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '"', '=', '\n', '\t':
			return true
		}
	}
	return false
}
