package main

import (
	"context"
	"sync"
	"time"
)

const (
	GameRefreshInterval time.Duration = 33 * time.Millisecond
)

type Game struct {
	mu   sync.RWMutex
	pump *time.Ticker
	Ctx  context.Context

	Active   []Session
	FrameNum uint64
}

func NewGame(players []Session, ctx context.Context) *Game {
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

GameLoop:
	for {
		g.loopInner()

		select {
		case <-doneCh:
			break GameLoop

		case <-updCh:
		}

		g.sendUpdate()
	}
}
