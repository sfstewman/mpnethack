package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

var (
	UnknownCommandError  error = errors.New("Unkown command")
	OnCooldownError      error = errors.New("Action still on cooldown")
	InvalidCooldownError error = errors.New("Action has no valid cooldown")
)

const (
	GameRefreshInterval time.Duration = 100 * time.Millisecond
)

// Game message levels
type MsgLevel int

const (
	MsgDebug MsgLevel = iota
	MsgInfo
	MsgChat
	MsgWarn
	MsgCrit
	MsgAdmin
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

const (
	MoveLeft = 1 + iota
	MoveRight
	MoveUp
	MoveDown
)

var UserActionCooldownTicks = [MaxActionType]uint64{
	Nothing: 0,
	Move:    2,
	Attack:  100,
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
	sess *Session
	act  ActionType
}

type action struct {
	Type ActionType
	Arg  uint16
}

const (
	LevelWidth  = 128
	LevelHeight = 128

	DefaultPlayerRow  = 3
	DefaultPlayerCol0 = LevelWidth / 4
)

type MobType uint32

type Mob struct {
	I, J, H, W int

	Type MobType
}

type Player struct {
	I, J int
	S    *Session
}

type Game struct {
	mu   sync.RWMutex
	pump *time.Ticker
	Ctx  context.Context

	Active   []*Session
	FrameNum uint64

	cooldowns map[*Session][]uint64
	actions   map[*Session]action

	Level   *Level
	Players []Player
}

func NewGame(players []*Session, ctx context.Context) *Game {
	g := &Game{
		pump:   time.NewTicker(GameRefreshInterval),
		Active: players,
		Ctx:    ctx,

		cooldowns: make(map[*Session][]uint64),
		actions:   make(map[*Session]action),
		Level:     NewBoxLevel(LevelWidth, LevelHeight),
	}

	g.Players = make([]Player, len(players))
	for i := range g.Players {
		g.Players[i].S = players[i]
		g.Players[i].I = DefaultPlayerRow
		g.Players[i].J = DefaultPlayerCol0 + i
	}

	return g
}

func (g *Game) Shutdown() {
	g.pump.Stop()
}

func (g *Game) GetCooldowns(s *Session, cds Cooldowns) Cooldowns {
	now := g.FrameNum
	last, ok := g.cooldowns[s]
	if !ok {
		if cds == nil {
			return make(Cooldowns, len(zeroCooldowns))
		}

		copy(cds, zeroCooldowns[:])
		return cds
	}

	return calcCooldowns(now, last, cds)
}

func (g *Game) UserAction(s *Session, act ActionType, arg uint16) error {
	if int(act) >= len(UserActionCooldownTicks) {
		return InvalidCooldownError
	}

	// k := actionKey{s, act}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.FrameNum

	actionCDs, ok := g.cooldowns[s]
	if !ok {
		actionCDs = make([]uint64, len(UserActionCooldownTicks))
		g.cooldowns[s] = actionCDs
	}

	last := actionCDs[act]
	if last > 0 && now-last < UserActionCooldownTicks[act] {
		return OnCooldownError
	}

	if act != Nothing {
		actionCDs[act] = now
		fmt.Printf("action %v, cooldowns %v\n", act, actionCDs)
	}

	g.handleAction(s, action{act, arg})

	return nil
}

func (g *Game) handleAction(s *Session, act action) {
	switch act.Type {
	case Move:
		dir := "<unknown>"
		switch act.Arg {
		case MoveLeft:
			dir = "left"
		case MoveRight:
			dir = "right"
		case MoveUp:
			dir = "up"
		case MoveDown:
			dir = "down"
		}

		g.messagef(MsgInfo, "%p moving %s", s, dir)
	case Attack:
		g.messagef(MsgInfo, "%p attacking", s)
	case Defend:
		g.messagef(MsgInfo, "%p defending", s)
	}
}

func (g *Game) loopInner() {
	g.mu.Lock()
	defer g.mu.Unlock()

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

	// update area effects

	// update frame counter
	g.FrameNum++
	// debugging frameno printout
	if g.FrameNum%64 == 0 {
		log.Printf("frame %d", g.FrameNum)
		g.messagef(MsgInfo, "frame %d", g.FrameNum)
	}
}

func (g *Game) sendUpdate() {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, s := range g.Active {
		s.Update()
	}
}

func (g *Game) Loop() {
	// game loop calculation

	doneCh := g.Ctx.Done()
	updCh := g.pump.C

	g.Message(MsgInfo, "Welcome!")

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

func (g *Game) Command(sess *Session, txt string) error {
	switch {
	case txt == "/quit":
		sess.App.Stop()
		return nil

	default:
		return UnknownCommandError
	}
}

func (g *Game) Input(l MsgLevel, sess *Session, txt string) error {
	return g.Message(l, txt)
}

func (g *Game) Messagef(l MsgLevel, fmtStr string, args ...interface{}) error {
	s := fmt.Sprintf(fmtStr, args...)
	return g.Message(l, s)
}

func (g *Game) Message(l MsgLevel, s string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.message(l, s)
}

func (g *Game) messagef(l MsgLevel, fmtStr string, args ...interface{}) error {
	s := fmt.Sprintf(fmtStr, args...)
	return g.message(l, s)
}

// Assumes lock is held (either read or write)
func (g *Game) message(l MsgLevel, s string) error {
	// XXX: global game log
	var errs []error
	for _, sess := range g.Active {
		err := sess.Message(l, s)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if errs == nil {
		return nil
	}

	// XXX: fix this!
	return errors.New("multiple errors")
}
