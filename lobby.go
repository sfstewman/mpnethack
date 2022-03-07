package mpnethack

import (
	"log"
	"sync"
)

type Lobby struct {
	Sessions []Session
	Games    []*Game

	mu sync.Mutex
}

func (l *Lobby) NewGame(sess Session) (*Game, error) {
	if g := sess.Game(); g != nil {
		return g, nil
	}

	lvl := SingleRoomLevel(64, 128, 32, 64)

	lvl.PlayerI0 = 64 / 2
	lvl.PlayerJ0 = 128 / 2

	lemmingStats := UnitStats{
		ArmorClass:         8,
		THAC0:              4,
		HP:                 10,
		MaxHP:              10,
		HealthRecoveryRate: 200,
	}

	viciousLemmingStats := UnitStats{
		ArmorClass:         8,
		THAC0:              6,
		HP:                 14,
		MaxHP:              14,
		HealthRecoveryRate: 200,
	}

	mobs := []struct {
		Type  MobType
		Stats UnitStats
		I, J  int
		Direc Direction
		State MobState
	}{
		{MobLemming, lemmingStats, 18, 34, Down, MobPatrol},
		{MobLemming, lemmingStats, 18, 45, Right, MobPatrol},
		{MobLemming, lemmingStats, 45, 92, Up, MobPatrol},
		{MobViciousLemming, viciousLemmingStats, 18, 92, Left, MobWander},

		{MobViciousLemming, viciousLemmingStats, lvl.PlayerI0, lvl.PlayerJ0 + 3, Right /* NoDirection */, MobSentry},
	}

	for _, m := range mobs {
		err := lvl.AddMob(m.Type, m.Stats, m.I, m.J, m.Direc, m.State)
		if err != nil {
			log.Printf("error adding mob \"%v\" @ %d,%d [state=%v]: %v",
				m.Type, m.I, m.J, m.Direc, err)
		}
	}

	lvl.Set(lvl.PlayerI0, lvl.PlayerJ0-3, MarkerCactus)
	lvl.Set(lvl.PlayerI0-2, lvl.PlayerJ0, MarkerCactus)

	g, err := NewGame(lvl)
	if err != nil {
		return nil, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.Games = append(l.Games, g)
	err = sess.Join(g)
	if err != nil {
		return nil, err
	}

	for i, s := range l.Sessions {
		if s == sess {
			l.Sessions = append(l.Sessions[:i], l.Sessions[:i+1]...)
			break
		}
	}

	return g, nil
}

func (l *Lobby) AddSession(sess Session) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.Sessions = append(l.Sessions, sess)
	// signal?
}
