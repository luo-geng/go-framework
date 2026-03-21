package slog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"time"

	"gorm.io/gorm/logger"
)

const (
	gray = "\033[90m"
	// green  = logger.Green
	yellow = logger.Yellow
	red    = logger.Red
	cyan   = logger.Cyan
	reset  = logger.Reset
)

type ColorHandler struct {
	w    io.Writer
	opts *slog.HandlerOptions
}

func NewColorHandler(w io.Writer, opts *slog.HandlerOptions) slog.Handler {
	return &ColorHandler{w: w, opts: opts}
}

func (h *ColorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.opts == nil || h.opts.Level == nil {
		return true
	}
	return level >= h.opts.Level.Level()
}

func (h *ColorHandler) Handle(ctx context.Context, r slog.Record) error {

	color := gray

	switch r.Level {
	case slog.LevelInfo:
		color = cyan
	case slog.LevelWarn:
		color = yellow
	case slog.LevelError:
		color = red

	}

	t := time.Now().Format(time.TimeOnly)
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	fmt.Fprintf(
		h.w,
		"%s[%s] %-5s %s:%d %s",
		color,
		t,
		r.Level.String(),
		f.File, f.Line,
		r.Message,
	)

	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.w, " %s=%v", a.Key, a.Value)
		return true
	})

	fmt.Fprintf(h.w, "%s\n", reset)

	return nil
}

func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *ColorHandler) WithGroup(name string) slog.Handler       { return h }
