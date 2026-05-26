//go:build windows

package winlog

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/sys/windows/svc/eventlog"
)

const EventID = 1

type Handler struct {
	elog  *eventlog.Log
	level slog.Leveler
	attrs []slog.Attr
	group string
}

func NewHandler(source string, level slog.Leveler) (*Handler, error) {
	elog, err := eventlog.Open(source)
	if err != nil {
		return nil, fmt.Errorf("opening event log source %q: %w", source, err)
	}
	return &Handler{elog: elog, level: level}, nil
}

func (h *Handler) Close() error {
	return h.elog.Close()
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)

	if len(h.attrs) > 0 || r.NumAttrs() > 0 {
		b.WriteByte(' ')
	}

	for _, a := range h.attrs {
		fmt.Fprintf(&b, " %s=%v", h.prefixedKey(a.Key), a.Value)
	}
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", h.prefixedKey(a.Key), a.Value)
		return true
	})

	msg := b.String()

	switch {
	case r.Level >= slog.LevelError:
		return h.elog.Error(EventID, msg)
	case r.Level >= slog.LevelWarn:
		return h.elog.Warning(EventID, msg)
	default:
		return h.elog.Info(EventID, msg)
	}
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		elog:  h.elog,
		level: h.level,
		attrs: append(slicesClone(h.attrs), attrs...),
		group: h.group,
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}
	return &Handler{
		elog:  h.elog,
		level: h.level,
		attrs: slicesClone(h.attrs),
		group: newGroup,
	}
}

func (h *Handler) prefixedKey(key string) string {
	if h.group == "" {
		return key
	}
	return h.group + "." + key
}

func slicesClone(s []slog.Attr) []slog.Attr {
	if s == nil {
		return nil
	}
	c := make([]slog.Attr, len(s))
	copy(c, s)
	return c
}
