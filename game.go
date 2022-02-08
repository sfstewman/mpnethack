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

func (direc Direction) Vectors() (ui, uj, vi, vj int) {
	switch direc {
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

	return
}

func (direc Direction) Mirror() Direction {
	switch direc {
	case Up:
		return Down
	case Down:
		return Up
	case Left:
		return Right
	case Right:
		return Left
	default:
		return NoDirection
	}
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

type Action struct {
	Player *Player
	Type   ActionType
	Arg    int16
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
	ArmorClass         int
	THAC0              int
	HP                 int
	MaxHP              int
	HealthRecoveryRate int16
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

	TakeDamage(dmg int, u Unit)
	IsAlive() bool
}

type MobType uint32

type MobInfo struct {
	Type MobType

	Name   string
	Marker rune
	W, H   int

	MoveRate       int16
	ChaseRate      int16
	SeekTargetRate int16

	DefaultWeapon     Item
	DefaultAggression Aggression

	ViewDistance int
	FieldOfView  int

	InitialState    MobState
	InitialStateArg int
}

const (
	MobLemming MobType = iota
	MobViciousLemming
)

var mobTypes = []MobInfo{
	MobInfo{
		Type:              MobLemming,
		Name:              "Lemming",
		Marker:            'L',
		W:                 1,
		H:                 1,
		MoveRate:          10,
		ChaseRate:         8,
		SeekTargetRate:    300,
		DefaultWeapon:     LemmingClaws,
		DefaultAggression: AggressionDefends,
		ViewDistance:      3,
		FieldOfView:       3,
		InitialState:      MobPatrol,
	},
	MobInfo{
		Type:              MobViciousLemming,
		Name:              "Vicious lemming",
		Marker:            'V',
		W:                 1,
		H:                 1,
		MoveRate:          5,
		ChaseRate:         3,
		SeekTargetRate:    200,
		DefaultWeapon:     LemmingClaws,
		DefaultAggression: AggressionAttacks,
		ViewDistance:      3,
		FieldOfView:       3,
		InitialState:      MobPatrol,
	},
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

type MobEvent int

const (
	// No events have happened to this mob
	MobEventNone MobEvent = iota

	// Mob was attacked, but no damage was done
	MobEventAttacked

	// Mob was recently hit
	MobEventHit

	// Mob's health is below 25%
	MobEventBadlyHurt

	// Possible future events:
	// MobEventStunned
	// MobEventFriendDied
	// MobEventHurt
)

type MobState int

const (
	MobStill MobState = iota
	MobSentry
	MobWander
	MobPatrol
	MobSeekTarget
	MobAttack
	MobFlee
)

func (st MobState) String() string {
	switch st {
	case MobStill:
		return "still"
	case MobSentry:
		return "sentry"
	case MobWander:
		return "wander"
	case MobPatrol:
		return "patrol"
	case MobSeekTarget:
		return "seek_target"
	case MobAttack:
		return "attack"
	case MobFlee:
		return "flee"
	default:
		return fmt.Sprintf("state_%d", int(st))
	}
}

// Mob aggression levels
//
// Loosely indicates what the mob will do when it encounters another character
// or mob
//
// Passive           - mob is passive and will try to run away if attacked
// Defends           - mob will not attack unless attacked
// Attacks           - mob will attack players when they are found
// Attacks mobs      - mob will attack other mobs that are not of its species/tribe/etc.
// Attacks only mobs - mob will attack other mobs that are not of its species/tribe/etc.
//                     but not players, unless attacked
// Blind rage        - mob will attack anything
//
// Aggression is something that can be changed/escalated by the mob's state machine
//   - Attack can become 'Attacks mobs' if attacked by another mob
//
//   - Vicious lemmings start with Aggression='Attacks', but after taking enough damage
//     this will escalate into Aggression='Blind rage'.
//
//   - Lemmings start out as
//
type Aggression int

const (
	AggressionPassive Aggression = iota
	// consider: AggressionStoic: defends if attacked and damaged (or damaged enough)
	AggressionDefends
	AggressionAttacks
	AggressionAttacksMobs
	AggressionBlindRage
)

func (agg Aggression) String() string {

	switch agg {
	case AggressionPassive:
		return "passive"
	case AggressionDefends:
		return "defends"
	case AggressionAttacks:
		return "attacks"
	case AggressionAttacksMobs:
		return "attacks_mobs"
	case AggressionBlindRage:
		return "blind_rage"
	default:
		return fmt.Sprintf("aggression_%d", int(agg))
	}
}

type Mob struct {
	I, J int

	Stats UnitStats
	Type  MobType

	MoveTick   int16
	StunTick   int16
	SeekTick   int16
	AttackTick int16

	ActionTick [4]uint16

	Direc Direction

	Weapon Item

	Event      MobEvent
	EventCause Unit

	State    MobState
	StateArg int // helper state for state machine controlling behavior

	Aggression  Aggression
	Target      Unit
	LastTargetI int
	LastTargetJ int
}

var _ Unit = &Mob{}

func (m *Mob) TakeDamage(dmg int, u Unit) {
	hp := m.Stats.HP - dmg
	if hp < 0 {
		hp = 0
		m.Direc = NoDirection
	}

	m.Stats.HP = hp

	if hp < m.Stats.MaxHP/4 {
		m.Event = MobEventBadlyHurt
		m.EventCause = u
	} else {
		m.Event = MobEventHit
		m.EventCause = u
	}
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

	BusyTick   int16
	HealthTick int16

	SwingRate   int16
	SwingTick   int16
	SwingState  int16
	SwingFacing Direction
}

var _ Unit = &Player{}

func (p *Player) GetStats() *UnitStats {
	return &p.Stats
}

func (p *Player) IsAlive() bool {
	return p.Stats.HP > 0
}

func (p *Player) TakeDamage(dmg int, u Unit) {
	hp := p.Stats.HP - dmg
	if hp <= 0 {
		hp = 0
		p.BusyTick = 0
		p.HealthTick = 0
		p.SwingTick = 0
		p.SwingState = 0
		p.SwingFacing = NoDirection
	}

	p.Stats.HP = hp
	if hp < p.Stats.MaxHP {
		p.HealthTick = p.Stats.HealthRecoveryRate
	}
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

	pendingActions []Action

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

func RollDirection(d Dice) Direction {
	switch d.Roll1dN(4) {
	case 1:
		return Up
	case 2:
		return Left
	case 3:
		return Down
	case 4:
		return Right

	default:
		return NoDirection
	}
}

func (g *Game) hasCollision(newI, newJ int) (Namer, bool) {
	lvl := g.Level

	if newI < 0 || newJ < 0 || newI >= lvl.H || newJ >= lvl.W {
		return nil, true
	}

	if what := lvl.Get(newI, newJ); what != MarkerEmpty {
		return what, true
	}

	// TODO: better collision detect for players/mobs
	for _, pl := range g.Markers {
		if newI == pl.I && newJ == pl.J {
			return pl, true
		}
	}

	mobs := g.Mobs
	for i := range mobs {
		m := &mobs[i]

		if newI == m.I && newJ == m.J {
			return m, true
		}
	}

	return nil, false
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
		// actions: make(map[Session]action),

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
			ArmorClass:         10,
			THAC0:              0,
			HP:                 16,
			MaxHP:              16,
			HealthRecoveryRate: 50,
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

func (g *Game) UserAction(s Session, actType ActionType, arg int16) error {
	if int(actType) >= len(UserActionCooldownTicks) {
		return InvalidCooldownError
	}

	// k := actionKey{s, act}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.FrameNum

	pl := s.Player()
	actionCDs := pl.Cooldowns

	if pl.BusyTick > 0 || pl.SwingState > 0 || !pl.IsAlive() {
		return OnCooldownError
	}

	if len(actionCDs) == 0 {
		actionCDs = make([]uint64, len(UserActionCooldownTicks))
		pl.Cooldowns = actionCDs
	}

	last := actionCDs[actType]
	if last > 0 && now-last < UserActionCooldownTicks[actType] {
		return OnCooldownError
	}

	if actType != Nothing {
		actionCDs[actType] = now
		log.Printf("action %v, cooldowns %v\n", actType, actionCDs)
	}

	g.handleAction(Action{pl, actType, arg})
	// g.pendingActions = append(g.pendingActions, Action{pl, actType, arg})

	return nil
}

func (g *Game) Move(s Session, direc Direction) error {
	return g.UserAction(s, Move, int16(direc))
}

func clipCoord(x, xMin, xMaxPlusOne int) int {
	if x < xMin {
		return xMin
	}

	if x >= xMaxPlusOne {
		return xMaxPlusOne - 1
	}

	return x
}

func (g *Game) handleAction(act Action) {
	pl := act.Player
	if pl == nil {
		return
	}

	user := pl.S.UserName()
	lvl := g.Level

	if pl.BusyTick > 0 {
		return
	}

	// General cooldown timer
	pl.BusyTick = 1

	switch act.Type {
	case Move:
		direc := Direction(act.Arg)
		di, dj, _, _ := direc.Vectors()
		dir := direc.Name()

		newI := clipCoord(pl.I+di, 0, lvl.H)
		newJ := clipCoord(pl.J+dj, 0, lvl.W)

		if what, hasColl := g.hasCollision(newI, newJ); hasColl {
			whatName := "border of space and time"
			if what != nil {
				whatName = what.Name()
			}

			g.messagef(chat.Game, "%s tried to move %s but hit a %s", user, dir, whatName)
			if what != nil {
				switch obj := what.(type) {
				case Marker:
					if obj == MarkerCactus {
						pl.TakeDamage(2, nil)
						g.messagef(chat.Game, "Ouch!  %s takes %d damage from %s", user, 2, what.Name())
					}
				default:
				}
			}
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

func (g *Game) meleeAttack(attacker, victim Unit, weaponItem Item) {
	shortName := weaponItem.ShortName()
	if !victim.IsAlive() {
		g.messagef(chat.Game, "%s swings the %s futility at the %s.",
			attacker.Name(), shortName, victim.Name())
		return
	}

	stats := victim.GetStats()
	toHit := attacker.GetStats().ToHit(stats)

	var dmg int
	switch w := weaponItem.(type) {
	case *MeleeWeapon:
		dmg = w.Damage(victim, g.Dice)
	case Item:
		dmg = 1
	}

	if g.Dice.RollD20() <= toHit {
		g.messagef(chat.Game, "%s slashes %s with a %s for %d damage", attacker.Name(), victim.Name(), shortName, dmg)

		victim.TakeDamage(dmg, attacker)

		if !victim.IsAlive() {
			g.messagef(chat.Game, "%s killed %s", attacker.Name(), victim.Name())
		}
	} else {
		if mob, ok := victim.(*Mob); ok {
			mob.Event = MobEventAttacked
			mob.EventCause = attacker
		}
		g.messagef(chat.Game, "%s swings wildy at %s with a %s but misses", attacker.Name(), victim.Name(), shortName)
	}
}

func (g *Game) playerAttack(pl *Player) {
	weaponItem := pl.Weapon
	if weaponItem == nil {
		weaponItem = BareHands
	}

	shortName := weaponItem.ShortName()
	ui, uj, vi, vj := pl.SwingFacing.Vectors()
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
		return
	}

	swI := pl.I + swDI
	swJ := pl.J + swDJ

	if pl.SwingState > 0 {
		coll, hasColl := g.hasCollision(swI, swJ)
		if coll == nil && hasColl {
			coll = MarkerBorder
		}

		if coll != nil && pl.SwingTick == 0 {
			switch victim := coll.(type) {
			case *Mob:
				g.meleeAttack(pl, victim, weaponItem)

			case Marker:
				if w, ok := weaponItem.(*MeleeWeapon); ok && len(w.HitObjectDescription) > 0 {
					g.messagef(chat.Game, "%s swings the %s futility at the %s.  %s",
						pl.Name(), shortName, coll.Name(), w.HitObjectDescription)
				} else {
					g.messagef(chat.Game, "%s swings the %s futility at the %s.",
						pl.Name(), shortName, coll.Name())
				}

				// Stop swing
				pl.SwingTick = 0
				pl.SwingState = 0

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

		if pl.SwingTick == 0 && pl.SwingState > 0 {
			pl.SwingTick = pl.SwingRate
		}
	}
}

type AABB struct {
	I0, J0, I1, J1 int
}

func (bb *AABB) Width() int {
	return bb.J1 - bb.J0
}

func (bb *AABB) Height() int {
	return bb.I1 - bb.I0
}

func (bb *AABB) Inside(i, j int) bool {
	return (i >= bb.I0) && (i < bb.I1) && (j >= bb.J0) && (j < bb.J1)
}

/*
func (bb *AABB) Intersect(other *AABB) (AABB, bool) {
	// check for no overlap
	if bb.I1 < other.I0 || other.I1 < bb.I0 || bb.J1 < other.J0 || other.J1 < bb.J0 {
		return AABB{}, false
	}

	// check for one AABB enclosing the other
	if bb.I0 <= other.I0 && bb.I1 >= other.I1 && bb.J0 <= other.J0 && bb.J1 >= other.J1 {
		return *other, true
	}

	// FIXME: test!
	i0, j0, i1, j1 := bb.I0, bb.J0, bb.I1, bb.J1
	if other.I0 > i0 {
		i0 = other.I0
	}

	if other.I1 < i1 {
		i1 = other.I1
	}

	if other.J0 > j0 {
		j0 = other.J0
	}

	if other.J1 < j1 {
		j1 = other.J1
	}

	return AABB{I0: i0, J0: j0, I1: i1, J1: j1}, (i0 < i1 && j0 < j1)
}
*/

func minInt(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a >= b {
		return a
	}
	return b
}

func (g *Game) PerceptionArea(mob *Mob) AABB {
	info := LookupMobInfo(mob.Type)

	i := mob.I
	j := mob.J

	lvl := g.Level

	// FIXME: this is a simple placeholder perception
	// approach.  We'll need something better.
	//
	// Downsides:
	//   - expensive: quadratic in the number of mobs+players
	//   - inaccurate: perceive through walls

	ui, uj, vi, vj := mob.Direc.Vectors()

	var i0, j0, i1, j1 int
	if ui == 0 && uj == 0 {
		// MxM square, where M = half view distance (rounded down)
		viewDist := info.ViewDistance / 2
		i0 = i - viewDist
		i1 = i + viewDist

		j0 = j - viewDist
		j1 = j + viewDist
	} else {
		viewDist := info.ViewDistance
		fov := info.FieldOfView

		ui = ui * viewDist
		uj = uj * viewDist

		vi = vi * fov
		vj = vj * fov

		i00 := i + vi
		j00 := j + vj

		i01 := i - vi
		j01 := j - vj

		i10 := i00 + ui
		j10 := j00 + uj

		i11 := i01 + ui
		j11 := j01 + uj

		i0 = minInt(minInt(i00, i01), minInt(i10, i11))
		j0 = minInt(minInt(j00, j01), minInt(j10, j11))
		i1 = maxInt(maxInt(i00, i01), maxInt(i10, i11))
		j1 = maxInt(maxInt(j00, j01), maxInt(j10, j11))
	}

	i0 = clipCoord(i0, 0, lvl.H)
	i1 = clipCoord(i1, 0, lvl.H)

	j0 = clipCoord(j0, 0, lvl.W)
	j1 = clipCoord(j1, 0, lvl.W)

	return AABB{I0: i0, J0: j0, I1: i1, J1: j1}
}

// TODO: both collision detection and "visual perception"
//       will need an overhaul to a better set of data structures
func (g *Game) detectOthers(mob *Mob) []Unit {
	// info := LookupMobInfo(mob.Type)

	// i := mob.I
	// j := mob.J

	// lvl := g.Level

	// FIXME: this is a simple placeholder perception
	// approach.  We'll need something better.
	//
	// Downsides:
	//   - expensive: quadratic in the number of mobs+players
	//   - inaccurate: perceive through walls

	seenUnits := []Unit{}
	pa := g.PerceptionArea(mob)
	for _, pl := range g.Players {
		if pa.Inside(pl.I, pl.J) {
			seenUnits = append(seenUnits, pl)
		}
	}

	for i := range g.Mobs {
		m := &g.Mobs[i]
		if pa.Inside(m.I, m.J) {
			seenUnits = append(seenUnits, m)
		}
	}

	return seenUnits
}

func SignAndMagnitude(val int) (sign int, magnitude int) {

	switch {
	case val > 0:
		sign = 1
		magnitude = val
	case val < 0:
		sign = -1
		magnitude = -val
	case val == 0:
		sign = 0
		magnitude = 0
	}

	return
}

type MoveRelative int

const (
	MoveFarther MoveRelative = -1
	MoveCloser  MoveRelative = 1
)

// relative == +1 moves closer
// relative == -1 moves farther
func (g *Game) mobMoveRelative(mob *Mob, destI, destJ int, moveRel MoveRelative) {
	di := int(moveRel) * (destI - mob.I)
	dj := int(moveRel) * (destJ - mob.J)

	vi, absDI := SignAndMagnitude(di)
	vj, absDJ := SignAndMagnitude(dj)

	lvl := g.Level

	coord := 0
	if absDI < absDJ {
		coord = 1
	}

	for try := 0; try < 2; try++ {
		i1 := mob.I
		j1 := mob.J
		if coord == 0 {
			mob.Direc = Down
			if vi < 0 {
				mob.Direc = Up
			}

			i1 = clipCoord(mob.I+vi, 0, lvl.H)
		} else {
			mob.Direc = Right
			if vj < 0 {
				mob.Direc = Left
			}
			j1 = clipCoord(mob.J+vj, 0, lvl.W)
		}

		_, hasColl := g.hasCollision(i1, j1)
		if !hasColl {
			mob.I = i1
			mob.J = j1
			return
		}

		coord = 1 - coord
	}
}

func (g *Game) mobWander(mob *Mob, wanderRollD20 int) {
	// pick a direction and wander
	if g.Dice.RollD20() <= wanderRollD20 {
		mob.Direc = RollDirection(g.Dice)
	}

	for i := 0; i < 4; i++ {
		di, dj, _, _ := mob.Direc.Vectors()
		i1 := mob.I + di
		j1 := mob.J + dj

		_, hasColl := g.hasCollision(i1, j1)
		if !hasColl {
			mob.I = i1
			mob.J = j1
			return
		}

		mob.Direc = RollDirection(g.Dice)
	}
}

func (g *Game) mobUpdate(mob *Mob) {
	if !mob.IsAlive() {
		return
	}

	// Mob updates based on behavior
	//
	//   -
	//
	//

	mobInfo := LookupMobInfo(mob.Type)
	seenUnits := g.detectOthers(mob)
	// TODO: check for interesting objects within line of sight, too

	if mob.Target != nil && !mob.Target.IsAlive() {
		mob.Target = nil
	}

	// 1. handle any events that have happened to the mob
	switch mob.Event {
	case MobEventAttacked, MobEventHit:
		if mob.Aggression != AggressionPassive {
			if mob.EventCause != nil {
				// TODO: handle multi-unit aggro
				if mob.Target == nil || mob.State == MobSeekTarget {
					mob.Target = mob.EventCause
					ti, tj, _, _ := mob.Target.GetPos()
					mob.LastTargetI = ti
					mob.LastTargetJ = tj

					mob.State = MobSeekTarget
					mob.SeekTick = mobInfo.SeekTargetRate
					mob.MoveTick = mobInfo.ChaseRate
				}

				// TODO: add log message
			}
		} else if mob.Event == MobEventHit || mob.State != MobSentry {
			// passive mobs flee if they take damage or aren't sentries
			if mob.EventCause != nil {
				mob.Target = mob.EventCause
				mob.State = MobFlee
				mob.MoveTick = 1

				ti, tj, _, _ := mob.Target.GetPos()
				mob.LastTargetI = ti
				mob.LastTargetJ = tj

				// TODO: add log message
			}
		}

	case MobEventBadlyHurt:
		if mob.Aggression != AggressionBlindRage && mob.EventCause != nil {
			mob.State = MobFlee
			mob.MoveTick = 1

			if mob.EventCause != nil {
				u := mob.EventCause
				mob.Target = u
				ti, tj, _, _ := u.GetPos()
				mob.LastTargetI = ti
				mob.LastTargetJ = tj
			} else if mob.Target == nil {
				mob.LastTargetI = mob.I
				mob.LastTargetJ = mob.J
			}

			// TODO: add log message
		}
	}

	var targetInSight bool
	var searchedForTarget bool
	// targetSqDist := -1

	// 2. handle any state changes based on seenUnits
	switch mob.Aggression {
	case AggressionPassive, AggressionDefends:
		break

	case AggressionAttacks, AggressionAttacksMobs, AggressionBlindRage:
		// TODO: handle AttacksMobs and BlindRage
		if mob.Target == nil {
			mi := mob.I
			mj := mob.J

			// FIXME: nearest should probably take into account
			// pathfinding.
			//
			// TODO: implement pathfinding...

			// look for the nearest player unit
			var nearest Unit
			var nearestSqDist int
			for _, u := range seenUnits {
				if pl, ok := u.(*Player); ok {
					di := pl.I - mi
					dj := pl.J - mj
					sqdist := di*di + dj*dj
					if nearest == nil || sqdist < nearestSqDist {
						nearest = pl
						nearestSqDist = sqdist
					}
				}
			}

			if nearest != nil {
				mob.Target = nearest
				targetInSight = true
				searchedForTarget = true
				mob.State = MobAttack
				// targetSqDist = nearestSqDist
			}

			// TODO: add log message
		}
	}

	// if mob already has a target, check if the mob can see the target
	if mob.Target != nil && !searchedForTarget {
		for _, u := range seenUnits {
			if u == mob.Target {
				ti, tj, _, _ := mob.Target.GetPos()

				// di := ti - mob.I
				// dj := tj - mob.J
				// targetSqDist = di*di + dj*dj
				targetInSight = true

				mob.LastTargetI = ti
				mob.LastTargetJ = tj
			}
		}
	}

	// handle state transitions
	switch mob.State {
	case MobStill, MobSentry:
		break

	case MobPatrol:
		if mob.Direc == 0 {
			mob.State = MobWander
		}

	case MobAttack:
		// FIXME: this isn't the right transition
		if !mob.Target.IsAlive() {
			mob.Target = nil
		}

		if mob.Target == nil {
			mob.State = MobWander
		} else if !targetInSight {
			mob.State = MobSeekTarget
			mob.SeekTick = mobInfo.SeekTargetRate
		}

	case MobSeekTarget:
		// FIXME: this isn't the right transition
		if mob.Target == nil || mob.SeekTick == 0 {
			mob.State = MobWander
			mob.Target = nil
			mob.LastTargetI = -1
			mob.LastTargetJ = -1
		} else if targetInSight {
			mob.State = MobAttack
		}

	case MobFlee:
		switch mob.Aggression {
		case AggressionAttacks, AggressionAttacksMobs, AggressionBlindRage:
			stats := mob.GetStats()
			if stats.HP > stats.MaxHP/2 {
				mob.State = MobAttack
			}
		}
	}

	// 3. Actually move, attack, use ability, etc.
	switch mob.State {
	case MobStill, MobSentry:
		break

	case MobWander:
		if mob.MoveTick--; mob.MoveTick <= 0 {
			g.mobWander(mob, 7)
			mob.MoveTick = mobInfo.MoveRate
		}

	case MobPatrol:
		if mob.Direc == 0 {
			break
		}

		if mob.MoveTick--; mob.MoveTick <= 0 {
			mob.MoveTick = mobInfo.MoveRate

			di, dj, _, _ := mob.Direc.Vectors()

			i1 := mob.I + di
			j1 := mob.J + dj

			if _, hasColl := g.hasCollision(i1, j1); hasColl {
				i1 = mob.I
				j1 = mob.J

				mob.Direc = mob.Direc.Mirror()
			}

			mob.I = i1
			mob.J = j1
		}

	case MobSeekTarget:
		mob.SeekTick--
		if mob.SeekTick > 0 {
			mob.MoveTick--
			if mob.MoveTick <= 0 {
				var di, dj int
				if mob.LastTargetI >= 0 && mob.LastTargetJ >= 0 {
					// Move toward last known
					di = mob.LastTargetI - mob.I
					dj = mob.LastTargetJ - mob.I

					if di == 0 && dj == 0 {
						mob.LastTargetI = -1
						mob.LastTargetJ = -1
					}
				}

				if mob.LastTargetI >= 0 && mob.LastTargetJ >= 0 {
					// FIXME: this approach yields paths that are really
					// kind of weird.
					//
					// FIXME: Use something like Bresenham's algorithm
					g.mobMoveRelative(mob, mob.LastTargetI, mob.LastTargetJ, MoveCloser)
				} else {
					g.mobWander(mob, 14)
				}

				mob.MoveTick = mobInfo.ChaseRate
			}
		}

	case MobAttack:
		if mob.Target != nil {
			ti, tj, _, _ := mob.Target.GetPos()
			di := ti - mob.I
			dj := tj - mob.J
			sqDist := di*di + dj*dj

			weaponItem := mob.Weapon
			if weaponItem == nil {
				weaponItem = BareHands
			}

			var attackRate int16 = 12
			if w, ok := weaponItem.(*MeleeWeapon); ok {
				attackRate = int16(w.swingTicks)
			}

			// FIXME: weapon length
			if sqDist > 1 {
				mob.AttackTick = attackRate
				if mob.MoveTick--; mob.MoveTick <= 0 {
					g.mobMoveRelative(mob, ti, tj, MoveCloser)
					mob.MoveTick = mobInfo.MoveRate
				}
			} else {
				mob.MoveTick = mobInfo.MoveRate
				if mob.AttackTick--; mob.AttackTick <= 0 {
					mob.AttackTick = attackRate
					g.meleeAttack(mob, mob.Target, mob.Weapon)
				}
			}
		}

	case MobFlee:
		ti, tj, _, _ := mob.Target.GetPos()
		di := ti - mob.I
		dj := tj - mob.J
		sqDist := di*di + dj*dj

		if sqDist < 50 {
			if mob.MoveTick--; mob.MoveTick <= 0 {
				mob.MoveTick = mobInfo.ChaseRate // TODO: add a flee rate

				g.mobMoveRelative(mob, ti, tj, MoveFarther)
			}
		}
	}

	// clear any events handled this tick
	mob.Event = MobEventNone
	mob.EventCause = nil

	// Update random information
	if mob.Target != nil {
		ti, tj, _, _ := mob.Target.GetPos()
		mob.LastTargetI = ti
		mob.LastTargetJ = tj
	}
}

func (g *Game) loopInner() {
	g.mu.Lock()
	defer g.mu.Unlock()

	/*** Game loop ***/

	// user actions
	for _, act := range g.pendingActions {
		if act.Type == Nothing {
			continue
		}

		g.handleAction(act)
	}
	g.pendingActions = g.pendingActions[:0]

	// update rooms

	g.EffectsOverlay = g.EffectsOverlay[:0]

	// player actions
	for _, pl := range g.Players {
		if pl.BusyTick > 0 {
			pl.BusyTick--
		}

		if tick := pl.HealthTick; tick > 0 {
			tick--
			if tick == 0 && pl.Stats.HP < pl.Stats.MaxHP {
				pl.Stats.HP++
				if pl.Stats.HP < pl.Stats.MaxHP {
					tick = pl.Stats.HealthRecoveryRate
				}
			}
			pl.HealthTick = tick
		}

		if pl.SwingState > 0 && pl.SwingFacing != NoDirection {
			pl.SwingTick--
			if pl.SwingTick == 0 {
				pl.SwingState--
			}

			g.playerAttack(pl)
		}
	}

	// update mobs
	for i := range g.Mobs {
		mob := &g.Mobs[i]
		g.mobUpdate(mob)
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

	default:
		return UnknownCommandError
	}

	return nil
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
