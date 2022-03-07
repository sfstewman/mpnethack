package store

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/sfstewman/mpnethack"
	// bolt "go.etcd.io/bbolt"
)

type DB struct {
	// db *bolt.DB
	// map[string]*mpnethack.Player

	mobs   map[string]mpnethack.MobType
	levels map[string]*mpnethack.Level

	lastItemId mpnethack.ItemId
	items      map[string]mpnethack.Item

	mu sync.RWMutex
}

const (
	FirstItemId mpnethack.ItemId = 1000
)

var ErrStoredItemHasNullId = errors.New("stored item has null item id")

func Open(path string) (*DB, error) {
	//db, err := bolt.Open(path,

	db := &DB{
		mobs:   make(map[string]mpnethack.MobType),
		levels: make(map[string]*mpnethack.Level),

		lastItemId: FirstItemId,
		items:      make(map[string]mpnethack.Item),
	}

	return db, nil
}

func (db *DB) registerItem(tag string, item mpnethack.Item) (mpnethack.ItemId, error) {
	if stored, ok := db.items[tag]; ok {
		id := stored.Id()
		if id == mpnethack.NullItemId {
			return mpnethack.NullItemId, ErrStoredItemHasNullId
		}

		return id, nil
	}

	db.lastItemId++
	return db.lastItemId, nil
}

type dbRegistrar struct {
	db *DB
}

func (dbr dbRegistrar) RegisterItem(tag string, item mpnethack.Item) (mpnethack.ItemId, error) {
	return dbr.db.registerItem(tag, item)
}

func (db *DB) addItem(item mpnethack.Item) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tag := item.Tag()
	if _, ok := db.items[tag]; ok {
		log.Printf("db store already has item \"%s\"", tag)
	} else {
		id, err := item.Register(dbRegistrar{db: db})
		if err != nil {
			return fmt.Errorf("error registering item \"%s\": %w", tag, err)
		}

		log.Printf("registered item \"%s\" with id %v", tag, id)

		db.items[tag] = item
	}

	return nil
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

func (db *DB) LookupItem(tag string) (mpnethack.Item, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	itm := db.items[tag]
	return itm, nil
}

func (db *DB) LookupItems(name []string) ([]mpnethack.Item, error) {
	return nil, nil
}
