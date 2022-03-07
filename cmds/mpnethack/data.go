package main

import (
	"embed"
	"fmt"

	"github.com/sfstewman/mpnethack/store"
)

//go:embed *.toml
var builtinData embed.FS

func LoadBuiltinData(db *store.DB) error {
	f, err := builtinData.Open("items.toml")
	if err != nil {
		return fmt.Errorf("error loading items: %w", err)
	}

	if err := store.LoadItems(db, f); err != nil {
		return fmt.Errorf("error loading items: %w", err)
	}

	return nil
}
