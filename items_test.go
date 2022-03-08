package mpnethack

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestUnmarshalItemFromTOML(t *testing.T) {
	sr := strings.NewReader(`
[[items]]
tag = "rusty_carrot_peeler"
name = "Ye olde rusty carrot peeler"
short_name = "Rusty carrot peeler"
description = "It used to peel carrots, but the edge has rusted off.  Maybe you can bludgeon the carrots instead?"
weight = 5


[[items]]
tag = "sharpened_carrot_peeler"
name = "Ye olde sharpened carrot peeler"
short_name = "Sharpened carrot peeler"
description = "Industrial strength carrot peeler.  Watch out!  That sucker is sharp!"
weight = 5
`)

	var loaded struct {
		Items []BasicItem `toml:"items"`
	}

	dec := toml.NewDecoder(sr)
	if _, err := dec.Decode(&loaded); err != nil {
		t.Errorf("error loading items: %v", err)
		return
	}

	expected := []BasicItem{
		{
			tag:         "rusty_carrot_peeler",
			name:        "Ye olde rusty carrot peeler",
			shortName:   "Rusty carrot peeler",
			description: "It used to peel carrots, but the edge has rusted off.  Maybe you can bludgeon the carrots instead?",
			weight:      5,
		},
		{
			tag:         "sharpened_carrot_peeler",
			name:        "Ye olde sharpened carrot peeler",
			shortName:   "Sharpened carrot peeler",
			description: "Industrial strength carrot peeler.  Watch out!  That sucker is sharp!",
			weight:      5,
		},
	}

	if len(loaded.Items) != len(expected) {
		t.Errorf("expected %d items, but found %d items", len(expected), len(loaded.Items))
		return
	}

	for i := range loaded.Items {
		if loaded.Items[i] != expected[i] {
			t.Errorf("item %d: expected %+v but found %+v", i, expected[i], loaded.Items[i])
		}
	}
}

func TestUnmarshalMeleeWeaponFromTOML(t *testing.T) {
	sr := strings.NewReader(`
[[weapons]]
tag = "rust_sword"
name = "rusty sword"
short_name = "rusty sword"
description = "An old sword, made with neither skill nor care.  The blade is pitted and rusty, but serves as an awkward club."
weight = 5
missed_description = "You missed and almost hit yourself!  Thankfully this sword can't get any duller."
damage = "1d4"
swing_arc = 1
swing_length = 1
swing_ticks = 3

[[weapons]]
tag = "rusty_dagger"
name = "rusty dagger"
short_name = "rusty dagger"
description = "An old dagger, made with neither skill nor care.  The blade is pitted and rusty.  A toothpick would be more frightening."
weight = 2
missed_description = "You missed and almost poked yourself!  Thankfully this dagger can't get any duller."
damage = "1d1"
swing_arc = 0
swing_length = 1
swing_ticks = 2

`)

	var loaded struct {
		Weapons []MeleeWeapon `toml:"weapons"`
	}

	dec := toml.NewDecoder(sr)
	if _, err := dec.Decode(&loaded); err != nil {
		t.Errorf("error loading items: %v", err)
		return
	}

	expected := []MeleeWeapon{
		{
			BasicItem: BasicItem{
				tag:         "rust_sword",
				name:        "rusty sword",
				shortName:   "rusty sword",
				description: "An old sword, made with neither skill nor care.  The blade is pitted and rusty, but serves as an awkward club.",
				weight:      5,
			},
			MissedDescription: "You missed and almost hit yourself!  Thankfully this sword can't get any duller.",
			damage:            Roll{M: 1, N: 4},
			swingArc:          1,
			swingLength:       1,
			swingTicks:        3,
		},
		{
			BasicItem: BasicItem{
				tag:         "rusty_dagger",
				name:        "rusty dagger",
				shortName:   "rusty dagger",
				description: "An old dagger, made with neither skill nor care.  The blade is pitted and rusty.  A toothpick would be more frightening.",
				weight:      2,
			},
			MissedDescription: "You missed and almost poked yourself!  Thankfully this dagger can't get any duller.",
			damage:            Roll{M: 1, N: 1},
			swingArc:          0,
			swingLength:       1,
			swingTicks:        2,
		},
	}

	if len(loaded.Weapons) != len(expected) {
		t.Errorf("expected %d weapons, but found %d weapons", len(expected), len(loaded.Weapons))
		return
	}

	for i := range loaded.Weapons {
		if loaded.Weapons[i] != expected[i] {
			t.Errorf("weapon %d: expected %+v but found %+v", i, expected[i], loaded.Weapons[i])
		}
	}
}
