package animation

import (
	"math"
	"math/rand"
	"time"
	"unicode/utf8"
)

const defaultEffectLimit = 64

var brightColors = [...]int{91, 92, 93, 94, 95, 96, 97}

type randomSource interface {
	Intn(int) int
}

type effect struct {
	label    string
	x, y     float64
	vx, vy   float64
	age      time.Duration
	lifetime time.Duration
	color    int
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
	label := token.Label
	if label == "" {
		return
	}
	if len(m.effects) == m.limit {
		copy(m.effects, m.effects[1:])
		m.effects = m.effects[:m.limit-1]
	}

	maxX := max(0, m.width-utf8.RuneCountInString(label))
	maxY := max(0, m.height-1)
	e := effect{
		label:    label,
		x:        float64(randomCoordinate(m.random, maxX)),
		y:        float64(randomCoordinate(m.random, maxY)),
		vx:       signedSpeed(m.random, 8, 13),
		vy:       signedSpeed(m.random, 3, 6),
		lifetime: time.Duration(1800+m.random.Intn(1201)) * time.Millisecond,
		color:    brightColors[m.random.Intn(len(brightColors))],
	}
	m.effects = append(m.effects, e)
}

func randomCoordinate(random randomSource, maximum int) int {
	if maximum <= 0 {
		return 0
	}
	return random.Intn(maximum + 1)
}

func signedSpeed(random randomSource, minimum, spread int) float64 {
	speed := float64(minimum + random.Intn(spread))
	if random.Intn(2) == 0 {
		return -speed
	}
	return speed
}

func (m *model) advance(elapsed time.Duration) {
	seconds := elapsed.Seconds()
	live := m.effects[:0]
	for i := range m.effects {
		e := m.effects[i]
		e.age += elapsed
		if e.age >= e.lifetime {
			continue
		}
		maxX := float64(max(0, m.width-utf8.RuneCountInString(e.label)))
		maxY := float64(max(0, m.height-1))
		e.x, e.vx = bounce(e.x+e.vx*seconds, e.vx, maxX)
		e.y, e.vy = bounce(e.y+e.vy*seconds, e.vy, maxY)
		live = append(live, e)
	}
	m.effects = live
}

func bounce(position, velocity, maximum float64) (float64, float64) {
	if maximum <= 0 {
		return 0, velocity
	}

	wrapped := math.Mod(position, 2*maximum)
	if wrapped < 0 {
		wrapped += 2 * maximum
	}
	direction := math.Copysign(1, velocity)
	if wrapped > maximum {
		return 2*maximum - wrapped, -math.Abs(velocity) * direction
	}
	return wrapped, math.Abs(velocity) * direction
}

func (m *model) clamp(e *effect) {
	maxX := float64(max(0, m.width-utf8.RuneCountInString(e.label)))
	maxY := float64(max(0, m.height-1))
	e.x = min(max(0, e.x), maxX)
	e.y = min(max(0, e.y), maxY)
}

func (m *model) frame() []sprite {
	frame := make([]sprite, len(m.effects))
	for i, e := range m.effects {
		frame[i] = sprite{label: e.label, x: int(e.x), y: int(e.y), color: e.color}
	}
	return frame
}
