package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

const (
	GameRefreshInterval time.Duration = 33 * time.Millisecond
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

type Game struct {
	mu   sync.RWMutex
	pump *time.Ticker
	Ctx  context.Context

	Active   []*Session
	FrameNum uint64
}

func NewGame(players []*Session, ctx context.Context) *Game {
	g := &Game{
		pump:   time.NewTicker(GameRefreshInterval),
		Active: players,
		Ctx:    ctx,
	}

	return g
}

func (g *Game) Shutdown() {
	g.pump.Stop()
}

func (g *Game) loopInner() {
	g.mu.Lock()
	defer g.mu.Unlock()

	// game loop calculation
	g.FrameNum++
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

func (g *Game) Message(l MsgLevel, s string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

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
