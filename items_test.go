package mpnethack

import (
	"strings"
	"testing"
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

	cfg, err := LoadItems(sr)
	if err != nil {
		t.Errorf("error loading items: %v", err)
		return
	}

	if len(cfg.Items) != 2 || len(cfg.MeleeWeapons) != 0 {
		t.Errorf("expected 2 items and 0 weapons, but found %d items and %d weapons", len(cfg.Items), len(cfg.MeleeWeapons))
		return
	}

	expected0 := BasicItem{
		tag:         "rusty_carrot_peeler",
		name:        "Ye olde rusty carrot peeler",
		shortName:   "Rusty carrot peeler",
		description: "It used to peel carrots, but the edge has rusted off.  Maybe you can bludgeon the carrots instead?",
		weight:      5,
	}
	if cfg.Items[0] != expected0 {
		t.Errorf("expected %+v but found %+v", expected0, cfg.Items[0])
	}

	expected1 := BasicItem{
		tag:         "sharpened_carrot_peeler",
		name:        "Ye olde sharpened carrot peeler",
		shortName:   "Sharpened carrot peeler",
		description: "Industrial strength carrot peeler.  Watch out!  That sucker is sharp!",
		weight:      5,
	}
	if cfg.Items[1] != expected1 {
		t.Errorf("expected %+v but found %+v", expected1, cfg.Items[1])
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

	cfg, err := LoadItems(sr)
	if err != nil {
		t.Errorf("error loading items: %v", err)
		return
	}

	if len(cfg.Items) != 0 || len(cfg.MeleeWeapons) != 2 {
		t.Errorf("expected 0 items and 2 weapons, but found %d items and %d weapons", len(cfg.Items), len(cfg.MeleeWeapons))
		return
	}

	expectedWeapons := []MeleeWeapon{
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

	if cfg.MeleeWeapons[0] != expectedWeapons[0] {
		t.Errorf("expected %+v but found %+v", expectedWeapons[0], cfg.MeleeWeapons[0])
	}

	if cfg.MeleeWeapons[1] != expectedWeapons[1] {
		t.Errorf("expected %+v but found %+v", expectedWeapons[1], cfg.MeleeWeapons[1])
	}
}
