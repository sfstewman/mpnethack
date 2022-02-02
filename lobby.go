package mpnethack

import "sync"

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
		ArmorClass: 8,
		THAC0:      4,
		HP:         10,
	}

	viciousLemmingStats := UnitStats{
		ArmorClass: 8,
		THAC0:      6,
		HP:         14,
	}

	lvl.AddMob(MobLemming, lemmingStats, 18, 34, Down)
	lvl.AddMob(MobLemming, lemmingStats, 18, 45, Right)
	lvl.AddMob(MobLemming, lemmingStats, 45, 92, Up)
	lvl.AddMob(MobViciousLemming, viciousLemmingStats, 18, 92, Left)

	lvl.AddMob(MobViciousLemming, viciousLemmingStats, lvl.PlayerI0, lvl.PlayerJ0+3, NoDirection)

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
