package animation

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

const (
	enterSequence   = "\x1b[?1049h\x1b[?25l\x1b[2J\x1b[H"
	restoreSequence = "\x1b[0m\x1b[?25h\x1b[?1049l"
)

type terminal interface {
	enter() error
	size() (int, int, error)
	write(string) error
	restore() error
}

type ansiTerminal struct {
	writer io.Writer
	fd     int

	mu          sync.Mutex
	entered     bool
	restoreOnce sync.Once
	restoreErr  error
}

func newANSITerminal(writer io.Writer) (*ansiTerminal, error) {
	file, ok := writer.(*os.File)
	if !ok {
		return nil, errors.New("animation requires an interactive terminal")
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return nil, errors.New("animation requires a color-capable terminal (TERM is dumb)")
	}
	if !term.IsTerminal(int(file.Fd())) {
		return nil, errors.New("animation requires an interactive terminal")
	}
	return &ansiTerminal{writer: writer, fd: int(file.Fd())}, nil
}

func (t *ansiTerminal) enter() error {
	// Mark the terminal entered before writing so a partial write is still
	// followed by a best-effort restoration.
	t.mu.Lock()
	t.entered = true
	t.mu.Unlock()
	return t.write(enterSequence)
}

func (t *ansiTerminal) size() (int, int, error) {
	width, height, err := term.GetSize(t.fd)
	if err != nil {
		return 0, 0, fmt.Errorf("get terminal size: %w", err)
	}
	return width, height, nil
}

func (t *ansiTerminal) write(value string) error {
	_, err := io.WriteString(t.writer, value)
	if err != nil {
		return fmt.Errorf("write animation: %w", err)
	}
	return nil
}

func (t *ansiTerminal) restore() error {
	t.restoreOnce.Do(func() {
		t.mu.Lock()
		entered := t.entered
		t.mu.Unlock()
		if entered {
			t.restoreErr = t.write(restoreSequence)
		}
	})
	return t.restoreErr
}
