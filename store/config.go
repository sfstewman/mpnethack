package store

import (
	"fmt"
	"io"
	"log"

	"github.com/BurntSushi/toml"
	"github.com/sfstewman/mpnethack"
)

func LoadItems(db *DB, r io.Reader) error {
	var configItems struct {
		Items   []mpnethack.BasicItem   `toml:"items"`
		Weapons []mpnethack.MeleeWeapon `toml:"weapons"`
	}

	dec := toml.NewDecoder(r)
	if _, err := dec.Decode(&configItems); err != nil {
		return fmt.Errorf("error decoding items: %w", err)
	}

	for i := range configItems.Items {
		itm := &configItems.Items[i]

		err := db.addItem(itm)
		if err != nil {
			log.Printf("Error adding basic item \"%s\" to db store: %v", itm.Tag(), err)
		} else {
			log.Printf("Added basic item %+v[\"%s\"] to db store", itm.Id(), itm.Tag())
		}
	}

	for i := range configItems.Weapons {
		itm := &configItems.Weapons[i]

		err := db.addItem(itm)
		if err != nil {
			log.Printf("Error adding weapon \"%s\" o store: %v", itm.Tag(), err)
		} else {
			log.Printf("Added weapon %+v[\"%s\"] to db store", itm.Id(), itm.Tag())
		}
	}

	return nil
}
