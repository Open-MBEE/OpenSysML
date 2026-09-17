package lord

// Snapshot is the warrior's standing as the model holds it, read attribute by
// attribute from the hero instance; the screen renders it and outcomes diff it.
type Snapshot struct {
	Name             string `json:"name"`
	Sex              string `json:"sex"`
	Class            string `json:"class"`
	Level            int64  `json:"level"`
	Experience       int64  `json:"experience"`
	HitPoints        int64  `json:"hitPoints"`
	MaxHitPoints     int64  `json:"maxHitPoints"`
	Strength         int64  `json:"strength"`
	Defense          int64  `json:"defense"`
	WeaponTier       int64  `json:"weaponTier"`
	ArmourTier       int64  `json:"armourTier"`
	Gold             int64  `json:"gold"`
	BankGold         int64  `json:"bankGold"`
	Gems             int64  `json:"gems"`
	Charm            int64  `json:"charm"`
	DeathKnightUses  int64  `json:"deathKnightUses"`
	MysticalUses     int64  `json:"mysticalUses"`
	ThievingUses     int64  `json:"thievingUses"`
	DeathKnightSkill int64  `json:"deathKnightPoints"`
	MysticalSkill    int64  `json:"mysticalPoints"`
	ThievingSkill    int64  `json:"thievingPoints"`
	FavouredMove     string `json:"favouredMove"`
	ForestFightsLeft int64  `json:"forestFightsLeft"`
	PlayerFightsLeft int64  `json:"playerFightsLeft"`
	Alive            bool   `json:"alive"`
	DragonKills      int64  `json:"dragonKills"`
	PlayerKills      int64  `json:"playerKills"`
	Spouse           string `json:"spouse"`
	Children         int64  `json:"children"`
	Horse            bool   `json:"horse"`
	Fairy            bool   `json:"fairy"`
	InnRoom          bool   `json:"innRoom"`
	Bribed           bool   `json:"bribed"`
	TrainedToday     bool   `json:"trainedToday"`
	News             string `json:"news"`
}

// Snapshot reads the warrior's attributes from the model.
func (g *Game) Snapshot() (*Snapshot, error) {
	s := &Snapshot{}
	var err error
	read := func(step func() error) {
		if err == nil {
			err = step()
		}
	}
	str := func(dst *string, attribute string) {
		read(func() (e error) { *dst, e = g.String(attribute); return })
	}
	lit := func(dst *string, attribute string) {
		read(func() (e error) { *dst, e = g.Literal(attribute); return })
	}
	num := func(dst *int64, attribute string) {
		read(func() (e error) { *dst, e = g.Int(attribute); return })
	}
	flag := func(dst *bool, attribute string) {
		read(func() (e error) { *dst, e = g.Bool(attribute); return })
	}
	str(&s.Name, "name")
	lit(&s.Sex, "sex")
	lit(&s.Class, "class")
	num(&s.Level, "level")
	num(&s.Experience, "experience")
	num(&s.HitPoints, "hitPoints")
	num(&s.MaxHitPoints, "maxHitPoints")
	num(&s.Strength, "strength")
	num(&s.Defense, "defense")
	num(&s.WeaponTier, "weaponTier")
	num(&s.ArmourTier, "armourTier")
	num(&s.Gold, "gold")
	num(&s.BankGold, "bankGold")
	num(&s.Gems, "gems")
	num(&s.Charm, "charm")
	num(&s.DeathKnightUses, "deathKnightUses")
	num(&s.MysticalUses, "mysticalUses")
	num(&s.ThievingUses, "thievingUses")
	num(&s.DeathKnightSkill, "deathKnightPoints")
	num(&s.MysticalSkill, "mysticalPoints")
	num(&s.ThievingSkill, "thievingPoints")
	lit(&s.FavouredMove, "favouredMove")
	num(&s.ForestFightsLeft, "forestFightsLeft")
	num(&s.PlayerFightsLeft, "playerFightsLeft")
	flag(&s.Alive, "alive")
	num(&s.DragonKills, "dragonKills")
	num(&s.PlayerKills, "playerKills")
	lit(&s.Spouse, "spouse")
	num(&s.Children, "children")
	flag(&s.Horse, "horse")
	flag(&s.Fairy, "fairy")
	flag(&s.InnRoom, "innRoom")
	flag(&s.Bribed, "bribed")
	flag(&s.TrainedToday, "trainedToday")
	str(&s.News, "news")
	if err != nil {
		return nil, err
	}
	return s, nil
}
