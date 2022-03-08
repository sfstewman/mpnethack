package mpnethack

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestUnmarshalMobFromTOML(t *testing.T) {
	sr := strings.NewReader(`
[[mobs]]
tag              = "lemming"
name             = "Lemming"
marker           = "L"
width            = 1
height           = 1
move_rate        = 10
chase_rate       = 8
seek_target_rate = 300
weapon           = "lemming_claws"
aggression       = "defends"
view_distance    = 3
field_of_view    = 3
state            = "patrol"

[[mobs]]
tag              = "vicious_lemming"
name             = "Vicious lemming"
marker           = "V"
width            = 1
height           = 1
move_rate        = 5
chase_rate       = 3
seek_target_rate = 200
weapon           = "lemming_claws"
aggression       = "attacks"
view_distance    = 3
field_of_view    = 3
state            = "patrol"
`)

	var loaded struct {
		Mobs []MobInfo `toml:"mobs"`
	}

	dec := toml.NewDecoder(sr)
	if _, err := dec.Decode(&loaded); err != nil {
		t.Errorf("error loading items: %v", err)
		return
	}

	expected := []MobInfo{
		MobInfo{
			Tag:               "lemming",
			Name:              "Lemming",
			Marker:            'L',
			W:                 1,
			H:                 1,
			MoveRate:          10,
			ChaseRate:         8,
			SeekTargetRate:    300,
			DefaultWeaponTag:  "lemming_claws", // LemmingClaws,
			DefaultAggression: AggressionDefends,
			ViewDistance:      3,
			FieldOfView:       3,
			InitialState:      MobPatrol,
		},
		MobInfo{
			Tag:               "vicious_lemming",
			Name:              "Vicious lemming",
			Marker:            'V',
			W:                 1,
			H:                 1,
			MoveRate:          5,
			ChaseRate:         3,
			SeekTargetRate:    200,
			DefaultWeaponTag:  "lemming_claws", // LemmingClaws,
			DefaultAggression: AggressionAttacks,
			ViewDistance:      3,
			FieldOfView:       3,
			InitialState:      MobPatrol,
		},
	}

	if len(loaded.Mobs) != len(expected) {
		t.Errorf("expected %d weapons, but found %d weapons", len(expected), len(loaded.Mobs))
		return
	}

	for i := range loaded.Mobs {
		if loaded.Mobs[i] != expected[i] {
			t.Errorf("mob %d: expected %+v but found %+v", i, expected[i], loaded.Mobs[i])
		}
	}
}
