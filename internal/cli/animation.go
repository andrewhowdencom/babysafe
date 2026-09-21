package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	evdev "github.com/holoplot/go-evdev"
)

const (
	animationWidth  = 72
	animationHeight = 17
)

var (
	burstShapes = []string{"●", "★", "✦", "◆", "♥", "☀"}
	burstColors = []string{"\x1b[38;5;213m", "\x1b[38;5;220m", "\x1b[38;5;51m", "\x1b[38;5;119m", "\x1b[38;5;207m", "\x1b[38;5;45m"}
)

// keyAnimator owns the terminal while a capture session is running. Input
// callbacks only enqueue events; the render goroutine does all writing so a
// burst can animate without delaying the device-drain goroutines.
type keyAnimator struct {
	w      io.Writer
	events chan keyAnimation
	stop   chan struct{}
	done   chan struct{}
	once   sync.Once
	decode echoDecoder
}

type keyAnimation struct {
	label string
	seed  int
}

func newKeyAnimator(w io.Writer) *keyAnimator {
	return &keyAnimator{
		w:      w,
		events: make(chan keyAnimation, 16),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (a *keyAnimator) start() {
	go a.renderLoop()
}

func (a *keyAnimator) close() {
	a.once.Do(func() { close(a.stop) })
	<-a.done
}

// handle observes releases too, so the decoder's modifier state stays
// correct, but only presses and autorepeats create a new burst.
func (a *keyAnimator) handle(ev *evdev.InputEvent) {
	decoded := a.decode.feed(ev)
	if ev.Type != evdev.EV_KEY || ev.Value == 0 {
		return
	}

	label := strings.Trim(decoded, "[]")
	if label == "" {
		label = modifierLabel(ev.Code)
	}
	if label == "" {
		label = "★"
	}

	animation := keyAnimation{label: strings.ToUpper(label), seed: int(ev.Code)}
	select {
	case a.events <- animation:
	default:
		// Keep capture responsive if a key repeats faster than the terminal
		// can draw. The newest queued bursts will still be displayed.
	}
}

func (a *keyAnimator) renderLoop() {
	defer close(a.done)
	defer fmt.Fprint(a.w, "\x1b[0m\x1b[?25h\x1b[?1049l")

	fmt.Fprint(a.w, "\x1b[?1049h\x1b[?25l")
	ticker := time.NewTicker(70 * time.Millisecond)
	defer ticker.Stop()

	current := keyAnimation{}
	frame := -1
	sequence := 0
	for {
		select {
		case <-a.stop:
			return
		case current = <-a.events:
			sequence++
			current.seed += sequence * 17
			frame = 0
			fmt.Fprint(a.w, renderAnimationFrame(current, frame))
		case <-ticker.C:
			if frame >= 0 && frame < 10 {
				frame++
				fmt.Fprint(a.w, renderAnimationFrame(current, frame))
			} else if frame == -1 {
				fmt.Fprint(a.w, renderAnimationFrame(current, frame))
				frame = -2
			}
		}
	}
}

func renderAnimationFrame(animation keyAnimation, frame int) string {
	canvas := make([][]string, animationHeight)
	for row := range canvas {
		canvas[row] = make([]string, animationWidth)
		for col := range canvas[row] {
			canvas[row][col] = " "
		}
	}

	if frame < 0 {
		placeText(canvas, 6, centeredColumn("PRESS ANY KEY", animationWidth), "PRESS ANY KEY")
		placeText(canvas, 9, centeredColumn("and make some magic!", animationWidth), "and make some magic!")
	} else {
		centerRow, centerCol := 7, animationWidth/2
		radius := 2 + frame*2
		for i := 0; i < 28; i++ {
			rowOffset := ((i*11+animation.seed*3)%15 - 7) * radius / 12
			colOffset := ((i*19+animation.seed*5)%67 - 33) * radius / 12
			row, col := centerRow+rowOffset, centerCol+colOffset
			if row >= 0 && row < animationHeight && col >= 0 && col < animationWidth {
				canvas[row][col] = burstShapes[(i+animation.seed+frame)%len(burstShapes)]
			}
		}

		boxWidth := len(animation.label) + 6
		if boxWidth < 13 {
			boxWidth = 13
		}
		left := centeredColumn(strings.Repeat(" ", boxWidth), animationWidth)
		placeText(canvas, centerRow-1, left, "╭"+strings.Repeat("─", boxWidth-2)+"╮")
		placeText(canvas, centerRow, left, "│"+centeredText(animation.label, boxWidth-2)+"│")
		placeText(canvas, centerRow+1, left, "╰"+strings.Repeat("─", boxWidth-2)+"╯")
	}

	var b strings.Builder
	b.WriteString("\x1b[H\x1b[2J")
	colorOffset := animation.seed + frame
	for row, cells := range canvas {
		color := burstColors[positiveMod(row+colorOffset, len(burstColors))]
		b.WriteString(color)
		b.WriteString(strings.Join(cells, ""))
		b.WriteString("\x1b[0m\x1b[K\n")
	}
	b.WriteString("\x1b[38;5;245m")
	b.WriteString(centeredText("Ctrl+Alt+Esc to finish", animationWidth))
	b.WriteString("\x1b[0m\x1b[K")
	return b.String()
}

func placeText(canvas [][]string, row, col int, value string) {
	if row < 0 || row >= len(canvas) {
		return
	}
	for _, char := range value {
		if col >= 0 && col < len(canvas[row]) {
			canvas[row][col] = string(char)
		}
		col++
	}
}

func centeredColumn(value string, width int) int {
	return (width - len([]rune(value))) / 2
}

func centeredText(value string, width int) string {
	padding := width - len([]rune(value))
	if padding <= 0 {
		return value
	}
	return strings.Repeat(" ", padding/2) + value + strings.Repeat(" ", padding-padding/2)
}

func positiveMod(value, modulus int) int {
	value %= modulus
	if value < 0 {
		value += modulus
	}
	return value
}

func modifierLabel(code evdev.EvCode) string {
	switch code {
	case evdev.KEY_LEFTSHIFT, evdev.KEY_RIGHTSHIFT:
		return "SHIFT"
	case evdev.KEY_LEFTCTRL, evdev.KEY_RIGHTCTRL:
		return "CTRL"
	case evdev.KEY_LEFTALT, evdev.KEY_RIGHTALT:
		return "ALT"
	case evdev.KEY_LEFTMETA, evdev.KEY_RIGHTMETA:
		return "SUPER"
	default:
		return ""
	}
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
