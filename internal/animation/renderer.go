package animation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	evdev "github.com/holoplot/go-evdev"
)

const (
	frameInterval = time.Second / 30
	eventCapacity = 64
	footerText    = "Ctrl+Alt+Esc to release"
	maxWidth      = 500
	maxHeight     = 200
)

type ticker interface {
	Chan() <-chan time.Time
	Stop()
}

type clock interface {
	Now() time.Time
	NewTicker(time.Duration) ticker
}

type realClock struct{}

type realTicker struct{ *time.Ticker }

func (realClock) Now() time.Time { return time.Now() }
func (realClock) NewTicker(duration time.Duration) ticker {
	return realTicker{Ticker: time.NewTicker(duration)}
}
func (t realTicker) Chan() <-chan time.Time { return t.C }

// Renderer owns terminal output and receives captured events through Submit.
type Renderer struct {
	terminal     terminal
	normalizer   *normalizer
	events       chan Token
	clock        clock
	model        *model
	screenHeight int

	closeOnce sync.Once
	closeErr  error
}

// NewRenderer validates output without changing terminal state. The caller can
// therefore reject unsupported output before grabbing any devices.
func NewRenderer(output io.Writer) (*Renderer, error) {
	terminal, err := newANSITerminal(output)
	if err != nil {
		return nil, err
	}
	return newRenderer(terminal, realClock{}, rand.New(rand.NewSource(time.Now().UnixNano()))), nil //nolint:gosec // Visual variation only.
}

func newRenderer(terminal terminal, clock clock, random randomSource) *Renderer {
	return &Renderer{
		terminal:   terminal,
		normalizer: newNormalizer(),
		events:     make(chan Token, eventCapacity),
		clock:      clock,
		model:      newModel(random),
	}
}

// Submit performs bounded work and never waits for the render goroutine. When
// overloaded it evicts one stale token in favor of the newest press.
func (r *Renderer) Submit(event *evdev.InputEvent) {
	token, ok := r.normalizer.normalize(event)
	if !ok {
		return
	}
	select {
	case r.events <- token:
		return
	default:
	}
	select {
	case <-r.events:
	default:
	}
	select {
	case r.events <- token:
	default:
	}
}

// Run renders until cancellation or a terminal error. Run must only be called
// once; one goroutine owns all output for the duration of the call.
func (r *Renderer) Run(ctx context.Context) (err error) {
	if err := r.terminal.enter(); err != nil {
		return errors.Join(err, r.Close())
	}
	defer func() {
		err = errors.Join(err, r.Close())
	}()

	if err := r.resize(); err != nil {
		return err
	}
	if err := r.render(); err != nil {
		return err
	}

	ticker := r.clock.NewTicker(frameInterval)
	defer ticker.Stop()
	last := r.clock.Now()
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return ctx.Err()
		case token := <-r.events:
			r.model.spawn(token)
		case now := <-ticker.Chan():
			r.model.advance(now.Sub(last))
			last = now
			if err := r.resize(); err != nil {
				return err
			}
			if err := r.render(); err != nil {
				return err
			}
		}
	}
}

func (r *Renderer) resize() error {
	width, height, err := r.terminal.size()
	if err != nil {
		return err
	}
	width = min(max(1, width), maxWidth)
	height = min(max(1, height), maxHeight)
	r.screenHeight = height
	r.model.resize(width, max(1, height-1))
	return nil
}

func (r *Renderer) render() error {
	var frame strings.Builder
	frame.WriteString("\x1b[0m\x1b[2J\x1b[H")
	for _, sprite := range r.model.frame() {
		label := truncate(sprite.label, r.model.width)
		if label == "" {
			continue
		}
		x := min(sprite.x, max(0, r.model.width-utf8.RuneCountInString(label)))
		fmt.Fprintf(&frame, "\x1b[%d;%dH\x1b[%dm%s", sprite.y+1, x+1, sprite.color, label)
	}
	footer := truncate(footerText, r.model.width)
	footerX := max(0, (r.model.width-utf8.RuneCountInString(footer))/2)
	fmt.Fprintf(&frame, "\x1b[%d;%dH\x1b[90m%s\x1b[0m", max(1, r.screenHeight), footerX+1, footer)
	return r.terminal.write(frame.String())
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > width {
		runes = runes[:width]
	}
	return string(runes)
}

// Close restores terminal state. It is safe to call repeatedly.
func (r *Renderer) Close() error {
	r.closeOnce.Do(func() {
		r.closeErr = r.terminal.restore()
	})
	return r.closeErr
}
