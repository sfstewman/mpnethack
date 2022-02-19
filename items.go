package mpnethack

type Money struct {
	Gold   int
	Silver int
	Copper int
}

type ItemId int

type Item interface {
	Namer

	Tag() string

	Id() ItemId

	ShortName() string
	Description() string

	Weight() int
	// GeneralValue() Money
	// Modifiers() StatModifiers
}

type BasicItem struct {
	id          ItemId
	tag         string
	name        string
	shortName   string
	description string
	weight      int
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

var _ Item = &MeleeWeapon{}

const (
	RustySwordId ItemId = 1000 + iota // FIXME: just some value
	BareHandsId
	LemmingClawsId
)

var RustySword = &MeleeWeapon{
	BasicItem: BasicItem{
		id:          RustySwordId,
		tag:         "rusty_sword",
		name:        "rusty sword",
		shortName:   "rusty sword",
		description: "An old sword, made with neither skill nor care.  The blade is pitted and rusty, but serves as an awkward club.",
		weight:      5,
	},

	MissedDescription: "You missed an almost hit yourself!  Thankfully this sword can't get any duller.",

	damage:      Roll{M: 1, N: 4},
	swingArc:    1,
	swingLength: 1,
	swingTicks:  3,
}

var BareHands = &MeleeWeapon{
	BasicItem: BasicItem{
		id:          BareHandsId,
		tag:         "bare_hards",
		name:        "bare hards",
		shortName:   "bare hands",
		description: "Your fists.  The only thing that beats the personal touch of hired goons.",
		weight:      0,
	},

	MissedDescription: "Maybe hired goons would have been more reliable?",

	damage:      Roll{M: 1, N: 1},
	swingArc:    0,
	swingLength: 1,
	swingTicks:  6,
}

var LemmingClaws = &MeleeWeapon{
	BasicItem: BasicItem{
		id:          LemmingClawsId,
		tag:         "lemming_claws",
		name:        "lemming claws",
		shortName:   "claws",
		description: "Sharp, scary lemming claws",
		weight:      0,
	},

	MissedDescription: "The lemming looks confused, and the claws are still scary.",

	damage:      Roll{M: 1, N: 4},
	swingArc:    0,
	swingLength: 1,
	swingTicks:  12,
}
