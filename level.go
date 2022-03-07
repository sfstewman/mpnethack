package mpnethack

import "fmt"

const (
	LevelWidth  = 128
	LevelHeight = 128
)

type Marker uint32
type MarkerArchetype uint8

func NewMarker(ma MarkerArchetype, minst uint32) Marker {
	return Marker((minst & 0xffffff) | (uint32(ma) << 24))
}

func (m Marker) Type() MarkerArchetype {
	return MarkerArchetype(m >> 24)
}

func (m Marker) Arg() uint32 {
	return uint32(m) & 0xffffff
}

const (
	MarkerSpace MarkerArchetype = iota
	MarkerBounds
	MarkerObject
	MarkerPortal
	MarkerSpawner
	MarkerDoor
	MarkerMob
	MarkerPlayer
)

const (
	//lint:ignore SA4016 ignore irrelevant static analysis comment in const block
	MarkerVoid  Marker = Marker(uint32(MarkerSpace)<<24 | 0)
	MarkerEmpty Marker = Marker(uint32(MarkerSpace)<<24 | 1)

	//lint:ignore SA4016 ignore irrelevant static analysis comment in const block
	MarkerBorder Marker = Marker(uint32(MarkerBounds)<<24 | 0)
	MarkerWall   Marker = Marker(uint32(MarkerBounds)<<24 | 1)

	// placeholder
	MarkerCactus Marker = Marker(uint32(MarkerObject)<<24 | 23)
)

func (m Marker) Name() string {
	switch m {
	case MarkerVoid:
		return "the void"
	case MarkerEmpty:
		return "empty space"
	case MarkerBorder:
		return "a border of the world"
	case MarkerWall:
		return "a wall"
	case MarkerCactus:
		return "a cactus"
	}

	var arch string
	switch t := m.Type(); t {
	case MarkerSpace:
		arch = "space"
	case MarkerBounds:
		arch = "bounds"
	case MarkerObject:
		arch = "object"
	case MarkerPortal:
		arch = "portal"
	case MarkerSpawner:
		arch = "portal"
	case MarkerDoor:
		arch = "door"
	case MarkerMob:
		arch = "mob"
	case MarkerPlayer:
		arch = "player"
	default:
		arch = fmt.Sprintf("%v", t)
	}

	return fmt.Sprintf("%s_%d", arch, m.Arg())
}

func MobMarker(mobType MobType) Marker {
	return NewMarker(MarkerMob, uint32(mobType))
}

type Board struct {
	Elements []Marker
	W, H     int
}

type Level struct {
	Board
	Mobs []Mob

	PlayerI0, PlayerJ0 int
}

func (b *Board) Set(i, j int, m Marker) {
	ind := i*b.W + j
	b.Elements[ind] = m
}

func (b *Board) Get(i, j int) Marker {
	ind := i*b.W + j
	return b.Elements[ind]
}

func NewBoxLevel(w, h int) *Level {
	l := &Level{
		Board: Board{
			Elements: make([]Marker, w*h),
			W:        w,
			H:        h,
		},
	}

	for j := 0; j < w; j++ {
		l.Set(0, j, MarkerBorder)
		l.Set(h-1, j, MarkerBorder)
	}

	for i := 1; i < h-1; i++ {
		l.Set(i, 0, MarkerBorder)
		l.Set(i, w-1, MarkerBorder)
	}

	for i := 1; i < h-1; i++ {
		for j := 1; j < w-1; j++ {
			l.Set(i, j, MarkerEmpty)
		}
	}

	return l
}

func SingleRoomLevel(height, width, roomHeight, roomWidth int) *Level {
	levelWidth := width
	levelHeight := height

	if width < roomWidth+4 {
		levelWidth = roomWidth + 4
	}

	if height < roomHeight+4 {
		levelHeight = roomHeight + 4
	}

	roomI0 := (height - roomHeight) / 2
	roomJ0 := (width - roomWidth) / 2

	roomI1 := roomI0 + roomHeight
	roomJ1 := roomJ0 + roomWidth

	npts := levelWidth * levelHeight
	board := make([]Marker, npts)

	lvl := &Level{
		Board: Board{
			Elements: board,
			W:        width,
			H:        height,
		},
	}

	for i := 0; i < levelHeight; i++ {
		for j := 0; j < levelWidth; j++ {
			if i >= roomI0 && i < roomI1 && j >= roomJ0 && j < roomJ1 {
				lvl.Set(i, j, MarkerEmpty)
			} else if (i == roomI0-1 || i == roomI1) && j >= roomJ0-1 && j <= roomJ1 {
				lvl.Set(i, j, MarkerWall)
			} else if i >= roomI0-1 && i <= roomI1 && (j == roomJ0-1 || j == roomJ1) {
				lvl.Set(i, j, MarkerWall)
			} else {
				lvl.Set(i, j, MarkerVoid)
			}
		}
	}

	return lvl
}

func (l *Level) AddMob(mobType MobType, stats UnitStats, i, j int, direc Direction, state MobState, args ...int16) error {
	info, err := LookupMobInfo(mobType)
	if err != nil {
		return fmt.Errorf("error looking up mob info: %w", err)
	}

	var moveRate int16
	if info != nil {
		moveRate = info.MoveRate
	}

	m := Mob{
		I:          i,
		J:          j,
		Stats:      stats,
		Type:       mobType,
		MoveTick:   moveRate,
		Direc:      direc,
		Weapon:     info.DefaultWeapon,
		State:      state,
		Aggression: info.DefaultAggression,
	}

	l.Mobs = append(l.Mobs, m)

	return nil
}
