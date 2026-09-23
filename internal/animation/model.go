package animation

import (
	"math"
	"math/rand"
	"time"
	"unicode/utf8"
)

const (
	defaultEffectLimit = 64
	launchDuration     = 550 * time.Millisecond
	burstDuration      = 1500 * time.Millisecond
	particleCount      = 16
)

var brightColors = [...]int{91, 92, 93, 94, 95, 96, 97}

type randomSource interface {
	Intn(int) int
}

type particle struct {
	vx, vy float64
	color  int
}

type effect struct {
	label     string
	x, y      float64 // Center of the burst.
	launchY   float64
	age       time.Duration
	color     int
	particles [particleCount]particle
}

type sprite struct {
	label string
	x, y  int
	color int
}

type model struct {
	width, height int
	limit         int
	random        randomSource
	effects       []effect
}

func newModel(random randomSource) *model {
	if random == nil {
		random = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // Visual variation only.
	}
	return &model{width: 1, height: 1, limit: defaultEffectLimit, random: random}
}

func (m *model) resize(width, height int) {
	m.width = max(1, width)
	m.height = max(1, height)
	for i := range m.effects {
		m.clamp(&m.effects[i])
	}
}

func (m *model) spawn(token Token) {
	if token.Label == "" {
		return
	}
	if len(m.effects) == m.limit {
		copy(m.effects, m.effects[1:])
		m.effects = m.effects[:m.limit-1]
	}

	maxX := max(0, m.width-utf8.RuneCountInString(token.Label))
	// Keep the burst away from the edges when the terminal has room.
	marginX := min(m.width/5, maxX/2)
	minY := m.height / 5
	maxY := max(minY, m.height/2)
	e := effect{
		label:   token.Label,
		x:       float64(marginX + randomCoordinate(m.random, maxX-2*marginX)),
		y:       float64(minY + randomCoordinate(m.random, maxY-minY)),
		launchY: float64(m.height - 1),
		color:   brightColors[m.random.Intn(len(brightColors))],
	}
	for i := range e.particles {
		angle := 2 * math.Pi * float64(i) / particleCount
		speed := float64(12 + m.random.Intn(9))
		e.particles[i] = particle{
			vx:    math.Cos(angle) * speed * 1.6,
			vy:    math.Sin(angle) * speed * 0.65,
			color: brightColors[m.random.Intn(len(brightColors))],
		}
	}
	m.effects = append(m.effects, e)
}

func randomCoordinate(random randomSource, maximum int) int {
	if maximum <= 0 {
		return 0
	}
	return random.Intn(maximum + 1)
}

func (m *model) advance(elapsed time.Duration) {
	if elapsed < 0 {
		return
	}
	live := m.effects[:0]
	for _, e := range m.effects {
		e.age += elapsed
		if e.age < launchDuration+burstDuration {
			live = append(live, e)
		}
	}
	m.effects = live
}

func (m *model) clamp(e *effect) {
	maxX := float64(max(0, m.width-utf8.RuneCountInString(e.label)))
	maxY := float64(m.height - 1)
	e.x = min(max(0, e.x), maxX)
	e.y = min(max(0, e.y), maxY)
	e.launchY = min(max(0, e.launchY), maxY)
}

func (m *model) frame() []sprite {
	frame := make([]sprite, 0, len(m.effects)*(particleCount+4))
	for _, e := range m.effects {
		x := int(math.Round(e.x))
		if e.age < launchDuration {
			progress := float64(e.age) / float64(launchDuration)
			y := int(math.Round(e.launchY + (e.y-e.launchY)*progress))
			for offset, glyph := range []string{"✦", "+", "·"} {
				if y+offset+1 < m.height {
					frame = append(frame, sprite{label: glyph, x: x, y: y + offset + 1, color: e.color})
				}
			}
			frame = append(frame, sprite{label: e.label, x: x, y: y, color: e.color})
			continue
		}

		seconds := (e.age - launchDuration).Seconds()
		for _, p := range e.particles {
			px := int(math.Round(e.x + p.vx*seconds))
			py := int(math.Round(e.y + p.vy*seconds + 5*seconds*seconds))
			if px < 0 || px >= m.width || py < 0 || py >= m.height {
				continue
			}
			glyph := "✦"
			if seconds > 0.45 {
				glyph = "*"
			}
			if seconds > 0.95 {
				glyph = "·"
			}
			frame = append(frame, sprite{label: glyph, x: px, y: py, color: p.color})
		}
		// Keep the key visible at the heart of the firework while sparks expand.
		frame = append(frame, sprite{label: e.label, x: x, y: int(math.Round(e.y)), color: e.color})
	}
	return frame
}
