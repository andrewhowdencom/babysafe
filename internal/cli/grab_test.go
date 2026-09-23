package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	evdev "github.com/holoplot/go-evdev"

	"github.com/andrewhowdencom/babysafe/pkg/capture"
)

func TestGrabPreflightsTerminalBeforeCreatingSession(t *testing.T) {
	preflightErr := errors.New("not a terminal")
	var sessionCreated atomic.Bool
	command := newGrabCmdWithDependencies(grabDependencies{
		newRenderer: func(io.Writer) (grabRenderer, error) { return nil, preflightErr },
		newSession: func(context.Context, capture.Options) (capture.Session, error) {
			sessionCreated.Store(true)
			return nil, errors.New("unexpected session creation")
		},
	})
	command.SetOut(&bytes.Buffer{})

	err := command.Execute()
	if !errors.Is(err, preflightErr) {
		t.Fatalf("Execute() error = %v, want %v", err, preflightErr)
	}
	if sessionCreated.Load() {
		t.Fatal("session was created before terminal preflight succeeded")
	}
}

func TestGrabHelpDescribesAnimationAndHasNoEcho(t *testing.T) {
	command := newGrabCmd()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"--help"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "colorful firework animation") {
		t.Fatalf("help omitted animation description:\n%s", output.String())
	}
	if strings.Contains(output.String(), "--echo") {
		t.Fatalf("help retained --echo:\n%s", output.String())
	}
}

func TestRunGrabSessionCompletionCancelsRenderer(t *testing.T) {
	rendererStopped := make(chan struct{})
	renderer := &fakeGrabRenderer{run: func(ctx context.Context) error {
		<-ctx.Done()
		close(rendererStopped)
		return nil
	}}
	session := &fakeCaptureSession{run: func(context.Context) error { return nil }}

	if err := runGrab(context.Background(), renderer, session); err != nil {
		t.Fatal(err)
	}
	<-rendererStopped
	if renderer.closeCalls.Load() != 1 || session.closeCalls.Load() != 1 {
		t.Fatalf("cleanup calls renderer=%d session=%d", renderer.closeCalls.Load(), session.closeCalls.Load())
	}
}

func TestRunGrabRendererFailureCancelsCapture(t *testing.T) {
	renderErr := errors.New("render failed")
	sessionStopped := make(chan struct{})
	renderer := &fakeGrabRenderer{run: func(context.Context) error { return renderErr }}
	session := &fakeCaptureSession{run: func(ctx context.Context) error {
		<-ctx.Done()
		close(sessionStopped)
		return nil
	}}

	err := runGrab(context.Background(), renderer, session)
	if !errors.Is(err, renderErr) || !strings.Contains(err.Error(), "animation") {
		t.Fatalf("runGrab() error = %v", err)
	}
	<-sessionStopped
	if renderer.closeCalls.Load() != 1 || session.closeCalls.Load() != 1 {
		t.Fatalf("cleanup calls renderer=%d session=%d", renderer.closeCalls.Load(), session.closeCalls.Load())
	}
}

type fakeGrabRenderer struct {
	run        func(context.Context) error
	closeErr   error
	closeCalls atomic.Int32
}

func (*fakeGrabRenderer) Submit(*evdev.InputEvent) {}
func (r *fakeGrabRenderer) Run(ctx context.Context) error {
	return r.run(ctx)
}
func (r *fakeGrabRenderer) Close() error {
	r.closeCalls.Add(1)
	return r.closeErr
}

type fakeCaptureSession struct {
	run        func(context.Context) error
	closeErr   error
	closeCalls atomic.Int32
}

func (*fakeCaptureSession) Devices() []string { return []string{"/dev/input/event1"} }
func (s *fakeCaptureSession) Run(ctx context.Context) error {
	return s.run(ctx)
}
func (s *fakeCaptureSession) Close(context.Context) error {
	s.closeCalls.Add(1)
	return s.closeErr
}
