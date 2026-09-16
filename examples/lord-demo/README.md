# Legend of the Red Dragon demo

[`lord.sysml`](lord.sysml) is *Legend of the Red Dragon*, the 1989 BBS door
game, modelled as its rules were written: every mechanic a warrior meets — the
twelve forest levels and the Red Dragon, the three skill trees, Violet and Seth
Able at the inn, gems and charm, rooms and bribes, the slaughter of other
players, the fairies, the Old Hag and the Dark Cloak Tavern, and the town crier
at midnight — is an executable SysML action or transition, guarded so that the
warrior's invariants hold however it is reached. What it is not is the game
as a player saw it: no screens, no saved characters, no other players on the
line. Every output below is what the commands print.

The solver sections need `z3` on `PATH` — see
[installing a solver](../../docs/guide/01-install.md#installing-a-solver-optional).

The four packages:

| Package | What it holds |
| --- | --- |
| `Lord` | the realm: `Fighter` and the `Monster`, `Dragon` and `Master` kinds of it, `Weapon` and `Armour`, King Arthur's Weapons and Abdul's Armour with their thirteen items each, Turgon's hall with its eleven masters, the forest with all twelve levels of monsters, the Red Dragon, the Dark Cloak Tavern and the fairy glade, the bank, the healer, the Red Dragon Inn with Violet's and Seth Able's favours, and the `Town` that holds them |
| `LordPlay` | a `Warrior` with the stats screen's attributes, fourteen constraints the game never breaks, one action per deed the realm offers (`fight`, `fightDragon`, `attack`, `heal`, `deposit`, `withdraw`, `robTheBank`, `buyWeapon`, `buyArmour`, `train`, `learnSkill`, `meetTheOldHag`, `askTheFairies`, `catchAFairy`, `changeProfession`, `gamble`, `buyRoom`, `bribeTheBartender`, `tradeGems`, `flirt`, `divorce`, `listenToTheBard`, `newDay`), the `day` state machine that is the game's menu, and four warriors: `hero`, fresh off the boat; `heroine`, of the other sex and charming enough for the bard; `rival`, whom the slaughter menu offers; and `champion`, at level twelve with the best of both shops and a Death Knight master |
| `LordOdds` | requirements a warrior is checked against — for the dragon, for a wedding, for a slaughter — two the solver is asked to satisfy, one it is asked to explain, and four analyses it is asked to minimize |
| `LordViews` | the menu as a state diagram, the forest fight, the dragon fight, a slaughter and a flirt as flows, the town as a tree, and *The Daily Happenings*, a document with the warrior's standing, both shops' price lists, the masters, both sweethearts' favours and two levels of the forest |

## From the command line

Analyse the model:

```bash
./bin/sysml examples/lord-demo/lord.sysml -validate
```

```
✓ package Lord
✓ package LordPlay
✓ package LordOdds
✓ package LordViews
✓ examples/lord-demo/lord.sysml: no errors
```

**A blow.** `Blow` is the game's damage rule — strength less defense, never
less than one — and `BlowsToKill` counts how many of them a target takes.
`Shielded` halves a blow behind a Mystical Light Shield, `RoomPrice` is the
inn's rate by level, `SkillUses` turns skill points into a day's uses, and
`Interest` is what the bank adds overnight.

```bash
./bin/sysml examples/lord-demo/lord.sysml \
  -calc "LordPlay::Blow(10, 3)" \
  -calc "LordOdds::BlowsToKill(1100, 0, 15000)" \
  -calc "LordPlay::Shielded(2000, true)" \
  -calc "LordPlay::RoomPrice(12)" \
  -calc "LordPlay::SkillUses(40, 4)" \
  -calc "LordPlay::Interest(1500, 10)"
```

```
✓ LordPlay::Blow(10, 3)
  = 7
✓ LordOdds::BlowsToKill(1100, 0, 15000)
  = 14
✓ LordPlay::Shielded(2000, true)
  = 1000
✓ LordPlay::RoomPrice(12)
  = 4800
✓ LordPlay::SkillUses(40, 4)
  = 10
✓ LordPlay::Interest(1500, 10)
  = 150
```

**A fight.** `fight` is one forest encounter, against the Small Thief unless
its `foe` parameter names another monster of the warrior's level, opened with
the warrior's favoured move unless `move` names one. Each round the warrior
swings; whether the swing lands is the game's dice roll, and the model leaves
that roll to the schedule as a decision with two open guards — as it leaves
the monster's chance of dropping a gem. `-instantiate` creates the warrior and
`-action "<action> <object>"` performs the fight on it, so the gold and the
experience land on the hero:

```bash
./bin/sysml examples/lord-demo/lord.sysml \
  -instantiate LordPlay::hero -action "LordPlay::Warrior::fight hero"
```

```
✓ Created instance of LordPlay::hero
  ID: 1
✓ Action completed
  Final state: Completed
  2 choice points; %trace on to see them
  Results:
    foe = Instance(ID: 7)
    foeLeft = -1
    incoming = 0
    move = Move::attack
    rounds = 1
    skillReady = false
```

Under the default schedule the first swing lands and the thief dies in one
round. `-engine check` searches every schedule instead — every sequence of
hits and misses, gem or no gem — and reports what the fight can end as.
`-check-diverge` names the hero's features to compare across outcomes,
`-check-property` names the constraints to hold at every step, and
`-check-witness` writes the schedule that reaches each outcome:

```bash
./bin/sysml examples/lord-demo/lord.sysml -engine check \
  -instantiate LordPlay::hero -action "LordPlay::Warrior::fight hero" \
  -check-diverge this.alive -check-diverge this.gold -check-diverge this.gems \
  -check-property LordPlay::Warrior::theDeadHaveNoHitPoints \
  -check-property LordPlay::Warrior::noDebt \
  -check-property LordPlay::Warrior::gemsNotOwed \
  -check-property LordPlay::Warrior::fightsNotOverdrawn \
  -check-witness witnesses
```

```
✗ Action LordPlay::Warrior::fight: divergent (43 states, 42 moves, depth 22)
  divergent: this.alive ends as false or true
    this.alive = false (witness witnesses/LordPlay.Warrior.fight@hero-this.alive-1.witness)
    this.alive = true (witness witnesses/LordPlay.Warrior.fight@hero-this.alive-2.witness)
  divergent: this.gems ends as 0 or 1
    this.gems = 0 (witness witnesses/LordPlay.Warrior.fight@hero-this.gems-1.witness)
    this.gems = 1 (witness witnesses/LordPlay.Warrior.fight@hero-this.gems-2.witness)
  divergent: this.gold ends as 0 or 556
    this.gold = 0 (witness witnesses/LordPlay.Warrior.fight@hero-this.gold-1.witness)
    this.gold = 556 (witness witnesses/LordPlay.Warrior.fight@hero-this.gold-2.witness)
  outcome: foe = Lord::Forest::level1::smallThief#1{…}; foeLeft = -1; incoming = 0; move = Move::attack; rounds = 1; skillReady = false; this.alive = true; this.gems = 0; this.gold = 556
  outcome: foe = Lord::Forest::level1::smallThief#1{…}; foeLeft = -1; incoming = 0; move = Move::attack; rounds = 1; skillReady = false; this.alive = true; this.gems = 1; this.gold = 556
  …
  outcome: foe = Lord::Forest::level1::smallThief#1{…}; foeLeft = 9; incoming = 5; move = Move::attack; rounds = 2; skillReady = false; this.alive = false; this.gems = 0; this.gold = 0
  standing: sensitive (witnessed: 43 states, 42 moves searched, witness of 2 choices replayed)
```

Five outcomes: the thief dies on the first or the second swing, with or
without a gem in his pockets, or two misses in a row and his dagger — five
points a stab through the hero's single point of defense — kill a
ten-hit-point warrior, who wakes tomorrow with no gold. No property was
violated on any of the 43 states. The witness for the death is the two misses,
and `-schedule replay:` runs it again:

```bash
cat witnesses/LordPlay.Warrior.fight@hero-this.alive-1.witness
./bin/sysml examples/lord-demo/lord.sysml \
  -instantiate LordPlay::hero -action "LordPlay::Warrior::fight hero" \
  -schedule replay:witnesses/LordPlay.Warrior.fight@hero-this.alive-1.witness
```

```
step 7: decision swing -> 2->miss
step 15: decision swing -> 2->miss
…
✓ Action completed
  Final state: Completed
  2 choice points; %trace on to see them
  Results:
    …
    foeLeft = 9
    rounds = 2
```

**The dragon.** `fightDragon` is the fight the game is named for. Only a
twelfth-level warrior with fights left is let near it; each round the warrior
strikes, or spends a skill use on a power move, a sneak attack or one of the
Mystical arts, and the dragon answers with one of its four attacks — the
flaming breath no armour blunts, a stomp, a huge claw or a swish of its tail —
the choice being the dice's. A child of the warrior's may take a blow, a caught
fairy revives the fallen once, and the slayer is born again at level one,
keeping skills, charm, gems, family and the kill. The champion opens with the
Death Knights' power move, three blows for a use:

```bash
./bin/sysml examples/lord-demo/lord.sysml \
  -instantiate LordPlay::champion -action "LordPlay::Warrior::fightDragon champion"
```

Under the default schedule the dragon breathes every round, the champion's
4500 hit points take four breaths, and the fifth power move fells it.
`-engine check` says the dice can do worse:

```bash
./bin/sysml examples/lord-demo/lord.sysml -engine check \
  -instantiate LordPlay::champion -action "LordPlay::Warrior::fightDragon champion" \
  -check-diverge this.alive -check-diverge this.dragonKills \
  -check-property LordPlay::Warrior::levelInRange \
  -check-property LordPlay::Warrior::theDeadHaveNoHitPoints \
  -check-property LordPlay::Warrior::skillsWithinMastery
```

```
✗ Action LordPlay::Warrior::fightDragon: divergent (1811 states, 1958 moves, depth 60)
  divergent: this.alive ends as false or true
  divergent: this.dragonKills ends as 0 or 1
  …
  standing: sensitive (witnessed: 1811 states, 1958 moves searched, witness of 4 choices replayed)
```

Two stomps among the four answers — 1400 each through the champion's 600
defense, more than the breath — and the champion falls in the fourth round;
any gentler four and the dragon does. No property was violated on any of the
1811 states.

**Who may face the dragon.** `-satisfy` evaluates the `assert satisfy`
statements one element makes. `dragonFights` asserts `readyForTheDragon` — a
level-twelve warrior with the top item from each shop — of both warriors, and
three ways of outlasting the dragon of the champion:

```bash
./bin/sysml examples/lord-demo/lord.sysml -satisfy=LordOdds::dragonFights
```

```
✓ satisfy readyForTheDragon by champion holds (on LordPlay::champion ID: 1)
✗ satisfy readyForTheDragon by hero fails (on LordPlay::hero ID: 3)
  Required condition evaluated to false: warrior.level == 12
✗ satisfy outlastTheDragon by champion fails (on LordPlay::champion ID: 1)
  Required condition evaluated to false: warrior.hitPoints + Blow(dragonStrength, warrior.defense) > Blow(dragonStrength, warrior.defense) * BlowsToKill(warrior.strength, 0, dragonHitPoints)
✗ satisfy outlastTheBreath by champion fails (on LordPlay::champion ID: 1)
  Required condition evaluated to false: warrior.hitPoints + breath > breath * BlowsToKill(warrior.strength, 0, dragonHitPoints)
✗ satisfy slayWithPowerMoves by champion fails (on LordPlay::champion ID: 1)
  Required condition evaluated to false: warrior.hitPoints + max(breath, Blow(dragonStrength, warrior.defense)) > max(breath, Blow(dragonStrength, warrior.defense)) * BlowsToKill(3 * warrior.strength, 0, dragonHitPoints)
```

`outlastTheDragon` is a straight brawl, every blow landing and the warrior
swinging first: fourteen blows of 1100 fell the dragon, and thirteen stomps of
1400 land in the meantime. `outlastTheBreath` is the same brawl against the
breath alone. `slayWithPowerMoves` is the Death Knights' way, five power moves
against the dragon's hardest answer every round — the guarantee the checker
above found wanting. The champion's 4500 hit points satisfy none of them; the
solver section below says what would. `weddings` and `slaughter` check the
inn's and the slaughter menu's requirements the same way.

**The views.** `-render` writes a view in the form its definition names.
`townSquare` is the game's menu as a state diagram, one state per place and
one transition per key, guarded as the game guards them. `-render` writes
Mermaid when its output is a pipe or file and plain text at a terminal;
`-render-form mermaid` asks for the diagram either way:

```bash
./bin/sysml examples/lord-demo/lord.sysml -render LordViews::townSquare \
  -render-form mermaid
```

```
stateDiagram-v2
  state "LordPlay::Warrior::day<br>«exhibit state»" as n0 {
    state "townSquare<br>«state»<br>initial" as n1
    state "forest<br>«state»" as n2
    state "darkCloakTavern<br>«state»" as n3
    state "healersHut<br>«state»" as n4
    state "bank<br>«state»" as n5
    state "trainingHall<br>«state»" as n6
    state "inn<br>«state»" as n7
    state "asleep<br>«state»" as n8
    state "slaughter<br>«state»" as n9
    state "otherPlaces<br>«state»" as n10
    state "slain<br>«state»" as n11
    [*] --> n1
  }
  n1 --> n2 : townSquare_forest: accept EnterForest
  n1 --> n4 : townSquare_healer: accept VisitTheHealer / healing
  n1 --> n5 : townSquare_bank: accept VisitTheBank
  n1 --> n6 : townSquare_training: accept VisitTheTrainingHall / training
  n1 --> n7 : townSquare_inn: accept VisitTheInn
  n1 --> n9 : townSquare_slaughter: accept SlaughterOtherPlayers
  n1 --> n10 : townSquare_otherPlaces: accept OtherPlaces
  n1 --> n1 : townSquare_midnight: accept NewDay / midnight
  n2 --> n2 : forest_fight: accept LookForSomethingToKill [forestFightsLeft #gt; 0 and alive] / fighting
  n2 --> n2 : forest_dragon: accept SeekTheDragon [forestFightsLeft #gt; 0 and alive and level == 12] / braving
  n2 --> n2 : forest_guild: accept MeetYourGuild [alive] / learning
  n2 --> n2 : forest_hag: accept MeetTheOldHag [alive and gems #gt;= 1] / bargaining
  n2 --> n2 : forest_fairies: accept AskTheFairies [alive] / asking
  n2 --> n2 : forest_catch: accept CatchAFairy [alive and not fairy] / grabbing
  n2 --> n3 : forest_tavern: accept FindTheDarkCloakTavern [alive]
  n2 --> n11 : forest_slain: [not alive]
  n2 --> n1 : forest_town: accept ReturnToTown
  n3 --> n3 : tavern_gamble: accept Gamble [alive and gold #gt;= 100] / betting
  n3 --> n3 : tavern_profession: accept ChangeProfession [alive] / retraining
  n3 --> n2 : tavern_forest: accept ReturnToTheForest
  n4 --> n1 : healer_town: accept ReturnToTown
  n5 --> n5 : bank_rob: accept RobTheBank [alive and class == CharacterClass::thievingSkills and fairy] / robbing
  n5 --> n1 : bank_town: accept ReturnToTown
  n6 --> n1 : training_town: accept ReturnToTown
  n7 --> n7 : inn_flirt: accept FlirtAtTheInn [alive and not flirtedToday] / flirting
  n7 --> n7 : inn_bard: accept ListenToTheBard [alive and not heardTheBard] / listening
  n7 --> n7 : inn_bribe: accept BribeTheBartender [alive and level #gt;= 2 and not bribed and gold #gt;= BribePrice(level)] / bribing
  n7 --> n7 : inn_gems: accept TradeGems [alive and level #gt;= 2 and gems #gt;= town.inn.gemsPerStatPoint] / trading
  n7 --> n8 : inn_room: accept BuyARoom [alive and not innRoom and (charm #gt;= town.inn.charmForAFreeRoom or gold #gt;= RoomPrice(level))] / lodging
  n7 --> n1 : inn_town: accept ReturnToTown
  n8 --> n1 : asleep_midnight: accept NewDay / midnight
  n9 --> n9 : slaughter_attack: accept AttackAWarrior [playerFightsLeft #gt; 0 and alive] / attacking
  n9 --> n11 : slaughter_slain: [not alive]
  n9 --> n1 : slaughter_town: accept ReturnToTown
  n10 --> n1 : otherPlaces_town: accept ReturnToTown
  n11 --> n1 : slain_midnight: accept NewDay / midnight
```

`forestFight`, `dragonFight`, `slaughter` and `flirting` are the fights and the
flirt as flowcharts, with the merges the rounds loop through and the decisions
the dice make, and `theTown` is the town as a tree:

```bash
./bin/sysml examples/lord-demo/lord.sysml -render LordViews::forestFight
./bin/sysml examples/lord-demo/lord.sysml -render LordViews::dragonFight
./bin/sysml examples/lord-demo/lord.sysml -render LordViews::slaughter
./bin/sysml examples/lord-demo/lord.sysml -render LordViews::flirting
./bin/sysml examples/lord-demo/lord.sysml -render LordViews::theTown
```

**The Daily Happenings.** `-render-document` compiles a document definition,
runs its queries and writes Markdown. With `-instantiate`, the `Verdicts`
query reads the hero the session holds — every constraint asserted of the
warrior and every `satisfy` asserted about it — and the lists are `Project`
over the realm's parts: both shops, the masters, both sweethearts' favours,
and the monsters of two forest levels, the first and the twelfth, each
filtered from the forest by level and ordered by the experience they yield:

```bash
./bin/sysml examples/lord-demo/lord.sysml \
  -instantiate LordPlay::hero -render-document LordViews::DailyHappenings
```

```
# The Daily Happenings

What holds of the warrior, at the town square.

*The warrior's standing*

| path | kind | name | verdict | reason |
| --- | --- | --- | --- | --- |
| LordPlay::hero | constraint | levelInRange | holds |  |
| LordPlay::hero | constraint | fightsNotOverdrawn | holds |  |
| LordPlay::hero | constraint | playerFightsNotOverdrawn | holds |  |
…
| LordPlay::hero | constraint | theBardTakesBrides | holds |  |
| LordPlay::hero | satisfaction |  | violated | satisfaction satisfy readyForTheDragon by hero: require condition evaluated to false: warrior.level == 12 |
| LordPlay::hero | satisfaction |  | violated | satisfaction satisfy mayWedViolet by hero: require condition evaluated to false: warrior.charm >= favour.charmNeeded |
| LordPlay::hero | satisfaction |  | holds |  |

*King Arthur's Weapons*

| tier | strength | price | weapon |
| --- | --- | --- | --- |
| 1 | 5 | 200 | Stick |
…
| 13 | 800 | 40000000 | Nira's Teeth |

*Abdul's Armour*
…
*Turgon's Warrior Training*

| teaches | experienceRequired | hitPoints | strength | defense | master |
| --- | --- | --- | --- | --- | --- |
| 2 | 100 | 15 | 10 | 2 | Halder |
…
| 12 | 10000000 | 800 | 260 | 80 | Turgon |

*Violet's favours*

| charmNeeded | experiencePerLevel | favour |
| --- | --- | --- |
| 1 | 5 | Wink |
…
| 100 | 1000 | Marry Her |

*Seth Able's favours*
…
*The forest, level one*

| weapon | strength | hitPoints | gold | experience | monster |
| --- | --- | --- | --- | --- | --- |
| Sharp Teeth | 3 | 4 | 32 | 1 | Large Green Rat |
…
| Short Sword | 12 | 15 | 234 | 10 | Bran The Warrior |

*The forest, level twelve*

| weapon | strength | hitPoints | gold | experience | monster |
| --- | --- | --- | --- | --- | --- |
| Flaming Breath | 2000 | 15000 | 0 | 0 | The Red Dragon |
| Chant Of Insanity | 1497 | 1383 | 224964 | 39878 | The Wizard Of Darkness |
…
| Spiked Steel Mace | 1800 | 2878 | 524838 | 112833 | Great Ogre Of The North |
```

## From the REPL

```bash
./bin/sysml
```

```
%load examples/lord-demo/lord.sysml
```

Each section below starts afresh — `%clear`, then `%load` again — so the
object numbers in its output are the ones shown.

### A day in the realm

`%instantiate` creates the hero and starts the `day` machine the warrior
exhibits, in `townSquare`. `%state` binds the debugger to that running
machine; `%send` queues one of the menu's signals and `%advance 1` dispatches
it and lets the effect it triggers — a fight, a night's sleep, the stroke of
midnight — run to completion. (`%step` dispatches one event at a time; a
transition whose effect runs a fight takes two.) Looking for something to
kill fights a monster of the warrior's own level — the Small Thief for a
first-level hero, the Corinthian Giant for the twelfth-level champion:

```
%instantiate LordPlay::hero
%state LordPlay::hero
%send EnterForest
%advance 1
%send LookForSomethingToKill
%advance 1
%eval in LordPlay::hero : gold
%eval in LordPlay::hero : forestFightsLeft
%instantiate LordPlay::champion
%state LordPlay::champion
%send EnterForest
%advance 1
%send LookForSomethingToKill
%advance 1
%eval in LordPlay::champion : gold
%state LordPlay::hero
```

```
✓ Sent LookForSomethingToKill to object #1 of "LordPlay::hero"
  Accepted by state machine "day" in state forest: transition forest_fight fires on it
✓ Advanced to 2.0 (2 event(s) processed)
  Current state: forest
  2 choice points; %trace on to see them
✓ gold (on LordPlay::hero ID: 1)
  = 556
✓ forestFightsLeft (on LordPlay::hero ID: 1)
  = 14
…
✓ Sent LookForSomethingToKill to object #7 of "LordPlay::champion"
  Accepted by state machine "day" in state forest: transition forest_fight fires on it
✓ Advanced to 4.0 (2 event(s) processed)
  Current state: forest
  1 choice point; %trace on to see them
✓ gold (on LordPlay::champion ID: 7)
  = 337143
```

Back to town with the hero, a room at the inn for the night, and midnight
gives the fights back and turns the guest out:

```
%send ReturnToTown
%advance 1
%send VisitTheInn
%advance 1
%send BuyARoom
%advance 1
%eval in LordPlay::hero : gold
%send NewDay
%advance 1
%eval in LordPlay::hero : forestFightsLeft
%eval in LordPlay::hero : innRoom
```

```
✓ Sent BuyARoom to object #1 of "LordPlay::hero"
  Accepted by state machine "day" in state inn: transition inn_room fires on it
✓ Advanced to 7.0 (1 event(s) processed)
  Current state: asleep
✓ gold (on LordPlay::hero ID: 1)
  = 156
✓ Sent NewDay to object #1 of "LordPlay::hero"
  Accepted by state machine "day" in state asleep: transition asleep_midnight fires on it
✓ Advanced to 8.0 (1 event(s) processed)
  Current state: townSquare
  1 choice point; %trace on to see them
✓ forestFightsLeft (on LordPlay::hero ID: 1)
  = 15
✓ innRoom (on LordPlay::hero ID: 1)
  = false
```

A signal the current state does not accept is refused naming the state: in
`townSquare`, `%send LookForSomethingToKill` reports `accepts no signal
LookForSomethingToKill now: state machine "day" in state townSquare`. A
signal the state accepts but whose every transition is guarded off — `RobTheBank`
at the bank without a fairy, `TradeGems` with one gem — is refused too, before
it is queued: `would fire no transition on RobTheBank now, so it was not sent:
... the guard of every transition RobTheBank triggers is false`.

### The town's deeds, one at a time

`%invoke` performs one of the warrior's actions on the object directly, with
its parameters named — the monster, the master, the weapon, the favour are the
realm's parts. The Old Man is two blows' work for a first-level warrior and
lands one cane blow of four in between; the healer charges five gold a point
per level; Halder, the first master, turns away a warrior short of a hundred
experience; the bank pays ten percent overnight; the weapon shop refuses a
sale the gold on hand does not cover:

```
%instantiate LordPlay::hero
%invoke LordPlay::hero fight foe=Lord::town.forest.level1.oldMan
%eval in LordPlay::hero : hitPoints
%eval in LordPlay::hero : gold
%invoke LordPlay::hero heal
%eval in LordPlay::hero : hitPoints
%eval in LordPlay::hero : gold
%invoke LordPlay::hero train
%eval in LordPlay::hero : level
%invoke LordPlay::hero deposit amount=300
%invoke LordPlay::hero newDay
%eval in LordPlay::hero : bankGold
%invoke LordPlay::hero buyWeapon weapon=Lord::town.weapons.shortSword
%eval in LordPlay::hero : gold
%constraint LordPlay::hero::noDebt
```

```
✓ Invoked fight on object #1 of "LordPlay::hero"
✓ hitPoints (on LordPlay::hero ID: 1)
  = 6
✓ gold (on LordPlay::hero ID: 1)
  = 573
✓ Invoked heal on object #1 of "LordPlay::hero"
✓ hitPoints (on LordPlay::hero ID: 1)
  = 10
✓ gold (on LordPlay::hero ID: 1)
  = 553
✓ Invoked train on object #1 of "LordPlay::hero"
✓ level (on LordPlay::hero ID: 1)
  = 1
✓ Invoked deposit on object #1 of "LordPlay::hero"
✓ Invoked newDay on object #1 of "LordPlay::hero"
✓ bankGold (on LordPlay::hero ID: 1)
  = 330
✓ Invoked buyWeapon on object #1 of "LordPlay::hero"
✓ gold (on LordPlay::hero ID: 1)
  = 253
✓ Constraint LordPlay::hero::noDebt passed (on LordPlay::hero ID: 1)
  standing: holds (observed: 1 run under reverse)
```

Every deed checks its own preconditions, so the warrior's invariants hold
whichever way it is reached — through the `day` machine's guarded transitions
or directly here — and whatever its parameters say. The forest turns away a
dead warrior or one whose fights are spent, so `forestFightsLeft` never goes
below zero, and a monster of another level or the dragon, so a first-level
warrior never meets the Corinthian Giant and the champion neither preys on
the Small Thief nor meets the dragon outside its lair; the healer and the
masters turn away the dead, so the slain keep
no hit points until morning; the bank moves only gold the warrior has, and
pays interest but never charges it; a shop refuses a price below zero, the
forest a foe with negative stats or gold, and the hall a master with no hit
points to take; a master refuses a twelfth-level warrior, so `train` never
makes a level thirteen; and a warrior may not slaughter himself:

```
%instantiate LordPlay::champion
%invoke LordPlay::champion train master=Lord::town.training.turgon
%eval in LordPlay::champion : level
%invoke LordPlay::hero fight foe=Lord::town.forest.level12.corinthianGiant
%eval in LordPlay::hero : hitPoints
%invoke LordPlay::champion fight foe=Lord::town.forest.level1.smallThief
%invoke LordPlay::champion fight foe=Lord::town.forest.redDragon
%eval in LordPlay::champion : forestFightsLeft
%invoke LordPlay::hero deposit amount=1000
%eval in LordPlay::hero : gold
%invoke LordPlay::hero attack foe=LordPlay::hero
%eval in LordPlay::hero : playerFightsLeft
```

```
✓ Invoked train on object #11 of "LordPlay::champion"
✓ level (on LordPlay::champion ID: 11)
  = 12
✓ Invoked fight on object #1 of "LordPlay::hero"
✓ hitPoints (on LordPlay::hero ID: 1)
  = 10
✓ Invoked fight on object #11 of "LordPlay::champion"
✓ Invoked fight on object #11 of "LordPlay::champion"
✓ forestFightsLeft (on LordPlay::champion ID: 11)
  = 15
✓ Invoked deposit on object #1 of "LordPlay::hero"
✓ gold (on LordPlay::hero ID: 1)
  = 253
✓ Invoked attack on object #1 of "LordPlay::hero"
✓ playerFightsLeft (on LordPlay::hero ID: 1)
  = 3
```

### The skills

Each class has a master who teaches once a day: the Death Knights' castle
judges the warrior, the Mystical hut asks a riddle, the Thieves' Guild takes a
gem for the lesson. Whether the castle judges rightly is the dice, so the
default schedule grants the point; a second visit the same day is refused.
Forty points is mastery, and the master turns away a master. Midnight turns
points into uses — one per four points for the Death Knights and the thieves,
one per point for the mystics — and `fight` and `fightDragon` spend them when
`move` names a skill the warrior has the uses for, or swing the sword when it
does not:

```
%instantiate LordPlay::hero
%invoke LordPlay::hero learnSkill
%eval in LordPlay::hero : deathKnightPoints
%invoke LordPlay::hero learnSkill
%eval in LordPlay::hero : deathKnightPoints
%invoke LordPlay::hero newDay
%eval in LordPlay::hero : deathKnightUses
%invoke LordPlay::hero fight foe=Lord::town.forest.level1.oldMan move=Lord::Move::deathKnight
%eval in LordPlay::hero : hitPoints
%instantiate LordPlay::champion
%invoke LordPlay::champion learnSkill
%eval in LordPlay::champion : taughtToday
%instantiate LordPlay::heroine
%invoke LordPlay::heroine learnSkill
%invoke LordPlay::heroine newDay
%eval in LordPlay::heroine : mysticalUses
%invoke LordPlay::heroine fight foe=Lord::town.forest.level1.oldMan move=Lord::Move::pinchRealHard
%eval in LordPlay::heroine : hitPoints
%eval in LordPlay::heroine : mysticalUses
```

```
✓ deathKnightPoints (on LordPlay::hero ID: 1)
  = 1
✓ deathKnightPoints (on LordPlay::hero ID: 1)
  = 1
✓ deathKnightUses (on LordPlay::hero ID: 1)
  = 0
✓ hitPoints (on LordPlay::hero ID: 1)
  = 6
✓ taughtToday (on LordPlay::champion ID: 7)
  = false
✓ mysticalUses (on LordPlay::heroine ID: 9)
  = 1
✓ hitPoints (on LordPlay::heroine ID: 9)
  = 10
✓ mysticalUses (on LordPlay::heroine ID: 9)
  = 0
```

One point is no use, so the hero swung the sword and took the cane; the
champion's forty points leave nothing to teach; the heroine's one use was
Pinch Real Hard, two blows in one, and the Old Man fell before he could
answer. The spells cost their uses in the game's order — one to pinch, four
to disappear, eight for the heat wave, twelve for the light shield that
halves every blow until the fight ends, sixteen to shatter, twenty for the
mind heal.

### At the inn

`flirt` asks a favour of Violet or of Seth Able, whichever suits the warrior's
sex; each costs charm to be granted and experience by level to ask, once a
day. The heroine has the charm for the bard's hand but not the five
experience a wink costs at level one, so her first wink is refused; a boar
pays for it, and the wink adds a point of charm. Marrying the bard costs no
experience at all — the next day, since the wink was today's favour. A room
at the inn is free to a warrior of 101 charm; a divorce leaves the bard's
bride with thirty:

```
%instantiate LordPlay::heroine
%invoke LordPlay::heroine flirt favour=Lord::town.inn.sethAble.wink
%eval in LordPlay::heroine : charm
%invoke LordPlay::heroine fight foe=Lord::town.forest.level1.wildBoar
%invoke LordPlay::heroine flirt favour=Lord::town.inn.sethAble.wink
%eval in LordPlay::heroine : charm
%invoke LordPlay::heroine flirt favour=Lord::town.inn.sethAble.marryHim
%eval in LordPlay::heroine : spouse
%invoke LordPlay::heroine newDay
%invoke LordPlay::heroine flirt favour=Lord::town.inn.sethAble.marryHim
%eval in LordPlay::heroine : spouse
%invoke LordPlay::heroine buyRoom
%eval in LordPlay::heroine : gold
%invoke LordPlay::heroine divorce
%eval in LordPlay::heroine : charm
```

```
✓ charm (on LordPlay::heroine ID: 1)
  = 125
✓ charm (on LordPlay::heroine ID: 1)
  = 126
✓ spouse (on LordPlay::heroine ID: 1)
  = Spouse::nobody
✓ spouse (on LordPlay::heroine ID: 1)
  = Spouse::sethAble
✓ gold (on LordPlay::heroine ID: 1)
  = 558
✓ charm (on LordPlay::heroine ID: 1)
  = 30
```

The hero, with one point of charm, is refused Violet's hand and the bard's
wink alike, pays four hundred for his room, and cannot bribe the bartender
before level two or trade gems he does not have:

```
%instantiate LordPlay::hero
%invoke LordPlay::hero flirt favour=Lord::town.inn.violet.marryHer
%eval in LordPlay::hero : spouse
%invoke LordPlay::hero flirt favour=Lord::town.inn.sethAble.wink
%eval in LordPlay::hero : charm
%invoke LordPlay::hero buyRoom
%eval in LordPlay::hero : gold
%invoke LordPlay::hero bribeTheBartender
%eval in LordPlay::hero : bribed
%invoke LordPlay::hero tradeGems stat=Lord::Stat::hitPoints
%eval in LordPlay::hero : maxHitPoints
```

```
✓ spouse (on LordPlay::hero ID: 11)
  = Spouse::nobody
✓ charm (on LordPlay::hero ID: 11)
  = 1
✓ gold (on LordPlay::hero ID: 11)
  = 100
✓ bribed (on LordPlay::hero ID: 11)
  = false
✓ maxHitPoints (on LordPlay::hero ID: 11)
  = 10
```

### The slaughter

`attack` is a fight with another warrior, three a day, against the `rival`
unless `foe` names another. The loser's gold and gems go to the winner with a
tenth of the loser's experience, and the slain lie until morning. A warrior
asleep at the inn is out of reach unless the attacker has bribed the
bartender and is within a level of the sleeper:

```
%instantiate LordPlay::hero
%instantiate LordPlay::rival
%invoke LordPlay::hero attack
%eval in LordPlay::hero : gold
%eval in LordPlay::hero : gems
%eval in LordPlay::hero : experience
%eval in LordPlay::hero : playerKills
%eval in LordPlay::rival : alive
%invoke LordPlay::hero attack
%eval in LordPlay::hero : playerFightsLeft
%instantiate LordPlay::heroine
%invoke LordPlay::heroine buyRoom
%invoke LordPlay::hero attack foe=LordPlay::heroine
%eval in LordPlay::hero : playerFightsLeft
```

```
✓ gold (on LordPlay::hero ID: 1)
  = 620
✓ gems (on LordPlay::hero ID: 1)
  = 1
✓ experience (on LordPlay::hero ID: 1)
  = 5
✓ playerKills (on LordPlay::hero ID: 1)
  = 1
✓ alive (on LordPlay::rival ID: 3)
  = false
✓ playerFightsLeft (on LordPlay::hero ID: 1)
  = 2
✓ playerFightsLeft (on LordPlay::hero ID: 1)
  = 2
```

The dead rival is refused as a target, and so is the heroine in her room;
neither refusal costs a fight.

### Other places

Off the town square: the fairies grant a blessing to whoever asks — a kiss
that heals, a horse, a sad story worth a gem, or lore — and one may be caught
and carried, to revive the warrior once in the forest or at the dragon's feet,
or to open the bank's vault to a thief. The Dark Cloak Tavern changes a
warrior's profession and takes wagers; the Old Hag trades a gem for a hit
point and a full heal. Midnight pays the bank's interest, gives the fights
and the uses back, and the town crier's news is the dice:

```
%instantiate LordPlay::hero
%invoke LordPlay::hero deposit amount=400
%invoke LordPlay::hero catchAFairy
%eval in LordPlay::hero : fairy
%invoke LordPlay::hero robTheBank
%eval in LordPlay::hero : gold
%invoke LordPlay::hero changeProfession profession=Lord::CharacterClass::thievingSkills
%invoke LordPlay::hero robTheBank
%eval in LordPlay::hero : gold
%eval in LordPlay::hero : fairy
%invoke LordPlay::hero gamble
%eval in LordPlay::hero : gold
%invoke LordPlay::hero newDay
%eval in LordPlay::hero : bankGold
%eval in LordPlay::hero : news
```

```
✓ fairy (on LordPlay::hero ID: 1)
  = true
✓ gold (on LordPlay::hero ID: 1)
  = 100
✓ gold (on LordPlay::hero ID: 1)
  = 1100
✓ fairy (on LordPlay::hero ID: 1)
  = false
✓ gold (on LordPlay::hero ID: 1)
  = 1200
✓ bankGold (on LordPlay::hero ID: 1)
  = 440
✓ news (on LordPlay::hero ID: 1)
  = "More children are missing today."
```

A Death Knight with a fairy in his pocket cannot pick the lock; a thief with
one takes a thousand a level and the fairy is spent.

### What the solver says

`%check` asks whether a requirement's conditions can hold at all, `%solve`
fills in values that satisfy them, and `%explain` names the conditions that
conflict when they cannot. `FirstDayShopping` is the starting purse plus at
most fifteen fights of first-level gold, spent on a weapon; `FirstDayLongSword`
asks for the Long Sword:

```
%check LordOdds::FirstDayShopping
%solve LordOdds::FirstDayShopping
%check LordOdds::FirstDayLongSword
%explain LordOdds::FirstDayLongSword
```

```
✓ Requirement FirstDayShopping is satisfiable (z3, 9ms)
  LordOdds::FirstDayShopping::forestGold = 500
  LordOdds::FirstDayShopping::startingGold = 500
  LordOdds::FirstDayShopping::weaponPrice = 1000
✓ Requirement FirstDayShopping has values satisfying it (z3, 9ms)
  Synthesised:
    LordOdds::FirstDayShopping::forestGold = 500
    LordOdds::FirstDayShopping::startingGold = 500
    LordOdds::FirstDayShopping::weaponPrice = 1000
  One witness: a solver may answer with any of the assignments that satisfy it.
✗ Requirement FirstDayLongSword is unsatisfiable (z3, 8ms)
✗ Requirement FirstDayLongSword is unsatisfiable: 4 conditions conflict (z3, 42ms)
  Every condition below is needed: dropping any one leaves the rest satisfiable.
  1. required condition: `startingGold == 500` …
  2. required condition: `forestGold <= 15 * 110` …
  3. required condition: `weaponPrice <= startingGold + forestGold` …
  4. required condition: `weaponPrice >= 10000` …
```

A Dagger on the first day, yes; a Long Sword, no, and the four conditions that
rule it out are named.

`%optimize` minimizes an analysis case's objective. `ToughEnough` asks the
fewest hit points that win the straight brawl with the dragon, holding the
best of both shops; `ToughEnoughWithPowerMoves` asks the same of a Death
Knight who opens every round with a power move and takes the dragon's hardest
answer every round:

```
%optimize LordOdds::ToughEnough
%optimize LordOdds::ToughEnoughWithPowerMoves
```

```
✓ Analysis ToughEnough is optimized (z3, 9ms)
  minimize fewestHitPoints = `hitPoints`: 18201
  LordOdds::ToughEnough::defense = 600
  LordOdds::ToughEnough::hitPoints = 18201
  LordOdds::ToughEnough::roundsToKill = 14
  LordOdds::ToughEnough::strength = 1100
✓ Analysis ToughEnoughWithPowerMoves is optimized (z3, 9ms)
  minimize fewestHitPoints = `hitPoints`: 5601
  LordOdds::ToughEnoughWithPowerMoves::defense = 600
  LordOdds::ToughEnoughWithPowerMoves::hitPoints = 5601
  LordOdds::ToughEnoughWithPowerMoves::roundsToKill = 5
  LordOdds::ToughEnoughWithPowerMoves::strength = 1100
```

18201 hit points: no warrior in the game has them, which is why the game
gives its warriors skills. Five power moves need 5601 — the four stomps of
1400 a champion of 600 defense may take before the fifth move lands, and one
more — and a champion of 5601 hit points passes `-engine check` on
`fightDragon` with every outcome a kill. The shipped champion has 4500 and
wins when the dragon breathes, which the checker's defeat witness above shows
it need not. `PowerMoves` asks the fewest lessons that give three uses to a
warrior who will gain five levels, and `GemsForDefense` the gems for five
points of defense: seven and ten.

## What the model leaves open, and where it guesses

The dice are the schedule. The game rolls whether a swing lands, which
attack the dragon answers with, what the fairies grant, whether a monster
drops a gem, whether the castle judges rightly, whether a night upstairs
leaves a child, which song the bard sings and what the crier calls; the model
leaves each open as a decision with several true guards, and lets the
executor, the checker or a replayed witness make it.

The numbers are the game's where the game published them — the shops, the
masters, the monsters' names, weapons, hit points, strength, gold and
experience, the dragon's 15000 and 2000, the inn's 400 a level, Violet's 100
charm and Seth Able's 125, the forty points of mastery. Where it did not, the
model says so in a comment and picks: forest monsters have no published
defense and are given none; the dragon's breath is set at a thousand points
through any armour, the stomp, claw and tail at its strength less the
warrior's defense; a skill point is one use per four points for the Death
Knights and the thieves and one per point for the mystics; a bribe is four
times a room; a bank robbery takes a thousand gold a level; a child grows up
at once to take a blow. None of these is a rule of the executor — each is an
attribute or a calculation in `lord.sysml`, changed by editing it.

The game had a screen, a modem and a hundred players on one bulletin board;
the model has warriors, and `attack` takes one of them as its `foe`. There is
no character file: a warrior lives as long as the REPL session, or the
`-instantiate` that made it.
