package animation

import (
	"fmt"
	"testing"
	"time"
)

type zeroRandom struct{}

func (zeroRandom) Intn(int) int { return 0 }

func TestModelEvictsOldestAtLimit(t *testing.T) {
	model := newModel(zeroRandom{})
	model.resize(80, 20)
	for i := 0; i <= defaultEffectLimit; i++ {
		model.spawn(Token{Label: fmt.Sprint(i)})
	}

	frame := model.frame()
	if len(frame) != defaultEffectLimit {
		t.Fatalf("frame has %d effects, want %d", len(frame), defaultEffectLimit)
	}
	if frame[0].label != "1" || frame[len(frame)-1].label != "64" {
		t.Fatalf("labels range from %q to %q, want 1 to 64", frame[0].label, frame[len(frame)-1].label)
	}
}

func TestModelBouncesAndExpires(t *testing.T) {
	model := newModel(zeroRandom{})
	model.resize(10, 5)
	model.effects = []effect{{label: "A", x: 8.5, y: 4, vx: 10, vy: 10, lifetime: time.Second}}
	model.advance(100 * time.Millisecond)

	effect := model.effects[0]
	if effect.x < 0 || effect.x > 9 || effect.y < 0 || effect.y > 4 {
		t.Fatalf("effect escaped bounds: (%v, %v)", effect.x, effect.y)
	}
	if effect.vx >= 0 || effect.vy >= 0 {
		t.Fatalf("effect did not bounce: velocity (%v, %v)", effect.vx, effect.vy)
	}

	model.advance(time.Second)
	if len(model.effects) != 0 {
		t.Fatalf("expired effect remains: %d", len(model.effects))
	}
}

func TestModelResizeClampsLongLabels(t *testing.T) {
	model := newModel(zeroRandom{})
	model.resize(40, 20)
	model.spawn(Token{Label: "A VERY LONG KEY LABEL"})
	model.effects[0].x = 30
	model.effects[0].y = 19
	model.resize(3, 2)

	effect := model.effects[0]
	if effect.x != 0 || effect.y > 1 {
		t.Fatalf("resized effect at (%v, %v), want within 3x2", effect.x, effect.y)
	}
}

func TestModelFramePreservesLayerOrder(t *testing.T) {
	model := newModel(zeroRandom{})
	model.spawn(Token{Label: "first"})
	model.spawn(Token{Label: "second"})
	frame := model.frame()
	if frame[0].label != "first" || frame[1].label != "second" {
		t.Fatalf("frame order = %q, %q", frame[0].label, frame[1].label)
	}
}
