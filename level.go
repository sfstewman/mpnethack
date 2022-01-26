package main

type Marker uint16
type MarkerArchetype uint8

func NewMarker(ma MarkerArchetype, minst uint16) Marker {
	return Marker((minst & 0x3fff) | ((uint16(ma) & 7) << 13))
}

func (m Marker) Type() MarkerArchetype {
	return MarkerArchetype(m >> 13)
}

const (
	MarkerSpace MarkerArchetype = iota
	MarkerBounds
	MarkerObject
	MarkerPortal
	MarkerSpawner
	MarkerDoor
)

const (
	MarkerVoid  Marker = Marker(uint16(MarkerSpace)<<13 | 0)
	MarkerEmpty Marker = Marker(uint16(MarkerSpace)<<13 | 1)

	MarkerBorder Marker = Marker(uint16(MarkerBounds)<<13 | 0)
	MarkerWall   Marker = Marker(uint16(MarkerBounds)<<13 | 1)
)

type Level struct {
	W, H  int
	Board []Marker

	Mobs []Mob
}

func (lvl *Level) Set(i, j int, m Marker) {
	ind := i*lvl.W + j
	lvl.Board[ind] = m
}

func (lvl *Level) Get(i, j int) Marker {
	ind := i*lvl.W + j
	return lvl.Board[ind]
}

func NewBoxLevel(w, h int) *Level {
	l := &Level{
		W: w,
		H: h,
	}

	l.Board = make([]Marker, w*h)
	for j := 0; j < w; j++ {
		l.Board[0*w+j] = MarkerBorder
		l.Board[(h-1)*w+j] = MarkerBorder
	}

	for i := 1; i < h-1; i++ {
		l.Board[i*w+0] = MarkerBorder
		l.Board[i*w+w-1] = MarkerBorder
	}

	for i := 1; i < h-1; i++ {
		for j := 1; j < w-1; j++ {
			l.Board[w*i+j] = MarkerEmpty
		}
	}

	return l
}

func SingleRoomLevel(width, height, roomWidth, roomHeight int) *Level {
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
		W:     levelWidth,
		H:     levelHeight,
		Board: board,
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
