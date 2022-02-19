package db

import (
	"github.com/sfstewman/mpnethack"
	// bolt "go.etcd.io/bbolt"
)

type DB struct {
	// db *bolt.DB
	// map[string]*mpnethack.Player

	mobs   map[string]mpnethack.MobType
	levels map[string]*mpnethack.Level

	items map[string]mpnethack.Item
}

func Open(path string) (*DB, error) {
	//db, err := bolt.Open(path,
	return nil, nil
}

/*
func (db *DB) LookupPlayer(session *mpnethack.Session, name string) (*mpnethack.Player, error) {
	return nil, nil
}
*/

func (db *DB) LookupMob(name string) (mpnethack.MobType, error) {
	return 0, nil
}

func (db *DB) LookupLevel(name string) (*mpnethack.Level, error) {
	return nil, nil
}

func (db *DB) LookupItem(name string) (Item, error) {
	return nil, nil
}

func (db *DB) LookupItems(name []string) ([]Item, error) {
	return nil, nil
}
