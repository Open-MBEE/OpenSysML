// Legend of the Red Dragon, the 1989 BBS door game: the realm, a warrior's day,
// what a solver can say about the dragon, and the views. README.md runs it.

// The realm: fighters, the two shops' stock, the masters, the forest, the town.
package Lord {
    private import ScalarValues::*;

    enum def CharacterClass {
        enum deathKnight;
        enum mysticalSkills;
        enum thievingSkills;
    }

    // Anyone who trades blows: a warrior, a forest monster, a master, the dragon.
    abstract part def Fighter {
        attribute name : String;
        attribute hitPoints : Integer;
        attribute strength : Integer;
        attribute defense : Integer;
    }

    part def Monster :> Fighter {
        attribute level : Integer;
        attribute gold : Integer;
        attribute experience : Integer;
    }

    // The one monster the game is named for; only a level-twelve warrior meets it.
    part def Dragon :> Monster {
        attribute :>> name = "The Red Dragon";
        attribute :>> level = 12;
        attribute :>> hitPoints = 15000;
        attribute :>> strength = 2000;
        attribute :>> defense = 0;
        attribute :>> gold = 0;
        attribute :>> experience = 0;
    }

    // A master of Turgon's hall: beating one is the only way up a level.
    part def Master :> Fighter {
        attribute teaches : Integer;
        attribute experienceRequired : Integer;
        attribute hitPointsTaught : Integer;
        attribute strengthTaught : Integer;
        attribute defenseTaught : Integer;
    }

    // A tier is the item's place in its shop's list; the game sells one at a time.
    part def Weapon {
        attribute name : String;
        attribute tier : Integer;
        attribute strength : Integer;
        attribute price : Integer;
    }

    part def Armour {
        attribute name : String;
        attribute tier : Integer;
        attribute defense : Integer;
        attribute price : Integer;
    }

    // King Arthur's Weapons, as the game lists them.
    part def WeaponShop {
        part stick : Weapon { attribute :>> name = "Stick"; attribute :>> tier = 1; attribute :>> strength = 5; attribute :>> price = 200; }
        part dagger : Weapon { attribute :>> name = "Dagger"; attribute :>> tier = 2; attribute :>> strength = 10; attribute :>> price = 1000; }
        part shortSword : Weapon { attribute :>> name = "Short Sword"; attribute :>> tier = 3; attribute :>> strength = 20; attribute :>> price = 3000; }
        part longSword : Weapon { attribute :>> name = "Long Sword"; attribute :>> tier = 4; attribute :>> strength = 30; attribute :>> price = 10000; }
        part hugeAxe : Weapon { attribute :>> name = "Huge Axe"; attribute :>> tier = 5; attribute :>> strength = 40; attribute :>> price = 30000; }
        part boneCruncher : Weapon { attribute :>> name = "Bone Cruncher"; attribute :>> tier = 6; attribute :>> strength = 60; attribute :>> price = 100000; }
        part twinSwords : Weapon { attribute :>> name = "Twin Swords"; attribute :>> tier = 7; attribute :>> strength = 80; attribute :>> price = 150000; }
        part powerAxe : Weapon { attribute :>> name = "Power Axe"; attribute :>> tier = 8; attribute :>> strength = 120; attribute :>> price = 200000; }
        part ablesSword : Weapon { attribute :>> name = "Able's Sword"; attribute :>> tier = 9; attribute :>> strength = 180; attribute :>> price = 400000; }
        part wansWeapon : Weapon { attribute :>> name = "Wan's Weapon"; attribute :>> tier = 10; attribute :>> strength = 250; attribute :>> price = 1000000; }
        part spearOfGold : Weapon { attribute :>> name = "Spear of Gold"; attribute :>> tier = 11; attribute :>> strength = 350; attribute :>> price = 4000000; }
        part crystalShard : Weapon { attribute :>> name = "Crystal Shard"; attribute :>> tier = 12; attribute :>> strength = 500; attribute :>> price = 10000000; }
        part nirasTeeth : Weapon { attribute :>> name = "Nira's Teeth"; attribute :>> tier = 13; attribute :>> strength = 800; attribute :>> price = 40000000; }
    }

    // Abdul's Armour, as the game lists it.
    part def ArmourShop {
        part coat : Armour { attribute :>> name = "Coat"; attribute :>> tier = 1; attribute :>> defense = 1; attribute :>> price = 200; }
        part heavyCoat : Armour { attribute :>> name = "Heavy Coat"; attribute :>> tier = 2; attribute :>> defense = 3; attribute :>> price = 1000; }
        part leatherVest : Armour { attribute :>> name = "Leather Vest"; attribute :>> tier = 3; attribute :>> defense = 10; attribute :>> price = 3000; }
        part bronzeArmour : Armour { attribute :>> name = "Bronze Armor"; attribute :>> tier = 4; attribute :>> defense = 15; attribute :>> price = 10000; }
        part ironArmour : Armour { attribute :>> name = "Iron Armor"; attribute :>> tier = 5; attribute :>> defense = 25; attribute :>> price = 30000; }
        part graphiteArmour : Armour { attribute :>> name = "Graphite Armor"; attribute :>> tier = 6; attribute :>> defense = 35; attribute :>> price = 100000; }
        part erdricksArmour : Armour { attribute :>> name = "Erdrick's Armor"; attribute :>> tier = 7; attribute :>> defense = 50; attribute :>> price = 150000; }
        part armourOfDeath : Armour { attribute :>> name = "Armor of Death"; attribute :>> tier = 8; attribute :>> defense = 75; attribute :>> price = 200000; }
        part ablesArmour : Armour { attribute :>> name = "Able's Armor"; attribute :>> tier = 9; attribute :>> defense = 100; attribute :>> price = 400000; }
        part fullBodyArmour : Armour { attribute :>> name = "Full Body Armor"; attribute :>> tier = 10; attribute :>> defense = 150; attribute :>> price = 1000000; }
        part bloodArmour : Armour { attribute :>> name = "Blood Armor"; attribute :>> tier = 11; attribute :>> defense = 225; attribute :>> price = 4000000; }
        part magicProtection : Armour { attribute :>> name = "Magic Protection"; attribute :>> tier = 12; attribute :>> defense = 300; attribute :>> price = 10000000; }
        part belarsMail : Armour { attribute :>> name = "Belar's Mail"; attribute :>> tier = 13; attribute :>> defense = 400; attribute :>> price = 40000000; }
    }

    // Turgon's Warrior Training: one master per level a warrior can still gain.
    part def TrainingHall {
        part halder : Master { attribute :>> name = "Halder"; attribute :>> teaches = 2; attribute :>> experienceRequired = 100; attribute :>> hitPoints = 15; attribute :>> strength = 10; attribute :>> defense = 2; attribute :>> hitPointsTaught = 10; attribute :>> strengthTaught = 5; attribute :>> defenseTaught = 2; }
        part barak : Master { attribute :>> name = "Barak"; attribute :>> teaches = 3; attribute :>> experienceRequired = 400; attribute :>> hitPoints = 25; attribute :>> strength = 15; attribute :>> defense = 4; attribute :>> hitPointsTaught = 15; attribute :>> strengthTaught = 8; attribute :>> defenseTaught = 3; }
        part aragorn : Master { attribute :>> name = "Aragorn"; attribute :>> teaches = 4; attribute :>> experienceRequired = 1000; attribute :>> hitPoints = 40; attribute :>> strength = 22; attribute :>> defense = 7; attribute :>> hitPointsTaught = 20; attribute :>> strengthTaught = 10; attribute :>> defenseTaught = 5; }
        part olodrin : Master { attribute :>> name = "Olodrin"; attribute :>> teaches = 5; attribute :>> experienceRequired = 4000; attribute :>> hitPoints = 60; attribute :>> strength = 32; attribute :>> defense = 10; attribute :>> hitPointsTaught = 25; attribute :>> strengthTaught = 12; attribute :>> defenseTaught = 6; }
        part sandtiger : Master { attribute :>> name = "Sandtiger"; attribute :>> teaches = 6; attribute :>> experienceRequired = 10000; attribute :>> hitPoints = 90; attribute :>> strength = 45; attribute :>> defense = 15; attribute :>> hitPointsTaught = 30; attribute :>> strengthTaught = 15; attribute :>> defenseTaught = 8; }
        part sparhawk : Master { attribute :>> name = "Sparhawk"; attribute :>> teaches = 7; attribute :>> experienceRequired = 40000; attribute :>> hitPoints = 130; attribute :>> strength = 60; attribute :>> defense = 20; attribute :>> hitPointsTaught = 40; attribute :>> strengthTaught = 20; attribute :>> defenseTaught = 10; }
        part atsukoSensei : Master { attribute :>> name = "Atsuko Sensei"; attribute :>> teaches = 8; attribute :>> experienceRequired = 100000; attribute :>> hitPoints = 180; attribute :>> strength = 80; attribute :>> defense = 27; attribute :>> hitPointsTaught = 50; attribute :>> strengthTaught = 25; attribute :>> defenseTaught = 12; }
        part aladdin : Master { attribute :>> name = "Aladdin"; attribute :>> teaches = 9; attribute :>> experienceRequired = 400000; attribute :>> hitPoints = 250; attribute :>> strength = 105; attribute :>> defense = 35; attribute :>> hitPointsTaught = 60; attribute :>> strengthTaught = 30; attribute :>> defenseTaught = 15; }
        part princeCaspian : Master { attribute :>> name = "Prince Caspian"; attribute :>> teaches = 10; attribute :>> experienceRequired = 1000000; attribute :>> hitPoints = 350; attribute :>> strength = 140; attribute :>> defense = 45; attribute :>> hitPointsTaught = 80; attribute :>> strengthTaught = 40; attribute :>> defenseTaught = 20; }
        part gandalf : Master { attribute :>> name = "Gandalf"; attribute :>> teaches = 11; attribute :>> experienceRequired = 4000000; attribute :>> hitPoints = 500; attribute :>> strength = 190; attribute :>> defense = 60; attribute :>> hitPointsTaught = 100; attribute :>> strengthTaught = 50; attribute :>> defenseTaught = 25; }
        part turgon : Master { attribute :>> name = "Turgon"; attribute :>> teaches = 12; attribute :>> experienceRequired = 10000000; attribute :>> hitPoints = 800; attribute :>> strength = 260; attribute :>> defense = 80; attribute :>> hitPointsTaught = 150; attribute :>> strengthTaught = 70; attribute :>> defenseTaught = 35; }
    }

    // What a new warrior meets: the forest's first-level monsters, and the dragon.
    part def Forest {
        part largeMosquito : Monster { attribute :>> name = "Large Mosquito"; attribute :>> level = 1; attribute :>> hitPoints = 5; attribute :>> strength = 3; attribute :>> defense = 0; attribute :>> gold = 34; attribute :>> experience = 1; }
        part smallThief : Monster { attribute :>> name = "Small Thief"; attribute :>> level = 1; attribute :>> hitPoints = 6; attribute :>> strength = 4; attribute :>> defense = 0; attribute :>> gold = 45; attribute :>> experience = 2; }
        part wildBoar : Monster { attribute :>> name = "Wild Boar"; attribute :>> level = 1; attribute :>> hitPoints = 9; attribute :>> strength = 6; attribute :>> defense = 1; attribute :>> gold = 60; attribute :>> experience = 3; }
        part rockMan : Monster { attribute :>> name = "Rock Man"; attribute :>> level = 1; attribute :>> hitPoints = 11; attribute :>> strength = 8; attribute :>> defense = 3; attribute :>> gold = 85; attribute :>> experience = 4; }
        part treeGolem : Monster { attribute :>> name = "Tree Golem"; attribute :>> level = 1; attribute :>> hitPoints = 13; attribute :>> strength = 9; attribute :>> defense = 3; attribute :>> gold = 110; attribute :>> experience = 5; }

        part redDragon : Dragon;
    }

    part def Bank {
        attribute dailyInterestPercent : Integer = 10;
    }

    part def HealersHut {
        attribute goldPerHitPoint : Integer = 2;
    }

    // The Inn: a room for the night keeps a warrior off the slaughter list.
    part def Inn {
        attribute roomPrice : Integer = 400;
    }

    // The town square, one part per place the game's menu leads to.
    part def Town {
        part bank : Bank;
        part healer : HealersHut;
        part inn : Inn;
        part weapons : WeaponShop;
        part armour : ArmourShop;
        part training : TrainingHall;
        part forest : Forest;
    }

    part town : Town;
}

// A warrior's day: the deeds each place offers, the menu as a state machine, midnight.
package LordPlay {
    private import ScalarValues::*;
    private import IntegerFunctions::max;
    private import RealFunctions::floor;
    private import Lord::*;

    // A blow lands for strength less defense, and always for at least one point.
    calc def Blow {
        in strength : Integer;
        in defense : Integer;

        max(1, strength - defense)
    }

    // What the bank adds overnight, in whole gold pieces.
    calc def Interest {
        in bankGold : Integer;
        in percent : Integer;

        floor(bankGold * percent / 100)
    }

    // The menu keys of the game, as the signals a warrior's day accepts.
    attribute def EnterForest;
    attribute def LookForSomethingToKill;
    attribute def ReturnToTown;
    attribute def VisitTheHealer;
    attribute def VisitTheBank;
    attribute def VisitTheTrainingHall;
    attribute def SleepAtTheInn;
    attribute def NewDay;

    // A warrior: the stats screen, the deeds the town offers, and the day itself.
    part def Warrior :> Fighter {
        attribute class : CharacterClass default = CharacterClass::deathKnight;
        attribute level : Integer default = 1;
        attribute experience : Integer default = 0;
        attribute :>> hitPoints default = 10;
        attribute maxHitPoints : Integer default = 10;
        attribute :>> strength default = 10;
        attribute :>> defense default = 1;
        attribute weaponTier : Integer default = 1;
        attribute armourTier : Integer default = 1;
        attribute gold : Integer default = 500;
        attribute bankGold : Integer default = 0;
        attribute gems : Integer default = 0;
        attribute charm : Integer default = 1;
        attribute forestFightsLeft : Integer default = 15;
        attribute playerFightsLeft : Integer default = 3;
        attribute alive : Boolean default = true;
        attribute dragonKills : Integer default = 0;

        // What the game never lets happen, asserted of every warrior.
        assert constraint levelInRange { level >= 1 and level <= 12 }
        assert constraint fightsNotOverdrawn { forestFightsLeft >= 0 and forestFightsLeft <= 15 }
        assert constraint hitPointsWithinMaximum { hitPoints <= maxHitPoints }
        assert constraint theDeadHaveNoHitPoints { alive or hitPoints == 0 }
        assert constraint noDebt { gold >= 0 and bankGold >= 0 }

        // One forest fight, against the Large Mosquito unless told otherwise.
        // Whether each swing lands is the game's dice roll, left to the schedule.
        action fight {
            in foeHitPoints : Integer = 5;
            in foeStrength : Integer = 3;
            in foeDefense : Integer = 0;
            in foeGold : Integer = 34;
            in foeExperience : Integer = 1;

            attribute foeLeft : Integer = 0;
            attribute rounds : Integer = 0;

            first start;

            action engage {
                assign foeLeft := foeHitPoints;
                assign forestFightsLeft := forestFightsLeft - 1;
            }

            merge nextRound;
            decide swing;

            action strike {
                assign foeLeft := foeLeft - Blow(strength, foeDefense);
                assign rounds := rounds + 1;
            }

            action miss {
                assign rounds := rounds + 1;
            }

            decide foeStanding;

            action riposte {
                assign hitPoints := max(0, hitPoints - Blow(foeStrength, defense));
            }

            decide warriorStanding;

            action win {
                assign gold := gold + foeGold;
                assign experience := experience + foeExperience;
            }

            // Death in the forest costs the gold on hand and the rest of the day.
            action die {
                assign alive := false;
                assign gold := 0;
            }

            succession first start then engage;
            succession first engage then nextRound;
            succession first nextRound then swing;
            first swing if foeLeft > 0 then strike;
            first swing if foeLeft > 0 then miss;
            succession first strike then foeStanding;
            first foeStanding if foeLeft <= 0 then win;
            first foeStanding if foeLeft > 0 then riposte;
            succession first miss then riposte;
            succession first riposte then warriorStanding;
            first warriorStanding if hitPoints > 0 then nextRound;
            first warriorStanding if hitPoints <= 0 then die;
            succession first win then done;
            succession first die then done;
        }

        // The healer mends every point, or as many as the gold on hand pays for.
        action heal {
            in goldPerHitPoint : Integer = 2;

            attribute affordable : Integer = 0;

            first start;

            action price {
                assign affordable := floor(gold / goldPerHitPoint);
            }

            decide canAffordAll;

            action mendAll {
                assign gold := gold - (maxHitPoints - hitPoints) * goldPerHitPoint;
                assign hitPoints := maxHitPoints;
            }

            action mendSome {
                assign hitPoints := hitPoints + affordable;
                assign gold := gold - affordable * goldPerHitPoint;
            }

            succession first start then price;
            succession first price then canAffordAll;
            first canAffordAll if affordable >= maxHitPoints - hitPoints then mendAll;
            first canAffordAll if affordable < maxHitPoints - hitPoints then mendSome;
            succession first mendAll then done;
            succession first mendSome then done;
        }

        // Gold in the bank earns interest and cannot be taken by another player.
        action deposit {
            in amount : Integer;

            first start;
            then action move {
                assign bankGold := bankGold + amount;
                assign gold := gold - amount;
            }
            then done;
        }

        action withdraw {
            in amount : Integer;

            first start;
            then action move {
                assign bankGold := bankGold - amount;
                assign gold := gold + amount;
            }
            then done;
        }

        // A shop sells for the full price, the new item's stat replacing the old one's.
        action buyWeapon {
            in tier : Integer;
            in weaponStrength : Integer;
            in oldStrength : Integer;
            in price : Integer;

            first start;
            then decide canPay;
            if gold >= price then pay;
            else done;

            action pay {
                assign gold := gold - price;
                assign strength := strength - oldStrength + weaponStrength;
                assign weaponTier := tier;
            }
            then done;
        }

        action buyArmour {
            in tier : Integer;
            in armourDefense : Integer;
            in oldDefense : Integer;
            in price : Integer;

            first start;
            then decide canPay;
            if gold >= price then pay;
            else done;

            action pay {
                assign gold := gold - price;
                assign defense := defense - oldDefense + armourDefense;
                assign armourTier := tier;
            }
            then done;
        }

        // A bout at the hall, against Halder unless told otherwise; a win is the
        // next level and its stats, a loss costs only the bruises.
        action train {
            in masterHitPoints : Integer = 15;
            in masterStrength : Integer = 10;
            in masterDefense : Integer = 2;
            in experienceRequired : Integer = 100;
            in hitPointsTaught : Integer = 10;
            in strengthTaught : Integer = 5;
            in defenseTaught : Integer = 2;

            attribute masterLeft : Integer = 0;
            attribute promoted : Boolean = false;

            first start;
            then decide ready;
            if experience >= experienceRequired then bout;
            else turnedAway;

            action bout {
                assign masterLeft := masterHitPoints;
                loop {
                    assign masterLeft := masterLeft - Blow(strength, masterDefense);
                    assign hitPoints := if masterLeft > 0 ? max(1, hitPoints - Blow(masterStrength, defense)) else hitPoints;
                } until masterLeft <= 0 or hitPoints <= 1;
            }

            action turnedAway;

            decide verdict;

            action levelUp {
                assign level := level + 1;
                assign maxHitPoints := maxHitPoints + hitPointsTaught;
                assign hitPoints := hitPoints + hitPointsTaught;
                assign strength := strength + strengthTaught;
                assign defense := defense + defenseTaught;
                assign promoted := true;
            }

            action beaten;

            succession first bout then verdict;
            first verdict if masterLeft <= 0 then levelUp;
            first verdict if masterLeft > 0 then beaten;
            succession first levelUp then done;
            succession first beaten then done;
            succession first turnedAway then done;
        }

        // Midnight: the fights come back, the bank pays, the slain rise.
        action newDay {
            in interestPercent : Integer = 10;

            first start;
            then action reset {
                assign forestFightsLeft := 15;
                assign playerFightsLeft := 3;
                assign bankGold := bankGold + Interest(bankGold, interestPercent);
                assign hitPoints := if alive ? hitPoints else maxHitPoints;
                assign alive := true;
            }
            then done;
        }

        // The game's menu: the town square, the places its keys lead to, the
        // forest, and the grave a slain warrior waits in until the next day.
        exhibit state day {
            entry; then townSquare;

            state townSquare;

            state forest;

            state healersHut;

            state bank;

            state trainingHall;

            // Asleep at the Inn, a warrior waits for midnight.
            state inn;

            state slain;

            transition townSquare_forest
                first townSquare
                accept EnterForest
                then forest;

            // A fight is offered while the day has fights left; the warrior
            // stays in the forest afterwards, or goes to the grave if killed.
            transition forest_fight
                first forest
                accept LookForSomethingToKill
                if forestFightsLeft > 0 and alive
                do action fighting { perform fight; }
                then forest;

            transition forest_slain
                first forest
                if not alive
                then slain;

            transition forest_town
                first forest
                accept ReturnToTown
                then townSquare;

            transition townSquare_healer
                first townSquare
                accept VisitTheHealer
                do action healing { perform heal; }
                then healersHut;

            transition healer_town
                first healersHut
                accept ReturnToTown
                then townSquare;

            transition townSquare_bank
                first townSquare
                accept VisitTheBank
                then bank;

            transition bank_town
                first bank
                accept ReturnToTown
                then townSquare;

            transition townSquare_training
                first townSquare
                accept VisitTheTrainingHall
                do action training { perform train; }
                then trainingHall;

            transition training_town
                first trainingHall
                accept ReturnToTown
                then townSquare;

            transition townSquare_inn
                first townSquare
                accept SleepAtTheInn
                then inn;

            transition inn_midnight
                first inn
                accept NewDay
                do action midnight { perform newDay; }
                then townSquare;

            transition slain_midnight
                first slain
                accept NewDay
                do action midnight { perform newDay; }
                then townSquare;
        }
    }

    part hero : Warrior {
        attribute :>> name = "Sir Devin";
    }

    // A warrior at the top of the ladder, kitted out from both shops.
    part champion : Warrior {
        attribute :>> name = "Turgon's Equal";
        attribute :>> level = 12;
        attribute :>> experience = 10000000;
        attribute :>> hitPoints = 1200;
        attribute :>> maxHitPoints = 1200;
        attribute :>> strength = 1100;
        attribute :>> defense = 600;
        attribute :>> weaponTier = 13;
        attribute :>> armourTier = 13;
    }
}

// What holds of a warrior, and what a solver can say about the dragon and the shops.
package LordOdds {
    private import ScalarValues::*;
    private import RealFunctions::floor;
    private import TradeStudies::*;
    private import LordPlay::*;

    // Blows needed to take `targetHitPoints` off, landing every one.
    calc def BlowsToKill {
        in strength : Integer;
        in defense : Integer;
        in targetHitPoints : Integer;

        floor((targetHitPoints + Blow(strength, defense) - 1) / Blow(strength, defense))
    }

    // Only a level-twelve warrior with the best of both shops is let near the dragon.
    requirement def ReadyForTheDragon {
        subject warrior : Warrior;

        require constraint { warrior.level == 12 }
        require constraint { warrior.weaponTier == 13 }
        require constraint { warrior.armourTier == 13 }
    }

    // A straight brawl, every blow landing and the warrior swinging first: the
    // warrior must survive one blow fewer than it takes to fell the dragon.
    requirement def OutlastTheDragon {
        subject warrior : Warrior;
        attribute dragonHitPoints : Integer = 15000;
        attribute dragonStrength : Integer = 2000;

        require constraint {
            warrior.hitPoints + Blow(dragonStrength, warrior.defense)
                > Blow(dragonStrength, warrior.defense) * BlowsToKill(warrior.strength, 0, dragonHitPoints)
        }
    }

    requirement readyForTheDragon : ReadyForTheDragon;
    requirement outlastTheDragon : OutlastTheDragon;

    part dragonFights {
        assert satisfy readyForTheDragon by champion;
        assert satisfy readyForTheDragon by hero;
        assert satisfy outlastTheDragon by champion;
    }

    // The fewest hit points that win the brawl with the best of both shops.
    analysis def ToughEnough {
        attribute hitPoints : Integer;
        attribute strength : Integer;
        attribute defense : Integer;
        attribute roundsToKill : Integer;

        objective fewestHitPoints : MinimizeObjective {
            subject :>> selectedAlternative;
            require constraint { strength == 1100 }
            require constraint { defense == 600 }
            require constraint { hitPoints >= 1 }
            require constraint { roundsToKill >= 1 }
            require constraint { strength * roundsToKill >= 15000 }
            require constraint { strength * (roundsToKill - 1) < 15000 }
            require constraint { hitPoints + (2000 - defense) > (2000 - defense) * roundsToKill }

            in calc :>> eval { hitPoints }
        }
    }

    // A first day: the starting purse plus fifteen fights of forest gold, and a weapon from it.
    requirement def FirstDayShopping {
        attribute startingGold : Integer;
        attribute forestGold : Integer;
        attribute weaponPrice : Integer;

        require constraint { startingGold == 500 }
        require constraint { forestGold >= 0 }
        require constraint { forestGold <= 15 * 110 }
        require constraint { weaponPrice <= startingGold + forestGold }
        require constraint { weaponPrice >= 1000 }
    }

    // The same day, asking for a Long Sword.
    requirement def FirstDayLongSword :> FirstDayShopping {
        require constraint { weaponPrice >= 10000 }
    }
}

// How the game is shown: the menu as a state diagram, a fight as a flow, the town
// as a tree, and the news screen as a document with the shops' price lists.
package LordViews {
    private import Views::*;
    private import StandardViewDefinitions::*;
    private import DocumentQueries::*;
    private import KerML::Root::Element;
    private import ScalarValues::String;
    private import LordPlay::*;

    view townSquare : StateTransitionView {
        expose LordPlay::Warrior::day;
    }

    view forestFight : ActionFlowView {
        expose LordPlay::Warrior::fight;
    }

    view theTown {
        expose Lord::Town;
        render asTreeDiagram;
    }

    // The document's query roots: a table's query is bound to a usage.
    part kingArthurs : Lord::WeaponShop;
    part abduls : Lord::ArmourShop;

    // A shop's items, one row each, as its definition declares them.
    calc def Stock :> Query {
        in shop : Element;
        WhereType(
            source = OwnedElements(
                source = RelatedElements(
                    source = shop,
                    relationshipKind = "typing",
                    direction = "outgoing",
                    maxDepth = 1
                )
            ),
            type = "PartUsage"
        )
    }

    calc def WeaponList :> Query {
        in shop : Element;
        Project(
            source = Stock(shop = shop),
            properties = ("tier", "strength", "price"),
            columns = (Column(name = "weapon", expression = Lord::Weapon::name))
        )
    }

    calc def ArmourList :> Query {
        in shop : Element;
        Project(
            source = Stock(shop = shop),
            properties = ("tier", "defense", "price"),
            columns = (Column(name = "armour", expression = Lord::Armour::name))
        )
    }

    // Every assertion about the warrior bound to `root`.
    calc def Standing :> Query {
        in root : Element;
        Project(
            source = Verdicts(source = root),
            properties = ("path", "kind", "name", "verdict", "reason")
        )
    }

    // The Daily Happenings: the hero's standing as the session holds it, and the shops.
    part def DailyHappenings :> Document {
        attribute redefines title = "The Daily Happenings";

        part intro : Paragraph {
            attribute redefines text = "What holds of the warrior, at the town square.";
        }
        part standing : Table {
            attribute redefines caption = "The warrior's standing";
            calc rows : Standing {
                in root = hero;
            }
        }
        part weapons : Table {
            attribute redefines caption = "King Arthur's Weapons";
            calc rows : WeaponList {
                in shop = kingArthurs;
            }
        }
        part armour : Table {
            attribute redefines caption = "Abdul's Armour";
            calc rows : ArmourList {
                in shop = abduls;
            }
        }
    }
}
