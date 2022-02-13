package mpnethack

type Cooldowns []uint32

var zeroCooldowns = [MaxActionType]uint32{}

func calcCooldowns(now uint64, last []uint64, cd Cooldowns) Cooldowns {
	if cd == nil {
		cd = make(Cooldowns, len(last))
	}

	for i, when := range last {
		if i >= MaxActionType || when == 0 {
			cd[i] = 0
			continue
		}

		nextTime := when + UserActionCooldownTicks[i]
		if now >= nextTime {
			cd[i] = 0
			continue
		}

		cd[i] = uint32(nextTime - now)
	}

	return cd
}

type Action struct {
	Player *Player
	Type   ActionType
	Arg    int16
}

const (
	DefaultPlayerRow  = 3
	DefaultPlayerCol0 = LevelWidth / 4
)

type Player struct {
	S      Session
	I, J   int
	Marker rune
	Facing Direction

	Inventory []Item
	Weapon    Item

	Cooldowns []uint64

	Stats UnitStats

	BusyTick   int16
	HealthTick int16

	SwingRate   int16
	SwingTick   int16
	SwingState  int16
	SwingFacing Direction
}

var _ Unit = &Player{}

func (p *Player) GetStats() *UnitStats {
	return &p.Stats
}

func (p *Player) IsAlive() bool {
	return p.Stats.HP > 0
}

func (p *Player) TakeDamage(dmg int, u Unit) {
	hp := p.Stats.HP - dmg
	if hp <= 0 {
		hp = 0
		p.BusyTick = 0
		p.HealthTick = 0
		p.SwingTick = 0
		p.SwingState = 0
		p.SwingFacing = NoDirection
	}

	p.Stats.HP = hp
	if hp < p.Stats.MaxHP {
		p.HealthTick = p.Stats.HealthRecoveryRate
	}
}

func (p *Player) Name() string {
	return p.S.UserName()
}

func (p *Player) GetMarker() rune {
	return p.Marker
}

func (p *Player) GetPos() (i int, j int, h int, w int) {
	i = p.I
	j = p.J
	h = 1
	w = 1
	return
}
