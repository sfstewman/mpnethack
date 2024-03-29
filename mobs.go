package mpnethack

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sfstewman/mpnethack/config"
)

type MobType uint32

type MobInfo struct {
	Type MobType
	Tag  string

	Name   string
	Marker rune
	W, H   int

	MoveRate       int16
	ChaseRate      int16
	SeekTargetRate int16

	DefaultWeaponTag  string
	DefaultAggression Aggression

	ViewDistance int
	FieldOfView  int

	InitialState    MobState
	InitialStateArg int
}

const (
	MobLemming MobType = iota
	MobViciousLemming
)

var mobTypes = []MobInfo{
	MobInfo{
		Type:              MobLemming,
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
		Type:              MobViciousLemming,
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

func AddMobType(info MobInfo) MobType {
	mt := MobType(len(mobTypes))

	info.Type = mt
	mobTypes = append(mobTypes, info)

	return mt
}

func LookupMobInfo(mt MobType) (*MobInfo, error) {
	ind := int(mt)
	if ind >= len(mobTypes) {
		return nil, fmt.Errorf("invalid mob type %v", mt)
	}

	return &mobTypes[ind], nil
}

func (mi *MobInfo) UnmarshalTOML(data interface{}) error {
	*mi = MobInfo{}
	var marker string

	err := config.UnmarshalHelper(data, map[string]interface{}{
		"tag":              &mi.Tag,
		"name":             &mi.Name,
		"marker":           &marker,
		"width":            &mi.W,
		"height":           &mi.H,
		"move_rate":        &mi.MoveRate,
		"chase_rate":       &mi.ChaseRate,
		"seek_target_rate": &mi.SeekTargetRate,
		"weapon":           &mi.DefaultWeaponTag,
		"aggression":       &mi.DefaultAggression,
		"view_distance":    &mi.ViewDistance,
		"field_of_view":    &mi.FieldOfView,
		"state":            &mi.InitialState,
		"state_arg":        &mi.InitialStateArg,
	}, config.NoFlags)

	if err != nil {
		return err
	}

	runes := []rune(marker)
	if len(runes) != 1 {
		return fmt.Errorf("expected marker string to have one rune, but found \"%s\" (%d runes)", marker, len(runes))
	}

	mi.Marker = runes[0]

	return nil
}

type MobEvent int

const (
	// No events have happened to this mob
	MobEventNone MobEvent = iota

	// Mob was attacked, but no damage was done
	MobEventAttacked

	// Mob was recently hit
	MobEventHit

	// Mob's health is below 25%
	MobEventBadlyHurt

	// Possible future events:
	// MobEventStunned
	// MobEventFriendDied
	// MobEventHurt
)

type MobState int

const (
	MobStill MobState = iota
	MobSentry
	MobWander
	MobPatrol
	MobSeekTarget
	MobAttack
	MobFlee
)

func (st MobState) String() string {
	switch st {
	case MobStill:
		return "still"
	case MobSentry:
		return "sentry"
	case MobWander:
		return "wander"
	case MobPatrol:
		return "patrol"
	case MobSeekTarget:
		return "seek_target"
	case MobAttack:
		return "attack"
	case MobFlee:
		return "flee"
	default:
		return fmt.Sprintf("state_%d", int(st))
	}
}

func (st *MobState) UnmarshalText(text []byte) error {
	s := string(text)

	switch s {
	case "still":
		*st = MobStill
	case "sentry":
		*st = MobSentry
	case "wander":
		*st = MobWander
	case "patrol":
		*st = MobPatrol
	case "seek_target":
		*st = MobSeekTarget
	case "attack":
		*st = MobAttack
	case "flee":
		*st = MobFlee
	default:
		const prefix = "state_"
		if strings.HasPrefix(s, prefix) {
			suffix := s[len(prefix):]
			i, err := strconv.Atoi(suffix)
			if err != nil {
				*st = MobState(i)
				return nil
			}
		}

		return fmt.Errorf("invalid mob state \"%s\"", s)
	}

	return nil
}

// Mob aggression levels
//
// Loosely indicates what the mob will do when it encounters another character
// or mob
//
// Passive           - mob is passive and will try to run away if attacked
// Defends           - mob will not attack unless attacked
// Attacks           - mob will attack players when they are found
// Attacks mobs      - mob will attack other mobs that are not of its species/tribe/etc.
// Attacks only mobs - mob will attack other mobs that are not of its species/tribe/etc.
//                     but not players, unless attacked
// Blind rage        - mob will attack anything
//
// Aggression is something that can be changed/escalated by the mob's state machine
//   - Attack can become 'Attacks mobs' if attacked by another mob
//
//   - Vicious lemmings start with Aggression='Attacks', but after taking enough damage
//     this will escalate into Aggression='Blind rage'.
//
//   - Lemmings start out as
//
type Aggression int

const (
	AggressionPassive Aggression = iota
	// consider: AggressionStoic: defends if attacked and damaged (or damaged enough)
	AggressionDefends
	AggressionAttacks
	AggressionAttacksMobs
	AggressionBlindRage
)

func (agg Aggression) String() string {

	switch agg {
	case AggressionPassive:
		return "passive"
	case AggressionDefends:
		return "defends"
	case AggressionAttacks:
		return "attacks"
	case AggressionAttacksMobs:
		return "attacks_mobs"
	case AggressionBlindRage:
		return "blind_rage"
	default:
		return fmt.Sprintf("aggression_%d", int(agg))
	}
}

func (agg *Aggression) UnmarshalText(text []byte) error {
	s := string(text)
	switch s {
	case "passive":
		*agg = AggressionPassive
	case "defends":
		*agg = AggressionDefends
	case "attacks":
		*agg = AggressionAttacks
	case "attacks_mobs":
		*agg = AggressionAttacksMobs
	case "blind_rage":
		*agg = AggressionBlindRage
	default:
		return fmt.Errorf("unknown aggression \"%s\"", s)
	}

	return nil
}

type Mob struct {
	I, J int

	Stats UnitStats
	Type  MobType

	MoveTick   int16
	StunTick   int16
	SeekTick   int16
	AttackTick int16

	ActionTick [4]uint16

	Direc Direction

	Weapon Item

	Event      MobEvent
	EventCause Unit

	State    MobState
	StateArg int // helper state for state machine controlling behavior

	Aggression  Aggression
	Target      Unit
	LastTargetI int
	LastTargetJ int
}

var _ Unit = &Mob{}

func (m *Mob) TakeDamage(dmg int, u Unit) {
	hp := m.Stats.HP - dmg
	if hp < 0 {
		hp = 0
		m.Direc = NoDirection
	}

	m.Stats.HP = hp

	if hp < m.Stats.MaxHP/4 {
		m.Event = MobEventBadlyHurt
		m.EventCause = u
	} else {
		m.Event = MobEventHit
		m.EventCause = u
	}
}

func (m *Mob) IsAlive() bool {
	return m.Stats.HP > 0
}

func (m *Mob) GetStats() *UnitStats {
	return &m.Stats
}

func (m *Mob) Name() string {
	info, _ := LookupMobInfo(m.Type)

	var n string
	if info != nil {
		n = info.Name
	} else {
		n = "Mob_Unknown"
	}

	if m.IsAlive() {
		return n
	} else {
		return "dead " + n
	}
}

func (m *Mob) GetMarker() rune {
	info, _ := LookupMobInfo(m.Type)
	if info == nil {
		return 0
	}

	return info.Marker
}

func (m *Mob) GetPos() (i int, j int, h int, w int) {
	info, _ := LookupMobInfo(m.Type)

	i = m.I
	j = m.J

	if info != nil {
		h = info.H
		w = info.W
	} else {
		h = 1
		w = 1
	}

	return
}
