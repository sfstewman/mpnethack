package mpnethack

import (
	"encoding/json"

	"github.com/sfstewman/mpnethack/config"
)

type Money struct {
	Gold   int
	Silver int
	Copper int
}

type ItemId int

const (
	NullItemId ItemId = 0
)

type Item interface {
	Namer

	Tag() string

	Id() ItemId

	ShortName() string
	Description() string

	Weight() int
	// GeneralValue() Money
	// Modifiers() StatModifiers

	Register(registrar ItemRegistrar) (ItemId, error)
}

type BasicItem struct {
	id          ItemId
	tag         string
	name        string
	shortName   string
	description string
	weight      int
}

type ItemRegistrar interface {
	RegisterItem(tag string, item Item) (ItemId, error)
}

func (itm *BasicItem) Register(registrar ItemRegistrar) (ItemId, error) {
	if itm.id != NullItemId {
		return itm.id, nil
	}

	id, err := registrar.RegisterItem(itm.tag, itm)
	if err != nil {
		return NullItemId, err
	}

	itm.id = id
	return id, nil
}

func (itm *BasicItem) UnmarshalTOML(data interface{}) error {
	*itm = BasicItem{}
	return config.UnmarshalHelper(data, map[string]interface{}{
		"tag":         &itm.tag,
		"name":        &itm.name,
		"short_name":  &itm.shortName,
		"description": &itm.description,
		"weight":      &itm.weight,
	}, config.NoFlags)
}

func (itm *BasicItem) MarshalJSON() ([]byte, error) {
	out := struct {
		Id          ItemId `json:"id"`
		Name        string `json:"name"`
		ShortName   string `json:"short_name"`
		Description string `json:"description"`
		Weight      int    `json:"weight"`
	}{
		Id:          itm.id,
		Name:        itm.name,
		ShortName:   itm.shortName,
		Description: itm.description,
		Weight:      itm.weight,
	}

	return json.Marshal(&out)
}

func (itm *BasicItem) Tag() string {
	return itm.tag
}

func (itm *BasicItem) Id() ItemId {
	return itm.id
}

func (itm *BasicItem) Name() string {
	return itm.name
}

func (itm *BasicItem) ShortName() string {
	return itm.shortName
}

func (itm *BasicItem) Description() string {
	return itm.description
}

func (itm *BasicItem) Weight() int {
	return itm.weight
}

var _ Item = &BasicItem{}

type MeleeWeapon struct {
	BasicItem

	MissedDescription string

	damage      Roll
	swingArc    int
	swingLength int
	swingTicks  int
}

func (w *MeleeWeapon) DamageRoll(u Unit) Roll {
	return w.damage
}

func (w *MeleeWeapon) Damage(u Unit, d Dice) int {
	return w.DamageRoll(u).Roll(d)
}

func (w *MeleeWeapon) SwingStats() (arc int, length int, ticks int) {
	return w.swingArc, w.swingLength, w.swingTicks
}

func (w *MeleeWeapon) UnmarshalTOML(data interface{}) error {
	*w = MeleeWeapon{}
	if err := w.BasicItem.UnmarshalTOML(data); err != nil {
		return err
	}

	return config.UnmarshalHelper(data, map[string]interface{}{
		"missed_description": &w.MissedDescription,
		"damage":             &w.damage,
		"swing_arc":          &w.swingArc,
		"swing_length":       &w.swingLength,
		"swing_ticks":        &w.swingTicks,
	}, config.NoFlags)
}

var _ Item = &MeleeWeapon{}

var LookupItem func(tag string) (Item, error)
var BareHands *MeleeWeapon
