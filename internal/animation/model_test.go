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

	if len(model.effects) != defaultEffectLimit {
		t.Fatalf("model has %d effects, want %d", len(model.effects), defaultEffectLimit)
	}
	if model.effects[0].label != "1" || model.effects[len(model.effects)-1].label != "64" {
		t.Fatalf("unexpected effect labels after eviction")
	}
}

func TestModelLaunchesBurstsAndExpires(t *testing.T) {
	model := newModel(zeroRandom{})
	model.resize(80, 24)
	model.spawn(Token{Label: "A"})
	start := model.frame()
	if start[len(start)-1].label != "A" || start[len(start)-1].y != 23 {
		t.Fatalf("launch did not start at bottom: %+v", start)
	}
	model.advance(launchDuration / 2)
	mid := model.frame()
	if mid[len(mid)-1].y >= 23 || mid[len(mid)-1].y <= int(model.effects[0].y) {
		t.Fatalf("letter did not rise: %+v", mid[len(mid)-1])
	}
	model.advance(launchDuration / 2)
	model.advance(300 * time.Millisecond)
	burst := model.frame()
	if len(burst) <= 1 || burst[len(burst)-1].label != "A" {
		t.Fatalf("burst has no sparks around letter: %+v", burst)
	}
	model.advance(burstDuration)
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
	model.effects[0].launchY = 19
	model.resize(3, 2)

	effect := model.effects[0]
	if effect.x != 0 || effect.y > 1 || effect.launchY > 1 {
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

func TestModelFireworkStaysWithinTinyTerminal(t *testing.T) {
	model := newModel(zeroRandom{})
	model.resize(2, 1)
	model.spawn(Token{Label: "SPACE"})
	for _, age := range []time.Duration{0, launchDuration, launchDuration + 500*time.Millisecond} {
		model.effects[0].age = age
		for _, sprite := range model.frame() {
			if sprite.x < 0 || sprite.x >= model.width || sprite.y < 0 || sprite.y >= model.height {
				t.Fatalf("sprite outside tiny terminal: %+v", sprite)
			}
		}
	}
}
