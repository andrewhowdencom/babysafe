package cli

import (
	"sync"
	"testing"

	evdev "github.com/holoplot/go-evdev"
)

// TestEchoDecoder exercises the printable-key decoding, the special-
// key naming, and the shift bookkeeping that an --echo printer
// relies on. Run as a single test rather than t.Parallel() because
// the decoder's internal mutex is only meaningful when concurrent
// callers exist — see TestEchoDecoderConcurrent below for that.
func TestEchoDecoder(t *testing.T) {
	tests := []struct {
		name   string
		events []*evdev.InputEvent
		want   string
	}{
		{
			name: "lowercase letter",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 1},
			},
			want: "a",
		},
		{
			name: "shift then letter gives uppercase",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTSHIFT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_K, Value: 1},
			},
			want: "K",
		},
		{
			name: "release of shift then letter is lowercase again",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTSHIFT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTSHIFT, Value: 0},
				{Type: evdev.EV_KEY, Code: evdev.KEY_K, Value: 1},
			},
			want: "k",
		},
		{
			name: "right shift works the same",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_RIGHTSHIFT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_1, Value: 1},
			},
			want: "!",
		},
		{
			name: "digits and shift row",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_2, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTSHIFT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_7, Value: 1},
			},
			want: "2&",
		},
		{
			name: "punctuation unshifted",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_SEMICOLON, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_COMMA, Value: 1},
			},
			want: ";,",
		},
		{
			name: "punctuation shifted",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFTSHIFT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_SEMICOLON, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_COMMA, Value: 1},
			},
			want: ":<",
		},
		{
			name: "space is printable",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_SPACE, Value: 1},
			},
			want: " ",
		},
		{
			name: "enter prints its name",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_ENTER, Value: 1},
			},
			want: "[ENTER]",
		},
		{
			name: "tab prints its name",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_TAB, Value: 1},
			},
			want: "[TAB]",
		},
		{
			name: "function keys",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_F1, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_F12, Value: 1},
			},
			want: "[F1][F12]",
		},
		{
			name: "arrow keys",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_UP, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_DOWN, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_LEFT, Value: 1},
				{Type: evdev.EV_KEY, Code: evdev.KEY_RIGHT, Value: 1},
			},
			want: "[UP][DOWN][LEFT][RIGHT]",
		},
		{
			name: "key release produces no output but updates state",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 0},
				{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 1},
			},
			want: "a",
		},
		{
			name: "non-key events produce no output",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_REL, Code: 0, Value: 5},
				{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 1},
			},
			want: "a",
		},
		{
			name: "unknown key produces no output",
			events: []*evdev.InputEvent{
				{Type: evdev.EV_KEY, Code: evdev.EvCode(9999), Value: 1},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &echoDecoder{}
			var got string
			for _, ev := range tt.events {
				got += d.feed(ev)
			}
			if got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestEchoDecoderConcurrent exercises the decoder's mutex by having
// multiple goroutines call feed() in parallel. It is also a smoke
// test for the race detector when run with -race.
func TestEchoDecoderConcurrent(t *testing.T) {
	d := &echoDecoder{}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				d.feed(&evdev.InputEvent{
					Type:  evdev.EV_KEY,
					Code:  evdev.KEY_LEFTSHIFT,
					Value: 1,
				})
				d.feed(&evdev.InputEvent{
					Type:  evdev.EV_KEY,
					Code:  evdev.KEY_A,
					Value: 1,
				})
				d.feed(&evdev.InputEvent{
					Type:  evdev.EV_KEY,
					Code:  evdev.KEY_LEFTSHIFT,
					Value: 0,
				})
			}
		}()
	}
	wg.Wait()

	// After the goroutines have all set and cleared shift, the
	// decoder should be in a quiescent state: shift is false, and a
	// fresh 'a' press decodes to lowercase.
	if got := d.feed(&evdev.InputEvent{
		Type:  evdev.EV_KEY,
		Code:  evdev.KEY_A,
		Value: 1,
	}); got != "a" {
		t.Errorf("post-concurrent output = %q, want %q", got, "a")
	}
}
