package mpnethack

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
	"unicode"

	"github.com/sfstewman/mpnethack/chat"
)

var (
	UnknownCommandError  error = errors.New("Unkown command")
	OnCooldownError      error = errors.New("Action still on cooldown")
	InvalidCooldownError error = errors.New("Action has no valid cooldown")
)

const (
	GameRefreshInterval time.Duration = 100 * time.Millisecond
)

type ActionType uint16

const (
	Nothing ActionType = iota
	Move
	Attack
	Defend

	MaxActionType int = iota
)

func (act ActionType) String() string {
	switch act {
	case Nothing:
		return "ACT_NOP"
	case Move:
		return "ACT_MOV"
	case Attack:
		return "ACT_ATT"
	case Defend:
		return "ACT_DEF"
	default:
		return fmt.Sprintf("ACT_UNK_%d", int(act))
	}
}

type Direction int16

const (
	NoDirection Direction = iota
	Left
	Right
	Up
	Down
)

func (direc Direction) Name() string {
	switch direc {
	case NoDirection:
		return "none"
	case Left:
		return "left"
	case Right:
		return "right"
	case Up:
		return "up"
	case Down:
		return "down"
	}

	return fmt.Sprintf("Direction[%d]", direc)
}

var UserActionCooldownTicks = [MaxActionType]uint64{
	Nothing: 0,
	Move:    1,
	Attack:  5,
	Defend:  150,
}

type Cooldowns []uint32

var zeroCooldowns = [MaxActionType]uint32{}

func calcCooldowns(now uint64, last []uint64, cd Cooldowns) Cooldowns {
	if cd == nil {
		cd = make(Cooldowns, len(last))
	}

	for i, when := range last {
		if i >= MaxActionType || when == 0 {
			cd[i] = 0
			continue
		}

		nextTime := when + UserActionCooldownTicks[i]
		if now >= nextTime {
			cd[i] = 0
			continue
		}

		cd[i] = uint32(nextTime - now)
	}

	return cd
}

type actionKey struct {
	sess Session
	act  ActionType
}

type action struct {
	Type ActionType
	Arg  int16
}

const (
	DefaultPlayerRow  = 3
	DefaultPlayerCol0 = LevelWidth / 4
)

type Session interface {
	IsAdministrator() bool
	HasGame() bool

	Game() *Game
	Player() *Player
	UserName() string

	GetLog() *chat.Log
	ConsoleInput(string)
	Message(chat.MsgLevel, string) error

	Join(g *Game) error
	Update() error
	Quit()
}

type Namer interface {
	Name() string
}

type UnitStats struct {
	ArmorClass int
	THAC0      int
	HP         int
}

func (s *UnitStats) ToHit(other *UnitStats) int {
	return s.THAC0 + other.ArmorClass
}

// FIXME: not the best interface: 1) not a verb; 2) GetX
type Unit interface {
	Namer
	GetMarker() rune
	GetPos() (i int, j int, h int, w int)

	GetStats() *UnitStats

	IsAlive() bool
}

type MobType uint32

type MobInfo struct {
	Type MobType

	Name   string
	Marker rune
	W, H   int

	MoveTicks uint16
}

const (
	MobLemming MobType = iota
	MobViciousLemming
)

var mobTypes = []MobInfo{
	MobInfo{Type: MobLemming, Name: "Lemming", Marker: 'L', MoveTicks: 10},
	MobInfo{Type: MobViciousLemming, Name: "Vicious lemming", Marker: 'V', MoveTicks: 5},
}

func AddMobType(info MobInfo) MobType {
	mt := MobType(len(mobTypes))

	info.Type = mt
	mobTypes = append(mobTypes, info)

	return mt
}

func LookupMobInfo(mt MobType) *MobInfo {
	ind := int(mt)
	if ind >= len(mobTypes) {
		return nil
	}

	return &mobTypes[ind]
}

type Mob struct {
	I, J int

	Stats UnitStats
	Type  MobType

	MoveTick uint16
	StunTick uint16

	ActionTick [4]uint16

	Direc  Direction
	States [5]int16
}

func (m *Mob) TakeDamage(dmg int) {
	hp := m.Stats.HP - dmg
	if hp < 0 {
		hp = 0
		m.Direc = NoDirection
	}

	m.Stats.HP = hp
}

func (m *Mob) IsAlive() bool {
	return m.Stats.HP > 0
}

func (m *Mob) GetStats() *UnitStats {
	return &m.Stats
}

func (m *Mob) Name() string {
	info := LookupMobInfo(m.Type)

	var n string
	if info != nil {
		n = info.Name
	} else {
		n = "Mob_Unknown"
	}

	if m.IsAlive() {
		return n
	} else {
		return "dead " + n
	}
}

func (m *Mob) GetMarker() rune {
	info := LookupMobInfo(m.Type)
	if info == nil {
		return 0
	}

	return info.Marker
}

func (m *Mob) GetPos() (i int, j int, h int, w int) {
	info := LookupMobInfo(m.Type)

	i = m.I
	j = m.J

	if info != nil {
		h = info.H
		w = info.W
	} else {
		h = 1
		w = 1
	}

	return
}

type Player struct {
	S      Session
	I, J   int
	Marker rune
	Facing Direction

	Inventory []Item
	Weapon    Item

	Cooldowns []uint64

	Stats UnitStats

	SwingRate   uint16
	SwingTick   uint16
	SwingState  uint16
	SwingFacing Direction
}

func (p *Player) GetStats() *UnitStats {
	return &p.Stats
}

func (p *Player) Name() string {
	return p.S.UserName()
}

func (p *Player) GetMarker() rune {
	return p.Marker
}

func (p *Player) GetPos() (i int, j int, h int, w int) {
	i = p.I
	j = p.J
	h = 1
	w = 1
	return
}

type EffectType int

const (
	EffectSwordSwing EffectType = iota + 1
)

type Effect struct {
	I, J      int
	Rune      rune
	Collision Namer
}

type Game struct {
	mu   sync.RWMutex
	pump *time.Ticker
	Ctx  context.Context

	Dice Dice

	Active   []Session
	GameLog  *chat.Log
	FrameNum uint64

	actions map[Session]action

	Level   *Level
	Players map[string]*Player
	Markers map[rune]*Player
	// Rendered Board
	Mobs           []Mob
	EffectsOverlay []Effect

	Cancel context.CancelFunc
}

func (g *Game) Lock() {
	g.mu.Lock()
}

func (g *Game) Unlock() {
	g.mu.Unlock()
}

func (g *Game) RLock() {
	g.mu.RLock()
}

func (g *Game) RUnlock() {
	g.mu.RUnlock()
}

func (g *Game) hasCollision(newI, newJ int) Namer {
	lvl := g.Level

	if what := lvl.Get(newI, newJ); what != MarkerEmpty {
		return what
	}

	// TODO: better collision detect for players/mobs
	for _, pl := range g.Markers {
		if newI == pl.I && newJ == pl.J {
			return pl
		}
	}

	mobs := g.Mobs
	for i := range mobs {
		m := &mobs[i]

		if newI == m.I && newJ == m.J {
			return m
		}
	}

	return nil
}

const GameLogNumLines = 100

func NewGame(l *Level) (*Game, error) {
	ctx, cancelFunc := context.WithCancel(context.Background())

	dice, err := NewDice()
	if err != nil {
		return nil, err
	}

	g := &Game{
		pump: time.NewTicker(GameRefreshInterval),
		Ctx:  ctx,

		Dice: dice,

		GameLog: chat.NewLog(GameLogNumLines),

		// cooldowns: make(map[*Session][]uint64),
		actions: make(map[Session]action),

		Level:   l, // NewBoxLevel(LevelWidth, LevelHeight),
		Players: make(map[string]*Player),
		Markers: make(map[rune]*Player),

		Cancel: cancelFunc,
	}

	g.Mobs = make([]Mob, len(l.Mobs))
	copy(g.Mobs, l.Mobs)

	go g.Loop()
	return g, nil
}

func (g *Game) PickMarker(user string) (rune, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.pickMarker(user)
}

var ErrNoFreeMarkers = errors.New("no free markers for player")

const PlayerTokens = "@!#%*+123456789\u2460\u2461\u2462\u2463\u2464\u2465\u2466\u2467\u2468\u2469\u246a\u246b\u246c\u256d\u256e\u246f\u2470\u2471\u2472\u2473"

func (g *Game) pickMarker(user string) (rune, error) {
	tbl := g.Markers

	if user != "" {
		for i, initial := range user {
			if i >= 2 {
				break
			}

			if !unicode.IsLetter(initial) {
				continue
			}

			if tbl[initial] == nil {
				return initial, nil
			}

			if up := unicode.ToUpper(initial); up != initial && tbl[up] == nil {
				return up, nil
			}

			if lo := unicode.ToLower(initial); lo != initial && tbl[lo] == nil {
				return lo, nil
			}
		}
	}

	for _, ch := range PlayerTokens {
		if tbl[ch] == nil {
			return ch, nil
		}
	}

	return 0, ErrNoFreeMarkers
}

func (g *Game) PlayerJoin(sess Session) (*Player, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// i0 := DefaultPlayerRow
	// j0 := DefaultPlayerCol0

	name := sess.UserName()

	marker, err := g.pickMarker(name)
	if err != nil {
		return nil, err
	}

	pl := &Player{
		I:         g.Level.PlayerI0, // i0,
		J:         g.Level.PlayerJ0, // j0,
		Marker:    marker,
		S:         sess,
		Facing:    Up,
		Weapon:    RustySword,
		Inventory: []Item{},
		Stats: UnitStats{
			ArmorClass: 10,
			THAC0:      0,
			HP:         16,
		},
	}

	g.Players[name] = pl
	g.Markers[marker] = pl

	g.Active = append(g.Active, sess)
	g.messagef(chat.Info, "%s (%c) joined the game!", name, marker)

	return pl, nil
}

func (g *Game) PlayerLeave(sess Session) {
	g.mu.Lock()
	defer g.mu.Unlock()

	pl := sess.Player()
	name := sess.UserName()
	if pl.S != sess {
		return
	}

	// delete(g.Players, sess.User)
	delete(g.Players, name)

	for i, activeSess := range g.Active {
		if activeSess == sess {
			g.Active = append(g.Active[:i], g.Active[i+1:]...)
			break
		}
	}

	g.Messagef(chat.Info, "%s left the game!", name)
}

func (g *Game) Shutdown() {
	g.pump.Stop()
}

func (g *Game) GetCooldowns(s Session, cds Cooldowns) Cooldowns {
	pl := s.Player()
	last := pl.Cooldowns

	now := g.FrameNum
	if len(last) == 0 {
		if cds == nil {
			return make(Cooldowns, len(zeroCooldowns))
		}

		copy(cds, zeroCooldowns[:])
		return cds
	}

	return calcCooldowns(now, last, cds)
}

func (g *Game) UserAction(s Session, act ActionType, arg int16) error {
	if int(act) >= len(UserActionCooldownTicks) {
		return InvalidCooldownError
	}

	// k := actionKey{s, act}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.FrameNum

	pl := s.Player()
	actionCDs := pl.Cooldowns

	if len(actionCDs) == 0 {
		actionCDs = make([]uint64, len(UserActionCooldownTicks))
		pl.Cooldowns = actionCDs
	}

	last := actionCDs[act]
	if last > 0 && now-last < UserActionCooldownTicks[act] {
		return OnCooldownError
	}

	if act != Nothing {
		actionCDs[act] = now
		log.Printf("action %v, cooldowns %v\n", act, actionCDs)
	}

	g.handleAction(s, action{act, arg})

	return nil
}

func (g *Game) Move(s Session, direc Direction) error {
	return g.UserAction(s, Move, int16(direc))
}

func (g *Game) handleAction(s Session, act action) {
	pl := s.Player()
	if pl == nil {
		return
	}

	user := s.UserName()
	lvl := g.Level

	switch act.Type {
	case Move:
		di := 0
		dj := 0
		dir := "<unknown>"

		direc := Direction(act.Arg)
		switch direc {
		case Left:
			dir = "left"
			dj = -1
		case Right:
			dir = "right"
			dj = 1
		case Up:
			dir = "up"
			di = -1
		case Down:
			dir = "down"
			di = 1
		}

		newI := pl.I + di
		newJ := pl.J + dj

		if newI < 0 {
			newI = 0
		}

		if newI >= lvl.H {
			newI = lvl.H - 1
		}

		if newJ < 0 {
			newJ = 0
		}

		if newJ >= lvl.W {
			newJ = lvl.W - 1
		}

		if what := g.hasCollision(newI, newJ); what != nil {
			g.messagef(chat.Game, "%s tried to move %s but hit a %s", user, dir, what.Name())
		} else {
			pl.I = newI
			pl.J = newJ
		}

		pl.Facing = direc

	case Attack:
		switch facing := pl.Facing; facing {
		case Up, Down, Left, Right:
			// g.messagef(MsgGame, "%s swings weapon %s", user, facing.Name())

			pl.SwingRate = 3
			pl.SwingTick = pl.SwingRate
			pl.SwingState = 3
			pl.SwingFacing = facing
		}
	case Defend:
		g.messagef(chat.Game, "%s is defending", user)
	}
}

func (g *Game) loopInner() {
	g.mu.Lock()
	defer g.mu.Unlock()

	lvl := g.Level

	/*** Game loop ***/

	// user actions
	for _, s := range g.Active {
		act := g.actions[s]
		if act.Type == Nothing {
			continue
		}

		g.actions[s] = action{}
		g.handleAction(s, act)
	}

	// update rooms

	// update mobs
	for i := range g.Mobs {
		mob := &g.Mobs[i]

		if mob.Direc == 0 {
			continue
		}

		mobInfo := LookupMobInfo(mob.Type)

		if mob.MoveTick--; mob.MoveTick <= 0 {
			mob.MoveTick = mobInfo.MoveTicks

			var di, dj int
			var mirror Direction = NoDirection
			switch mob.Direc {
			case Up:
				di, dj = -1, 0
				mirror = Down
			case Down:
				di, dj = 1, 0
				mirror = Up
			case Left:
				di, dj = 0, -1
				mirror = Right
			case Right:
				di, dj = 0, 1
				mirror = Left
			}

			i1 := mob.I + di
			j1 := mob.J + dj

			if i1 < 0 {
				i1 = 0
			}

			if i1 >= lvl.H {
				i1 = lvl.H - 1
			}

			if j1 < 0 {
				j1 = 0
			}

			if j1 >= lvl.W {
				j1 = lvl.W - 1
			}

			if what := g.hasCollision(i1, j1); what != nil {
				i1 = mob.I
				j1 = mob.J

				mob.Direc = mirror
			}

			mob.I = i1
			mob.J = j1
		}
	}

	g.EffectsOverlay = g.EffectsOverlay[:0]

	// player actions
	for _, pl := range g.Players {
		if pl.SwingState > 0 && pl.SwingFacing != NoDirection {
			pl.SwingTick--
			if pl.SwingTick == 0 {
				pl.SwingState--
			}

			var ui, uj, vi, vj int
			switch pl.SwingFacing {
			case Up:
				ui, uj = -1, 0
				vi, vj = 0, 1
			case Down:
				ui, uj = 1, 0
				vi, vj = 0, -1
			case Left:
				ui, uj = 0, -1
				vi, vj = -1, 0
			case Right:
				ui, uj = 0, 1
				vi, vj = 1, 0
			}

			var swDI, swDJ int
			switch pl.SwingState {
			case 3:
				swDI, swDJ = ui+vi, uj+vj
			case 2:
				swDI, swDJ = ui, uj
			case 1:
				swDI, swDJ = ui-vi, uj-vj

			case 0:
				pl.SwingRate = 0
				pl.SwingTick = 0
				pl.SwingFacing = NoDirection
			}

			var swordRune rune
			if swDJ == 0 {
				swordRune = '|'
			} else if swDI == 0 {
				swordRune = '-'
			} else if swDI == -swDJ {
				swordRune = '/'
			} else if swDI == swDJ {
				swordRune = '\\'
			} else {
				log.Printf("invalid sword state?")
				continue
			}

			swI := pl.I + swDI
			swJ := pl.J + swDJ

			if pl.SwingState > 0 {
				coll := g.hasCollision(swI, swJ)
				if coll != nil && pl.SwingTick == 0 {
					weaponItem := pl.Weapon
					if weaponItem == nil {
						weaponItem = BareHands
					}

					shortName := weaponItem.ShortName()

					switch victim := coll.(type) {
					case *Mob:
						if victim.IsAlive() {
							stats := victim.GetStats()
							toHit := pl.GetStats().ToHit(stats)

							var dmg int
							switch w := weaponItem.(type) {
							case *MeleeWeapon:
								dmg = w.Damage(victim, g.Dice)
							case Item:
								dmg = 1
							}

							if g.Dice.RollD20() <= toHit {
								g.messagef(chat.Game, "%s slashes %s with a %s for %d damage", pl.Name(), coll.Name(), shortName, dmg)

								victim.TakeDamage(dmg)

								if !victim.IsAlive() {
									g.messagef(chat.Game, "%s killed %s", pl.Name(), coll.Name())
								}
							} else {
								g.messagef(chat.Game, "%s swings wildy at %s with a %s but misses", pl.Name(), coll.Name(), shortName)
							}
						} else {
							g.messagef(chat.Game, "%s swings the %s futility at the %s.",
								pl.Name(), shortName, coll.Name())
						}

					case Marker:
						if w, ok := weaponItem.(*MeleeWeapon); ok && len(w.HitObjectDescription) > 0 {
							g.messagef(chat.Game, "%s swings the %s futility at the %s.  %s",
								pl.Name(), shortName, coll.Name(), w.HitObjectDescription)
						} else {
							g.messagef(chat.Game, "%s swings the %s futility at the %s.",
								pl.Name(), shortName, coll.Name())
						}

					case *Player:
						g.messagef(chat.Game, "%s thwacks %s with the %s.  %s looks very miffed.",
							pl.Name(), coll.Name(), shortName, coll.Name())
					}
				}

				g.EffectsOverlay = append(g.EffectsOverlay, Effect{
					I:         swI,
					J:         swJ,
					Rune:      swordRune,
					Collision: coll,
				})

				if pl.SwingTick == 0 {
					pl.SwingTick = pl.SwingRate
				}
			}
		}
	}

	// update area effects

	// update frame counter
	g.FrameNum++

	// debugging frameno printout
	if g.FrameNum%64 == 0 {
		// log.Printf("frame %d", g.FrameNum)
		// g.messagef(MsgInfo, "frame %d", g.FrameNum)
	}
}

func (g *Game) sendUpdate() {
	active := (func() []Session {
		g.mu.RLock()
		defer g.mu.RUnlock()

		active := make([]Session, len(g.Active))
		copy(active, g.Active)

		return active
	})()

	for _, s := range active {
		s.Update()
	}
}

func (g *Game) Loop() {
	// game loop calculation

	doneCh := g.Ctx.Done()
	updCh := g.pump.C

	g.Message(chat.Info, "Welcome!")

GameLoop:
	for {
		g.loopInner()

		select {
		case <-doneCh:
			log.Printf("game %v stopping", g)
			break GameLoop

		case <-updCh:
		}

		g.sendUpdate()
	}
}

func (g *Game) Command(sess Session, txt string) error {
	switch {
	case txt == "/quit":
		sess.Quit()
		// sess.App.Stop()
		return nil

	default:
		return UnknownCommandError
	}
}

func (g *Game) Input(l chat.MsgLevel, txt string) error {
	return g.Message(l, txt)
}

func (g *Game) Messagef(l chat.MsgLevel, fmtStr string, args ...interface{}) error {
	s := fmt.Sprintf(fmtStr, args...)
	return g.Message(l, s)
}

func (g *Game) Message(l chat.MsgLevel, s string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.message(l, s)
}

func (g *Game) messagef(l chat.MsgLevel, fmtStr string, args ...interface{}) error {
	s := fmt.Sprintf(fmtStr, args...)
	return g.message(l, s)
}

// Assumes lock is held (either read or write)
func (g *Game) message(lvl chat.MsgLevel, s string) error {
	// XXX: global game log
	var errs []error
	for _, sess := range g.Active {
		err := sess.Message(lvl, s)
		if err != nil {
			errs = append(errs, err)
		}
	}

	g.GameLog.LogLine(lvl, s)

	if errs == nil {
		return nil
	}

	// XXX: fix this!
	return errors.New("multiple errors")
}
