package mpnethack

type Money struct {
	Gold   int
	Silver int
	Copper int
}

type ItemKind int

type Item interface {
	Namer

	ShortName() string
	Description() string

	Weight() int
	// GeneralValue() Money
	// Modifiers() StatModifiers
}

type BasicItem struct {
	name        string
	shortName   string
	description string
	weight      int
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

var _ Item = &MeleeWeapon{}
