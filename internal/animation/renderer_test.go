package animation

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	evdev "github.com/holoplot/go-evdev"
)

type fakeTerminal struct {
	mu          sync.Mutex
	width       int
	height      int
	writes      []string
	wrote       chan struct{}
	entered     chan struct{}
	enterOnce   sync.Once
	writeErr    error
	restoreCall int
}

func (t *fakeTerminal) enter() error {
	t.enterOnce.Do(func() {
		if t.entered != nil {
			close(t.entered)
		}
	})
	return nil
}
func (t *fakeTerminal) size() (int, int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.width, t.height, nil
}
func (t *fakeTerminal) write(value string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.writeErr != nil {
		return t.writeErr
	}
	t.writes = append(t.writes, value)
	if t.wrote != nil {
		select {
		case t.wrote <- struct{}{}:
		default:
		}
	}
	return nil
}
func (t *fakeTerminal) restore() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.restoreCall++
	return nil
}

func TestRendererSubmitIsBoundedAndFavorsNewest(t *testing.T) {
	renderer := newRenderer(&fakeTerminal{}, realClock{}, zeroRandom{})
	for i := 0; i < eventCapacity; i++ {
		renderer.Submit(eventPtr(keyEvent(evdev.KEY_A, 1)))
	}
	renderer.Submit(eventPtr(keyEvent(evdev.KEY_Z, 1)))

	if got := len(renderer.events); got != eventCapacity {
		t.Fatalf("queued events = %d, want %d", got, eventCapacity)
	}
	var last Token
	for len(renderer.events) > 0 {
		last = <-renderer.events
	}
	if last.Label != "z" {
		t.Fatalf("last token = %q, want z", last.Label)
	}
}

func TestRendererRunRendersFooterAndRestores(t *testing.T) {
	terminal := &fakeTerminal{width: 40, height: 10, entered: make(chan struct{})}
	renderer := newRenderer(terminal, realClock{}, zeroRandom{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- renderer.Run(ctx) }()
	<-terminal.entered
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	if len(terminal.writes) == 0 || !strings.Contains(terminal.writes[0], footerText) {
		t.Fatalf("initial frame omitted footer: %q", terminal.writes)
	}
	if terminal.restoreCall != 1 {
		t.Fatalf("restore calls = %d, want 1", terminal.restoreCall)
	}
}

func TestRendererReturnsWriterErrorAndRestores(t *testing.T) {
	writeErr := errors.New("broken writer")
	terminal := &fakeTerminal{width: 40, height: 10, writeErr: writeErr}
	renderer := newRenderer(terminal, realClock{}, zeroRandom{})
	err := renderer.Run(context.Background())
	if !errors.Is(err, writeErr) {
		t.Fatalf("Run() error = %v, want %v", err, writeErr)
	}
	if terminal.restoreCall != 1 {
		t.Fatalf("restore calls = %d, want 1", terminal.restoreCall)
	}
	if err := renderer.Close(); err != nil {
		t.Fatal(err)
	}
	if terminal.restoreCall != 1 {
		t.Fatalf("restore calls after Close = %d, want 1", terminal.restoreCall)
	}
}

func TestRenderTruncatesForNarrowTerminal(t *testing.T) {
	terminal := &fakeTerminal{width: 3, height: 2}
	renderer := newRenderer(terminal, realClock{}, zeroRandom{})
	renderer.model.resize(3, 1)
	renderer.model.spawn(Token{Label: "LONG LABEL"})
	if err := renderer.render(); err != nil {
		t.Fatal(err)
	}
	terminal.mu.Lock()
	defer terminal.mu.Unlock()
	if !strings.Contains(terminal.writes[0], "LON") {
		t.Fatalf("frame = %q", terminal.writes[0])
	}
}

func TestRendererIgnoresAutorepeat(t *testing.T) {
	renderer := newRenderer(&fakeTerminal{}, realClock{}, zeroRandom{})
	renderer.Submit(&evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 2})
	if len(renderer.events) != 0 {
		t.Fatal("autorepeat queued an animation")
	}
}

func TestRendererTicksAndAdaptsToResize(t *testing.T) {
	start := time.Unix(100, 0)
	clock := &fakeClock{now: start, ticks: make(chan time.Time, 1)}
	terminal := &fakeTerminal{
		width: 20, height: 8,
		entered: make(chan struct{}),
		wrote:   make(chan struct{}, 2),
	}
	renderer := newRenderer(terminal, clock, zeroRandom{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- renderer.Run(ctx) }()
	<-terminal.entered
	<-terminal.wrote // Initial frame.

	terminal.mu.Lock()
	terminal.width, terminal.height = 12, 5
	terminal.mu.Unlock()
	clock.ticks <- start.Add(frameInterval)
	<-terminal.wrote
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if renderer.model.width != 12 || renderer.model.height != 4 {
		t.Fatalf("model size = %dx%d, want 12x4", renderer.model.width, renderer.model.height)
	}
}

func TestTruncateUsesRunes(t *testing.T) {
	if got := truncate("★abc", 2); got != "★a" {
		t.Fatalf("truncate() = %q", got)
	}
}

type fakeClock struct {
	now   time.Time
	ticks chan time.Time
}

func (c *fakeClock) Now() time.Time                 { return c.now }
func (c *fakeClock) NewTicker(time.Duration) ticker { return fakeTicker{ticks: c.ticks} }

type fakeTicker struct{ ticks <-chan time.Time }

func (t fakeTicker) Chan() <-chan time.Time { return t.ticks }
func (fakeTicker) Stop()                    {}
