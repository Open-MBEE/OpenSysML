# Legend of the Red Dragon demo

[`lord.sysml`](lord.sysml) is *Legend of the Red Dragon*, the 1989 BBS door
game, modelled far enough to play a day of it from the REPL and to ask the
tool the questions a player asks: what a blow does, whether a fight can be
lost, how much a first day can afford, and what it takes to slay the dragon.
Every output below is what the commands print.

The solver sections need `z3` on `PATH` — see
[installing a solver](../../docs/guide/01-install.md#installing-a-solver-optional).

The four packages:

| Package | What it holds |
| --- | --- |
| `Lord` | the realm: `Fighter` and the `Monster`, `Dragon` and `Master` kinds of it, `Weapon` and `Armour`, King Arthur's Weapons and Abdul's Armour with their thirteen items each, Turgon's training hall with its eleven masters, the forest with its first-level monsters and the Red Dragon, the bank, the healer, the inn, and the `Town` that holds them |
| `LordPlay` | a `Warrior` with the stats screen's attributes, four constraints the game never breaks, one action per place in town (`fight`, `heal`, `deposit`, `withdraw`, `buyWeapon`, `buyArmour`, `train`, `newDay`), the `day` state machine that is the game's menu, and two warriors: `hero`, fresh off the boat, and `champion`, at level twelve with the best of both shops |
| `LordOdds` | requirements a warrior is checked against, two the solver is asked to satisfy, one it is asked to explain, and an analysis it is asked to minimize |
| `LordViews` | the menu as a state diagram, a fight as a flow, the town as a tree, and *The Daily Happenings*, a document with the warrior's standing and both shops' price lists |

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
`Interest` is what the bank adds overnight.

```bash
./bin/sysml examples/lord-demo/lord.sysml \
  -calc "LordPlay::Blow(10, 3)" \
  -calc "LordPlay::Blow(3, 50)" \
  -calc "LordOdds::BlowsToKill(1100, 0, 15000)" \
  -calc "LordPlay::Interest(1500, 10)"
```

```
✓ LordPlay::Blow(10, 3)
  = 7
✓ LordPlay::Blow(3, 50)
  = 1
✓ LordOdds::BlowsToKill(1100, 0, 15000)
  = 14
✓ LordPlay::Interest(1500, 10)
  = 150
```

**A fight.** `fight` is one forest encounter, against the Large Mosquito unless
its parameters say otherwise. Each round the warrior swings; whether the swing
lands is the game's dice roll, and the model leaves that roll to the schedule
as a decision with two open guards. `-instantiate` creates the warrior and
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
  1 choice point; %trace on to see them
  Results:
    foeDefense = 0
    foeExperience = 1
    foeGold = 34
    foeHitPoints = 5
    foeLeft = -5
    foeStrength = 3
    rounds = 1
```

Under the default schedule the first swing lands and the mosquito dies in one
round. `-engine check` searches every schedule instead — every sequence of
hits and misses — and reports what the fight can end as. `-check-diverge`
names the hero's features to compare across outcomes, `-check-property` names
the constraints to hold at every step, and `-check-witness` writes the schedule
that reaches each outcome:

```bash
./bin/sysml examples/lord-demo/lord.sysml -engine check \
  -instantiate LordPlay::hero -action "LordPlay::Warrior::fight hero" \
  -check-diverge this.alive -check-diverge this.gold \
  -check-property LordPlay::Warrior::theDeadHaveNoHitPoints \
  -check-property LordPlay::Warrior::noDebt \
  -check-property LordPlay::Warrior::fightsNotOverdrawn \
  -check-witness witnesses
```

```
✗ Action LordPlay::Warrior::fight: divergent (56 states, 55 moves, depth 30)
  divergent: this.alive ends as false or true
    this.alive = false (witness witnesses/LordPlay.Warrior.fight@hero-this.alive-1.witness)
    this.alive = true (witness witnesses/LordPlay.Warrior.fight@hero-this.alive-2.witness)
  divergent: this.gold ends as 0 or 534
    this.gold = 0 (witness witnesses/LordPlay.Warrior.fight@hero-this.gold-1.witness)
    this.gold = 534 (witness witnesses/LordPlay.Warrior.fight@hero-this.gold-2.witness)
  outcome: … rounds = 1; this.alive = true; this.gold = 534
  outcome: … rounds = 2; this.alive = true; this.gold = 534
  outcome: … rounds = 3; this.alive = true; this.gold = 534
  outcome: … rounds = 4; this.alive = true; this.gold = 534
  outcome: … rounds = 5; this.alive = true; this.gold = 534
  outcome: … rounds = 5; this.alive = false; this.gold = 0
  standing: sensitive (witnessed: 56 states, 55 moves searched, witness of 5 choices replayed)
```

Six outcomes: the mosquito dies on the first to fifth swing, or five misses in
a row and its five bites kill a ten-hit-point warrior, who wakes tomorrow with
no gold. No property was violated on any of the 56 states. The witness
for the death is the five misses, and `-schedule replay:` runs it again:

```bash
cat witnesses/LordPlay.Warrior.fight@hero-this.alive-1.witness
./bin/sysml examples/lord-demo/lord.sysml \
  -instantiate LordPlay::hero -action "LordPlay::Warrior::fight hero" \
  -schedule replay:witnesses/LordPlay.Warrior.fight@hero-this.alive-1.witness
```

```
step 5: decision swing -> 2->miss
step 10: decision swing -> 2->miss
step 15: decision swing -> 2->miss
step 20: decision swing -> 2->miss
step 25: decision swing -> 2->miss
…
✓ Action completed
  Final state: Completed
  5 choice points; %trace on to see them
  Results:
    …
    foeLeft = 5
    rounds = 5
```

**Who may face the dragon.** `-satisfy` evaluates the `assert satisfy`
statements one element makes. `dragonFights` asserts `readyForTheDragon` — a
level-twelve warrior with the top item from each shop — of both warriors, and
`outlastTheDragon` of the champion:

```bash
./bin/sysml examples/lord-demo/lord.sysml -satisfy=LordOdds::dragonFights
```

```
✓ satisfy readyForTheDragon by champion holds (on LordPlay::champion ID: 1)
✗ satisfy readyForTheDragon by hero fails (on LordPlay::hero ID: 3)
  Required condition evaluated to false: warrior.level == 12
✗ satisfy outlastTheDragon by champion fails (on LordPlay::champion ID: 1)
  Required condition evaluated to false: warrior.hitPoints + Blow(dragonStrength, warrior.defense) > Blow(dragonStrength, warrior.defense) * BlowsToKill(warrior.strength, 0, dragonHitPoints)
```

`outlastTheDragon` is a straight brawl, every blow landing and the warrior
swinging first: fourteen blows of 1100 fell the dragon, and thirteen of its
blows of 1400 land in the meantime. The champion's 1200 hit points do not
survive them; the solver section below says what would.

**The views.** `-render` writes a view in the form its definition names.
`townSquare` is the game's menu as a state diagram. `-render` writes Mermaid
when its output is a pipe or file and plain text at a terminal; `-render-form
mermaid` asks for the diagram either way:

```bash
./bin/sysml examples/lord-demo/lord.sysml -render LordViews::townSquare \
  -render-form mermaid
```

```
stateDiagram-v2
  state "LordPlay::Warrior::day<br>«exhibit state»" as n0 {
    state "townSquare<br>«state»<br>initial" as n1
    state "forest<br>«state»" as n2
    state "healersHut<br>«state»" as n3
    state "bank<br>«state»" as n4
    state "trainingHall<br>«state»" as n5
    state "inn<br>«state»" as n6
    state "slain<br>«state»" as n7
    [*] --> n1
  }
  n1 --> n2 : townSquare_forest: accept EnterForest
  n1 --> n3 : townSquare_healer: accept VisitTheHealer / healing
  n1 --> n4 : townSquare_bank: accept VisitTheBank
  n1 --> n5 : townSquare_training: accept VisitTheTrainingHall / training
  n1 --> n6 : townSquare_inn: accept SleepAtTheInn
  n2 --> n2 : forest_fight: accept LookForSomethingToKill [forestFightsLeft #gt; 0 and alive] / fighting
  n2 --> n7 : forest_slain: [not alive]
  n2 --> n1 : forest_town: accept ReturnToTown
  n3 --> n1 : healer_town: accept ReturnToTown
  n4 --> n1 : bank_town: accept ReturnToTown
  n5 --> n1 : training_town: accept ReturnToTown
  n6 --> n1 : inn_midnight: accept NewDay / midnight
  n7 --> n1 : slain_midnight: accept NewDay / midnight
```

`forestFight` is the fight as a flowchart, with the merge the rounds loop
through and the three decisions, and `theTown` is the town as a tree:

```bash
./bin/sysml examples/lord-demo/lord.sysml -render LordViews::forestFight
./bin/sysml examples/lord-demo/lord.sysml -render LordViews::theTown
```

**The Daily Happenings.** `-render-document` compiles a document definition,
runs its queries and writes Markdown. With `-instantiate`, the `Verdicts`
query reads the hero the session holds — every constraint asserted of the
warrior and the `satisfy` asserted about it — and the price lists are
`Project` over each shop's parts, the item's `name` attribute as a `Column`:

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
| LordPlay::hero | constraint | hitPointsWithinMaximum | holds |  |
| LordPlay::hero | constraint | theDeadHaveNoHitPoints | holds |  |
| LordPlay::hero | constraint | noDebt | holds |  |
| LordPlay::hero | satisfaction |  | violated | satisfaction satisfy readyForTheDragon by hero: require condition evaluated to false: warrior.level == 12 |

*King Arthur's Weapons*

| tier | strength | price | weapon |
| --- | --- | --- | --- |
| 1 | 5 | 200 | Stick |
| 2 | 10 | 1000 | Dagger |
| 3 | 20 | 3000 | Short Sword |
| 4 | 30 | 10000 | Long Sword |
…
| 13 | 800 | 40000000 | Nira's Teeth |

*Abdul's Armour*

| tier | defense | price | armour |
| --- | --- | --- | --- |
| 1 | 1 | 200 | Coat |
…
| 13 | 400 | 40000000 | Belar's Mail |
```

## From the REPL

```bash
./bin/sysml
```

```
%load examples/lord-demo/lord.sysml
```

### A day in the realm

`%instantiate` creates the hero and starts the `day` machine the warrior
exhibits, in `townSquare`. `%state` binds the debugger to that running
machine; `%send` queues one of the menu's signals and `%step` dispatches it.
The transitions into the forest fight, the healer's hut, the training hall and
past midnight perform the warrior's actions as their effects.

```
%instantiate LordPlay::hero
%state LordPlay::hero
%send EnterForest
%step
%send LookForSomethingToKill
%step
%eval in LordPlay::hero : gold
%eval in LordPlay::hero : forestFightsLeft
```

```
✓ Sent LookForSomethingToKill to object #1 of "LordPlay::hero"
  Accepted by state machine "day" in state forest: transition forest_fight fires on it
✓ Event dispatched
  Current state: forest
  1 choice point; %trace on to see them
✓ gold (on LordPlay::hero ID: 1)
  = 534
✓ forestFightsLeft (on LordPlay::hero ID: 1)
  = 14
```

Back to town, sleep at the inn, and midnight gives the fights back:

```
%send ReturnToTown
%step
%send SleepAtTheInn
%step
%send NewDay
%step
%eval in LordPlay::hero : forestFightsLeft
```

```
✓ Sent NewDay to object #1 of "LordPlay::hero"
  Accepted by state machine "day" in state inn: transition inn_midnight fires on it
✓ Event dispatched
  Current state: townSquare
✓ forestFightsLeft (on LordPlay::hero ID: 1)
  = 15
```

A signal the current state does not accept is refused naming the state: in
`townSquare`, `%send LookForSomethingToKill` reports `accepts no signal
LookForSomethingToKill now: state machine "day" in state townSquare`.

### The town's deeds, one at a time

`%invoke` performs one of the warrior's actions on the object directly, with
its parameters named. The Tree Golem is the strongest thing a first-level
warrior meets; the healer charges two gold a point; Halder, the first master,
turns away a warrior short of a hundred experience; the bank pays ten percent
overnight; a shop refuses a sale the gold on hand does not cover.

```
%instantiate LordPlay::hero
%invoke LordPlay::hero fight foeHitPoints=13 foeStrength=9 foeDefense=3 foeGold=110 foeExperience=5
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
%invoke LordPlay::hero buyWeapon tier=2 weaponStrength=10 oldStrength=5 price=1000
%eval in LordPlay::hero : gold
%constraint LordPlay::hero::noDebt
```

```
✓ Invoked fight on object #1 of "LordPlay::hero"
✓ hitPoints (on LordPlay::hero ID: 1)
  = 2
✓ gold (on LordPlay::hero ID: 1)
  = 610
✓ Invoked heal on object #1 of "LordPlay::hero"
✓ hitPoints (on LordPlay::hero ID: 1)
  = 10
✓ gold (on LordPlay::hero ID: 1)
  = 594
✓ Invoked train on object #1 of "LordPlay::hero"
✓ level (on LordPlay::hero ID: 1)
  = 1
✓ Invoked deposit on object #1 of "LordPlay::hero"
✓ Invoked newDay on object #1 of "LordPlay::hero"
✓ bankGold (on LordPlay::hero ID: 1)
  = 330
✓ Invoked buyWeapon on object #1 of "LordPlay::hero"
✓ gold (on LordPlay::hero ID: 1)
  = 294
✓ Constraint LordPlay::hero::noDebt passed (on LordPlay::hero ID: 1)
```

The golem's blow of eight took the warrior to two hit points; the eight
points mended cost sixteen gold; the Dagger was not bought.

Every deed checks its own preconditions, so the warrior's invariants hold
whichever way it is reached — through the `day` machine's guarded transitions
or directly here — and whatever its parameters say. The forest turns away a
dead warrior or one whose fifteen fights are spent, so `forestFightsLeft`
never goes below zero; the healer and the masters turn away the dead, so the
slain keep no hit points until morning; the bank moves only gold the warrior
has, and pays interest but never charges it; a shop refuses a price below
zero and the forest a foe with negative stats or gold; and a master refuses a
twelfth-level warrior, so `train` never makes a level thirteen:

```
%instantiate LordPlay::champion
%invoke LordPlay::champion train
%eval in LordPlay::champion : level
%invoke LordPlay::hero deposit amount=1000
%eval in LordPlay::hero : gold
```

```
✓ Invoked train on object #3 of "LordPlay::champion"
✓ level (on LordPlay::champion ID: 3)
  = 12
✓ Invoked deposit on object #1 of "LordPlay::hero"
✓ gold (on LordPlay::hero ID: 1)
  = 294
```

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
best of both shops:

```
%optimize LordOdds::ToughEnough
```

```
✓ Analysis ToughEnough is optimized (z3, 9ms)
  minimize fewestHitPoints = `hitPoints`: 18201
  LordOdds::ToughEnough::defense = 600
  LordOdds::ToughEnough::hitPoints = 18201
  LordOdds::ToughEnough::roundsToKill = 14
  LordOdds::ToughEnough::strength = 1100
```

18201 hit points: no warrior in the game has them, which is why the game gives
its warriors skills and its dice a critical hit. The model states the brawl as
the rules alone write it, and the solver says what those rules leave out.

## What the model does not do

The dice are the schedule. The game rolls whether a swing lands; the model
leaves that decision open and lets the executor, the checker or a replayed
witness make it. Skills, the three character classes' special attacks, player
versus player, the flirting at the inn, gems and charm are declared where the
stats screen shows them but no action spends them.
