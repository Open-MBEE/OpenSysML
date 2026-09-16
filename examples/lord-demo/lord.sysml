// Legend of the Red Dragon, the 1989 BBS door game: the realm, a warrior's day,
// what a solver can say about the dragon, and the views. README.md runs it.

// The realm: fighters, the shops' stock, the masters, the forest with its twelve
// levels of monsters and the dragon, the inn and its locals, the tavern, the town.
package Lord {
    private import ScalarValues::*;

    // The three skill trees; a warrior studies one at a time and keeps all three.
    enum def CharacterClass {
        enum deathKnight;
        enum mysticalSkills;
        enum thievingSkills;
    }

    enum def Sex {
        enum male;
        enum female;
    }

    // Whom a warrior is married to; the barmaid takes grooms, the bard brides.
    enum def Spouse {
        enum nobody;
        enum violet;
        enum sethAble;
    }

    // What a warrior does with a round of combat: swing, or spend a skill.
    enum def Move {
        enum attack;
        enum deathKnight;
        enum thieving;
        enum pinchRealHard;
        enum disappear;
        enum heatWave;
        enum lightShield;
        enum shatter;
        enum mindHeal;
    }

    // The stat the bartender raises for two gems.
    enum def Stat {
        enum strength;
        enum defense;
        enum hitPoints;
    }

    // What a warrior may ask the fairies for.
    enum def Blessing {
        enum kiss;
        enum horse;
        enum sadStory;
        enum lore;
    }

    // Anyone who trades blows: a warrior, a forest monster, a master, the dragon.
    abstract part def Fighter {
        attribute name : String;
        attribute hitPoints : Integer;
        attribute strength : Integer;
        attribute defense : Integer;
    }

    // A forest monster as the game's enemy file lists it; monsters have no armour.
    part def Monster :> Fighter {
        attribute weapon : String default = "";
        attribute level : Integer;
        attribute gold : Integer;
        attribute experience : Integer;
        attribute :>> defense default = 0;
        attribute deadly : Boolean default = false;
    }

    // The one monster the game is named for; only a level-twelve warrior meets it.
    // Its breath burns for a thousand whatever the armour; its other three attacks strike.
    part def Dragon :> Monster {
        attribute :>> name = "The Red Dragon";
        attribute :>> weapon = "Flaming Breath";
        attribute :>> level = 12;
        attribute :>> hitPoints = 15000;
        attribute :>> strength = 2000;
        attribute :>> gold = 0;
        attribute :>> experience = 0;
        attribute breath : Integer = 1000;
    }

    // A master of Turgon's hall: beating one is the only way up a level.
    part def Master :> Fighter {
        attribute weapon : String;
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
        part halder : Master { attribute :>> name = "Halder"; attribute :>> weapon = "Short Sword"; attribute :>> teaches = 2; attribute :>> experienceRequired = 100; attribute :>> hitPoints = 15; attribute :>> strength = 10; attribute :>> defense = 2; attribute :>> hitPointsTaught = 10; attribute :>> strengthTaught = 5; attribute :>> defenseTaught = 2; }
        part barak : Master { attribute :>> name = "Barak"; attribute :>> weapon = "Battle Axe"; attribute :>> teaches = 3; attribute :>> experienceRequired = 400; attribute :>> hitPoints = 25; attribute :>> strength = 15; attribute :>> defense = 4; attribute :>> hitPointsTaught = 15; attribute :>> strengthTaught = 8; attribute :>> defenseTaught = 3; }
        part aragorn : Master { attribute :>> name = "Aragorn"; attribute :>> weapon = "Twin Swords"; attribute :>> teaches = 4; attribute :>> experienceRequired = 1000; attribute :>> hitPoints = 40; attribute :>> strength = 22; attribute :>> defense = 7; attribute :>> hitPointsTaught = 20; attribute :>> strengthTaught = 10; attribute :>> defenseTaught = 5; }
        part olodrin : Master { attribute :>> name = "Olodrin"; attribute :>> weapon = "Power Axe"; attribute :>> teaches = 5; attribute :>> experienceRequired = 4000; attribute :>> hitPoints = 60; attribute :>> strength = 32; attribute :>> defense = 10; attribute :>> hitPointsTaught = 25; attribute :>> strengthTaught = 12; attribute :>> defenseTaught = 6; }
        part sandtiger : Master { attribute :>> name = "Sandtiger"; attribute :>> weapon = "Blessed Sword"; attribute :>> teaches = 6; attribute :>> experienceRequired = 10000; attribute :>> hitPoints = 90; attribute :>> strength = 45; attribute :>> defense = 15; attribute :>> hitPointsTaught = 30; attribute :>> strengthTaught = 15; attribute :>> defenseTaught = 8; }
        part sparhawk : Master { attribute :>> name = "Sparhawk"; attribute :>> weapon = "Double Bladed Sword"; attribute :>> teaches = 7; attribute :>> experienceRequired = 40000; attribute :>> hitPoints = 130; attribute :>> strength = 60; attribute :>> defense = 20; attribute :>> hitPointsTaught = 40; attribute :>> strengthTaught = 20; attribute :>> defenseTaught = 10; }
        part atsukoSensei : Master { attribute :>> name = "Atsuko Sensei"; attribute :>> weapon = "Huge Curved Blade"; attribute :>> teaches = 8; attribute :>> experienceRequired = 100000; attribute :>> hitPoints = 180; attribute :>> strength = 80; attribute :>> defense = 27; attribute :>> hitPointsTaught = 50; attribute :>> strengthTaught = 25; attribute :>> defenseTaught = 12; }
        part aladdin : Master { attribute :>> name = "Aladdin"; attribute :>> weapon = "Shiny Lamp"; attribute :>> teaches = 9; attribute :>> experienceRequired = 400000; attribute :>> hitPoints = 250; attribute :>> strength = 105; attribute :>> defense = 35; attribute :>> hitPointsTaught = 60; attribute :>> strengthTaught = 30; attribute :>> defenseTaught = 15; }
        part princeCaspian : Master { attribute :>> name = "Prince Caspian"; attribute :>> weapon = "Flashing Rapier"; attribute :>> teaches = 10; attribute :>> experienceRequired = 1000000; attribute :>> hitPoints = 350; attribute :>> strength = 140; attribute :>> defense = 45; attribute :>> hitPointsTaught = 80; attribute :>> strengthTaught = 40; attribute :>> defenseTaught = 20; }
        part gandalf : Master { attribute :>> name = "Gandalf"; attribute :>> weapon = "Huge Fireballs"; attribute :>> teaches = 11; attribute :>> experienceRequired = 4000000; attribute :>> hitPoints = 500; attribute :>> strength = 190; attribute :>> defense = 60; attribute :>> hitPointsTaught = 100; attribute :>> strengthTaught = 50; attribute :>> defenseTaught = 25; }
        part turgon : Master { attribute :>> name = "Turgon"; attribute :>> weapon = "Able's Sword"; attribute :>> teaches = 12; attribute :>> experienceRequired = 10000000; attribute :>> hitPoints = 800; attribute :>> strength = 260; attribute :>> defense = 80; attribute :>> hitPointsTaught = 150; attribute :>> strengthTaught = 70; attribute :>> defenseTaught = 35; }
    }

    // One level of the forest: the eleven monsters a warrior of that level meets.
    part def ForestLevel {
        attribute level : Integer;
    }

    // Chance's tavern, hidden in the forest: change your profession, gamble with the old man.
    part def DarkCloakTavern {
        attribute gemsForTheRecords : Integer = 2;
        attribute oddsInTheHousesFavour : Integer = 49;
    }

    // Teesha and Nolan: a blessing for the asking, or a fairy for the catching.
    part def FairyGlade {
        attribute loreExperiencePerLevel : Integer = 10;
    }

    // The forest: twelve levels of monsters as the game's enemy file lists them
    // (strength, hit points, gold, experience; the deadliest of each level marked),
    // the Red Dragon, and the places a warrior stumbles on between fights.
    part def Forest {
        part level1 : ForestLevel {
            attribute :>> level = 1;
            part smallThief : Monster { attribute :>> name = "Small Thief"; attribute :>> weapon = "Small Dagger"; attribute :>> level = 1; attribute :>> hitPoints = 9; attribute :>> strength = 6; attribute :>> gold = 56; attribute :>> experience = 2; }
            part rudeBoy : Monster { attribute :>> name = "Rude Boy"; attribute :>> weapon = "Cudgel"; attribute :>> level = 1; attribute :>> hitPoints = 7; attribute :>> strength = 3; attribute :>> gold = 7; attribute :>> experience = 3; }
            part oldMan : Monster { attribute :>> name = "Old Man"; attribute :>> weapon = "Cane"; attribute :>> level = 1; attribute :>> hitPoints = 13; attribute :>> strength = 5; attribute :>> gold = 73; attribute :>> experience = 4; }
            part largeGreenRat : Monster { attribute :>> name = "Large Green Rat"; attribute :>> weapon = "Sharp Teeth"; attribute :>> level = 1; attribute :>> hitPoints = 4; attribute :>> strength = 3; attribute :>> gold = 32; attribute :>> experience = 1; }
            part wildBoar : Monster { attribute :>> name = "Wild Boar"; attribute :>> weapon = "Sharp Tusks"; attribute :>> level = 1; attribute :>> hitPoints = 9; attribute :>> strength = 10; attribute :>> gold = 58; attribute :>> experience = 5; }
            part uglyOldHag : Monster { attribute :>> name = "Ugly Old Hag"; attribute :>> weapon = "Garlic Breath"; attribute :>> level = 1; attribute :>> hitPoints = 9; attribute :>> strength = 6; attribute :>> gold = 109; attribute :>> experience = 4; }
            part largeMosquito : Monster { attribute :>> name = "Large Mosquito"; attribute :>> weapon = "Blood Sucker"; attribute :>> level = 1; attribute :>> hitPoints = 3; attribute :>> strength = 2; attribute :>> gold = 46; attribute :>> experience = 2; }
            part branTheWarrior : Monster { attribute :>> name = "Bran The Warrior"; attribute :>> weapon = "Short Sword"; attribute :>> level = 1; attribute :>> hitPoints = 15; attribute :>> strength = 12; attribute :>> gold = 234; attribute :>> experience = 10; attribute :>> deadly = true; }
            part evilWretch : Monster { attribute :>> name = "Evil Wretch"; attribute :>> weapon = "Finger Nail"; attribute :>> level = 1; attribute :>> hitPoints = 12; attribute :>> strength = 7; attribute :>> gold = 76; attribute :>> experience = 3; }
            part smallBear : Monster { attribute :>> name = "Small Bear"; attribute :>> weapon = "Claws"; attribute :>> level = 1; attribute :>> hitPoints = 7; attribute :>> strength = 9; attribute :>> gold = 154; attribute :>> experience = 6; }
            part smallTroll : Monster { attribute :>> name = "Small Troll"; attribute :>> weapon = "Uglyness"; attribute :>> level = 1; attribute :>> hitPoints = 14; attribute :>> strength = 6; attribute :>> gold = 87; attribute :>> experience = 5; }
        }
        part level2 : ForestLevel {
            attribute :>> level = 2;
            part greenPython : Monster { attribute :>> name = "Green Python"; attribute :>> weapon = "Dripping Fangs"; attribute :>> level = 2; attribute :>> hitPoints = 17; attribute :>> strength = 13; attribute :>> gold = 80; attribute :>> experience = 6; }
            part gathTheBarbarian : Monster { attribute :>> name = "Gath The Barbarian"; attribute :>> weapon = "Huge Spiked Club"; attribute :>> level = 2; attribute :>> hitPoints = 13; attribute :>> strength = 12; attribute :>> gold = 134; attribute :>> experience = 9; }
            part evilWoodNymph : Monster { attribute :>> name = "Evil Wood Nymph"; attribute :>> weapon = "Flirtatios Behavior"; attribute :>> level = 2; attribute :>> hitPoints = 10; attribute :>> strength = 15; attribute :>> gold = 160; attribute :>> experience = 11; }
            part fedrickTheLimpingBaboon : Monster { attribute :>> name = "Fedrick The Limping Baboon"; attribute :>> weapon = "Scary Faces"; attribute :>> level = 2; attribute :>> hitPoints = 23; attribute :>> strength = 8; attribute :>> gold = 97; attribute :>> experience = 6; }
            part wildMan : Monster { attribute :>> name = "Wild Man"; attribute :>> weapon = "Hands"; attribute :>> level = 2; attribute :>> hitPoints = 14; attribute :>> strength = 13; attribute :>> gold = 134; attribute :>> experience = 8; }
            part brorandiaTheViking : Monster { attribute :>> name = "Brorandia The Viking"; attribute :>> weapon = "Hugely Spiked Mace"; attribute :>> level = 2; attribute :>> hitPoints = 18; attribute :>> strength = 21; attribute :>> gold = 330; attribute :>> experience = 20; attribute :>> deadly = true; }
            part hugeBaldMan : Monster { attribute :>> name = "Huge Bald Man"; attribute :>> weapon = "Glare From Forehead"; attribute :>> level = 2; attribute :>> hitPoints = 19; attribute :>> strength = 19; attribute :>> gold = 311; attribute :>> experience = 16; }
            part senileSeniorCitizen : Monster { attribute :>> name = "Senile Senior Citizen"; attribute :>> weapon = "Crazy Ravings"; attribute :>> level = 2; attribute :>> hitPoints = 11; attribute :>> strength = 13; attribute :>> gold = 270; attribute :>> experience = 13; }
            part membrainMan : Monster { attribute :>> name = "Membrain Man"; attribute :>> weapon = "Strange Ooze"; attribute :>> level = 2; attribute :>> hitPoints = 16; attribute :>> strength = 10; attribute :>> gold = 190; attribute :>> experience = 11; }
            part bentRiverDryad : Monster { attribute :>> name = "Bent River Dryad"; attribute :>> weapon = "Pouring Waterfall"; attribute :>> level = 2; attribute :>> hitPoints = 16; attribute :>> strength = 12; attribute :>> gold = 150; attribute :>> experience = 9; }
            part rockMan : Monster { attribute :>> name = "Rock Man"; attribute :>> weapon = "Large Stones"; attribute :>> level = 2; attribute :>> hitPoints = 27; attribute :>> strength = 8; attribute :>> gold = 300; attribute :>> experience = 12; }
        }
        part level3 : ForestLevel {
            attribute :>> level = 3;
            part lazyBum : Monster { attribute :>> name = "Lazy Bum"; attribute :>> weapon = "Unwashed Body Odor"; attribute :>> level = 3; attribute :>> hitPoints = 29; attribute :>> strength = 19; attribute :>> gold = 380; attribute :>> experience = 18; }
            part twoHeadedRotwieler : Monster { attribute :>> name = "Two Headed Rotwieler"; attribute :>> weapon = "Twin Barking"; attribute :>> level = 3; attribute :>> hitPoints = 32; attribute :>> strength = 18; attribute :>> gold = 384; attribute :>> experience = 17; }
            part purpleMonchichi : Monster { attribute :>> name = "Purple Monchichi"; attribute :>> weapon = "Continous Whining"; attribute :>> level = 3; attribute :>> hitPoints = 29; attribute :>> strength = 14; attribute :>> gold = 763; attribute :>> experience = 23; }
            part bone : Monster { attribute :>> name = "Bone"; attribute :>> weapon = "Terrible Smoke Smell"; attribute :>> level = 3; attribute :>> hitPoints = 11; attribute :>> strength = 27; attribute :>> gold = 432; attribute :>> experience = 16; }
            part redNeck : Monster { attribute :>> name = "Red Neck"; attribute :>> weapon = "Awfull Country Slang"; attribute :>> level = 3; attribute :>> hitPoints = 16; attribute :>> strength = 19; attribute :>> gold = 563; attribute :>> experience = 19; }
            part wingedDemonOfDeath : Monster { attribute :>> name = "Winged Demon Of Death"; attribute :>> weapon = "Red Glare"; attribute :>> level = 3; attribute :>> hitPoints = 23; attribute :>> strength = 42; attribute :>> gold = 830; attribute :>> experience = 28; attribute :>> deadly = true; }
            part blackOwl : Monster { attribute :>> name = "Black Owl"; attribute :>> weapon = "Hooked Beak"; attribute :>> level = 3; attribute :>> hitPoints = 29; attribute :>> strength = 28; attribute :>> gold = 711; attribute :>> experience = 26; }
            part muscledMidget : Monster { attribute :>> name = "Muscled Midget"; attribute :>> weapon = "Low Punch"; attribute :>> level = 3; attribute :>> hitPoints = 19; attribute :>> strength = 26; attribute :>> gold = 870; attribute :>> experience = 32; }
            part headbangerOfTheWest : Monster { attribute :>> name = "Headbanger Of The West"; attribute :>> weapon = "Ear Shattering Noises"; attribute :>> level = 3; attribute :>> hitPoints = 27; attribute :>> strength = 23; attribute :>> gold = 245; attribute :>> experience = 43; }
            part morbidWalker : Monster { attribute :>> name = "Morbid Walker"; attribute :>> weapon = "Endless Walking"; attribute :>> level = 3; attribute :>> hitPoints = 10; attribute :>> strength = 28; attribute :>> gold = 764; attribute :>> experience = 9; }
            part magicalEvilGnome : Monster { attribute :>> name = "Magical Evil Gnome"; attribute :>> weapon = "Spell Of Fire"; attribute :>> level = 3; attribute :>> hitPoints = 25; attribute :>> strength = 24; attribute :>> gold = 638; attribute :>> experience = 28; }
        }
        part level4 : ForestLevel {
            attribute :>> level = 4;
            part deathDog : Monster { attribute :>> name = "Death Dog"; attribute :>> weapon = "Teeth"; attribute :>> level = 4; attribute :>> hitPoints = 52; attribute :>> strength = 36; attribute :>> gold = 1150; attribute :>> experience = 36; }
            part weakOrc : Monster { attribute :>> name = "Weak Orc"; attribute :>> weapon = "Spiked Club"; attribute :>> level = 4; attribute :>> hitPoints = 32; attribute :>> strength = 27; attribute :>> gold = 900; attribute :>> experience = 25; }
            part darkElf : Monster { attribute :>> name = "Dark Elf"; attribute :>> weapon = "Small bow"; attribute :>> level = 4; attribute :>> hitPoints = 57; attribute :>> strength = 43; attribute :>> gold = 1070; attribute :>> experience = 33; }
            part evilHobbit : Monster { attribute :>> name = "Evil Hobbit"; attribute :>> weapon = "Smoking Pipe"; attribute :>> level = 4; attribute :>> hitPoints = 95; attribute :>> strength = 35; attribute :>> gold = 1240; attribute :>> experience = 46; }
            part shortGoblin : Monster { attribute :>> name = "Short Goblin"; attribute :>> weapon = "Short Sword"; attribute :>> level = 4; attribute :>> hitPoints = 45; attribute :>> strength = 34; attribute :>> gold = 768; attribute :>> experience = 24; }
            part hugeBlackBear : Monster { attribute :>> name = "Huge Black Bear"; attribute :>> weapon = "Razor Claws"; attribute :>> level = 4; attribute :>> hitPoints = 48; attribute :>> strength = 67; attribute :>> gold = 1765; attribute :>> experience = 76; }
            part rabidWolf : Monster { attribute :>> name = "Rabid Wolf"; attribute :>> weapon = "Deathlock Fangs"; attribute :>> level = 4; attribute :>> hitPoints = 39; attribute :>> strength = 45; attribute :>> gold = 1400; attribute :>> experience = 43; }
            part youngWizard : Monster { attribute :>> name = "Young Wizard"; attribute :>> weapon = "Weak Magic"; attribute :>> level = 4; attribute :>> hitPoints = 35; attribute :>> strength = 64; attribute :>> gold = 1754; attribute :>> experience = 64; }
            part mudMan : Monster { attribute :>> name = "Mud Man"; attribute :>> weapon = "Mud Balls"; attribute :>> level = 4; attribute :>> hitPoints = 65; attribute :>> strength = 56; attribute :>> gold = 870; attribute :>> experience = 43; }
            part deathJester : Monster { attribute :>> name = "Death Jester"; attribute :>> weapon = "Horrible Jokes"; attribute :>> level = 4; attribute :>> hitPoints = 46; attribute :>> strength = 34; attribute :>> gold = 1343; attribute :>> experience = 32; }
            part rockMan : Monster { attribute :>> name = "Rock Man"; attribute :>> weapon = "Large Stones"; attribute :>> level = 4; attribute :>> hitPoints = 54; attribute :>> strength = 87; attribute :>> gold = 1754; attribute :>> experience = 76; attribute :>> deadly = true; }
        }
        part level5 : ForestLevel {
            attribute :>> level = 5;
            part pandionKnight : Monster { attribute :>> name = "Pandion Knight"; attribute :>> weapon = "Orkos Broadsword"; attribute :>> level = 5; attribute :>> hitPoints = 59; attribute :>> strength = 64; attribute :>> gold = 3100; attribute :>> experience = 98; }
            part jabba : Monster { attribute :>> name = "Jabba"; attribute :>> weapon = "Whiplashing Tail"; attribute :>> level = 5; attribute :>> hitPoints = 198; attribute :>> strength = 61; attribute :>> gold = 2384; attribute :>> experience = 137; }
            part manokenSloth : Monster { attribute :>> name = "Manoken Sloth"; attribute :>> weapon = "Dripping Paws"; attribute :>> level = 5; attribute :>> hitPoints = 69; attribute :>> strength = 54; attribute :>> gold = 2452; attribute :>> experience = 97; }
            part trojanWarrior : Monster { attribute :>> name = "Trojan Warrior"; attribute :>> weapon = "Twin Swords"; attribute :>> level = 5; attribute :>> hitPoints = 87; attribute :>> strength = 73; attribute :>> gold = 3432; attribute :>> experience = 154; }
            part misfitTheUgly : Monster { attribute :>> name = "Misfit The Ugly"; attribute :>> weapon = "Strange Ideas"; attribute :>> level = 5; attribute :>> hitPoints = 89; attribute :>> strength = 75; attribute :>> gold = 2563; attribute :>> experience = 120; }
            part georgeOfTheJungle : Monster { attribute :>> name = "George Of The Jungle"; attribute :>> weapon = "Echoing Screams"; attribute :>> level = 5; attribute :>> hitPoints = 43; attribute :>> strength = 56; attribute :>> gold = 2230; attribute :>> experience = 128; }
            part silentDeath : Monster { attribute :>> name = "Silent Death"; attribute :>> weapon = "Pale Smoke"; attribute :>> level = 5; attribute :>> hitPoints = 98; attribute :>> strength = 113; attribute :>> gold = 4711; attribute :>> experience = 230; attribute :>> deadly = true; }
            part baldMedusa : Monster { attribute :>> name = "Bald Medusa"; attribute :>> weapon = "Glare Of Stone"; attribute :>> level = 5; attribute :>> hitPoints = 120; attribute :>> strength = 78; attribute :>> gold = 4000; attribute :>> experience = 256; }
            part blackAlligator : Monster { attribute :>> name = "Black Alligator"; attribute :>> weapon = "Extra Sharp Teeth"; attribute :>> level = 5; attribute :>> hitPoints = 65; attribute :>> strength = 65; attribute :>> gold = 3245; attribute :>> experience = 123; }
            part clancySonOfEmpororLen : Monster { attribute :>> name = "Clancy, Son Of Emporor Len"; attribute :>> weapon = "Spiked Bull Whip"; attribute :>> level = 5; attribute :>> hitPoints = 324; attribute :>> strength = 52; attribute :>> gold = 4764; attribute :>> experience = 324; }
            part blackSorcerer : Monster { attribute :>> name = "Black Sorcerer"; attribute :>> weapon = "Spell Of Lightning"; attribute :>> level = 5; attribute :>> hitPoints = 25; attribute :>> strength = 86; attribute :>> gold = 2838; attribute :>> experience = 154; }
        }
        part level6 : ForestLevel {
            attribute :>> level = 6;
            part ironWarrior : Monster { attribute :>> name = "Iron Warrior"; attribute :>> weapon = "3 Iron"; attribute :>> level = 6; attribute :>> hitPoints = 253; attribute :>> strength = 100; attribute :>> gold = 6542; attribute :>> experience = 364; }
            part blackSoul : Monster { attribute :>> name = "Black Soul"; attribute :>> weapon = "Black Candle"; attribute :>> level = 6; attribute :>> hitPoints = 432; attribute :>> strength = 112; attribute :>> gold = 5865; attribute :>> experience = 432; attribute :>> deadly = true; }
            part goldMan : Monster { attribute :>> name = "Gold Man"; attribute :>> weapon = "Rock Arm"; attribute :>> level = 6; attribute :>> hitPoints = 354; attribute :>> strength = 86; attribute :>> gold = 8964; attribute :>> experience = 493; }
            part screamingZombie : Monster { attribute :>> name = "Screaming Zombie"; attribute :>> weapon = "Gaping Mouth Full Of Teeth"; attribute :>> level = 6; attribute :>> hitPoints = 286; attribute :>> strength = 98; attribute :>> gold = 5322; attribute :>> experience = 354; }
            part satansHelper : Monster { attribute :>> name = "Satans Helper"; attribute :>> weapon = "Pack Of Lies"; attribute :>> level = 6; attribute :>> hitPoints = 165; attribute :>> strength = 112; attribute :>> gold = 7543; attribute :>> experience = 453; }
            part wildStallion : Monster { attribute :>> name = "Wild Stallion"; attribute :>> weapon = "Hoofs"; attribute :>> level = 6; attribute :>> hitPoints = 245; attribute :>> strength = 78; attribute :>> gold = 4643; attribute :>> experience = 532; }
            part belar : Monster { attribute :>> name = "Belar"; attribute :>> weapon = "Fists Of Rage"; attribute :>> level = 6; attribute :>> hitPoints = 352; attribute :>> strength = 120; attribute :>> gold = 9432; attribute :>> experience = 565; attribute :>> deadly = true; }
            part emptyArmour : Monster { attribute :>> name = "Empty Armour"; attribute :>> weapon = "Cutting Wind"; attribute :>> level = 6; attribute :>> hitPoints = 390; attribute :>> strength = 67; attribute :>> gold = 6431; attribute :>> experience = 432; }
            part ragingLion : Monster { attribute :>> name = "Raging Lion"; attribute :>> weapon = "Teeth And Claws"; attribute :>> level = 6; attribute :>> hitPoints = 274; attribute :>> strength = 98; attribute :>> gold = 3643; attribute :>> experience = 365; }
            part hugeStoneWarrior : Monster { attribute :>> name = "Huge Stone Warrior"; attribute :>> weapon = "Rock Fist"; attribute :>> level = 6; attribute :>> hitPoints = 232; attribute :>> strength = 112; attribute :>> gold = 4942; attribute :>> experience = 543; }
            part magicalEvilGnome : Monster { attribute :>> name = "Magical Evil Gnome"; attribute :>> weapon = "Spell Of Fire"; attribute :>> level = 6; attribute :>> hitPoints = 234; attribute :>> strength = 89; attribute :>> gold = 6384; attribute :>> experience = 321; }
        }
        part level7 : ForestLevel {
            attribute :>> level = 7;
            part emporerLen : Monster { attribute :>> name = "Emporer Len"; attribute :>> weapon = "Lightning Bull Whip"; attribute :>> level = 7; attribute :>> hitPoints = 432; attribute :>> strength = 210; attribute :>> gold = 12043; attribute :>> experience = 764; }
            part nightHawk : Monster { attribute :>> name = "Night Hawk"; attribute :>> weapon = "Blood Red Talons"; attribute :>> level = 7; attribute :>> hitPoints = 675; attribute :>> strength = 220; attribute :>> gold = 10433; attribute :>> experience = 686; attribute :>> deadly = true; }
            part chargingRhinoceros : Monster { attribute :>> name = "Charging Rhinoceros"; attribute :>> weapon = "Rather Large Horn"; attribute :>> level = 7; attribute :>> hitPoints = 454; attribute :>> strength = 187; attribute :>> gold = 9853; attribute :>> experience = 654; }
            part goblinPygmy : Monster { attribute :>> name = "Goblin Pygmy"; attribute :>> weapon = "Death Squeeze"; attribute :>> level = 7; attribute :>> hitPoints = 576; attribute :>> strength = 165; attribute :>> gold = 13252; attribute :>> experience = 754; }
            part goliath : Monster { attribute :>> name = "Goliath"; attribute :>> weapon = "Six Fingered Fist"; attribute :>> level = 7; attribute :>> hitPoints = 343; attribute :>> strength = 243; attribute :>> gold = 14322; attribute :>> experience = 898; }
            part angryLiontaur : Monster { attribute :>> name = "Angry Liontaur"; attribute :>> weapon = "Arms And Teeth"; attribute :>> level = 7; attribute :>> hitPoints = 495; attribute :>> strength = 187; attribute :>> gold = 13259; attribute :>> experience = 753; }
            part fallenAngel : Monster { attribute :>> name = "Fallen Angel"; attribute :>> weapon = "Throwing Halos"; attribute :>> level = 7; attribute :>> hitPoints = 654; attribute :>> strength = 154; attribute :>> gold = 12339; attribute :>> experience = 483; }
            part wickedWombat : Monster { attribute :>> name = "Wicked Wombat"; attribute :>> weapon = "The Dark Wombats Curse"; attribute :>> level = 7; attribute :>> hitPoints = 464; attribute :>> strength = 198; attribute :>> gold = 13283; attribute :>> experience = 786; }
            part massiveDinosaur : Monster { attribute :>> name = "Massive Dinosaur"; attribute :>> weapon = "Gaping Jaws"; attribute :>> level = 7; attribute :>> hitPoints = 986; attribute :>> strength = 200; attribute :>> gold = 16753; attribute :>> experience = 1204; attribute :>> deadly = true; }
            part swissButcher : Monster { attribute :>> name = "Swiss Butcher"; attribute :>> weapon = "Meat Cleaver"; attribute :>> level = 7; attribute :>> hitPoints = 453; attribute :>> strength = 230; attribute :>> gold = 8363; attribute :>> experience = 532; }
            part deathGnome : Monster { attribute :>> name = "Death Gnome"; attribute :>> weapon = "Touch Of Death"; attribute :>> level = 7; attribute :>> hitPoints = 232; attribute :>> strength = 270; attribute :>> gold = 10000; attribute :>> experience = 654; }
        }
        part level8 : ForestLevel {
            attribute :>> level = 8;
            part screechingWitch : Monster { attribute :>> name = "Screeching Witch"; attribute :>> weapon = "Spell Of Ice"; attribute :>> level = 8; attribute :>> hitPoints = 674; attribute :>> strength = 300; attribute :>> gold = 19753; attribute :>> experience = 2283; }
            part rundorig : Monster { attribute :>> name = "Rundorig"; attribute :>> weapon = "Poison Claws"; attribute :>> level = 8; attribute :>> hitPoints = 675; attribute :>> strength = 330; attribute :>> gold = 17853; attribute :>> experience = 2748; }
            part wheeler : Monster { attribute :>> name = "Wheeler"; attribute :>> weapon = "Annoying Laugh"; attribute :>> level = 8; attribute :>> hitPoints = 786; attribute :>> strength = 250; attribute :>> gold = 23433; attribute :>> experience = 1980; }
            part deathKnight : Monster { attribute :>> name = "Death Knight"; attribute :>> weapon = "Huge Silver Sword"; attribute :>> level = 8; attribute :>> hitPoints = 674; attribute :>> strength = 287; attribute :>> gold = 21923; attribute :>> experience = 4282; }
            part werewolf : Monster { attribute :>> name = "Werewolf"; attribute :>> weapon = "Fangs"; attribute :>> level = 8; attribute :>> hitPoints = 543; attribute :>> strength = 230; attribute :>> gold = 19474; attribute :>> experience = 3853; }
            part fireOrk : Monster { attribute :>> name = "Fire Ork"; attribute :>> weapon = "FireBall"; attribute :>> level = 8; attribute :>> hitPoints = 674; attribute :>> strength = 267; attribute :>> gold = 24933; attribute :>> experience = 3942; }
            part wansBeast : Monster { attribute :>> name = "Wans Beast"; attribute :>> weapon = "Crushing Embrace"; attribute :>> level = 8; attribute :>> hitPoints = 1243; attribute :>> strength = 193; attribute :>> gold = 17141; attribute :>> experience = 2432; }
            part lordMathese : Monster { attribute :>> name = "Lord Mathese"; attribute :>> weapon = "Fencing Sword"; attribute :>> level = 8; attribute :>> hitPoints = 875; attribute :>> strength = 245; attribute :>> gold = 24935; attribute :>> experience = 2422; }
            part kingVidion : Monster { attribute :>> name = "King Vidion"; attribute :>> weapon = "Long Sword Of Death"; attribute :>> level = 8; attribute :>> hitPoints = 1243; attribute :>> strength = 400; attribute :>> gold = 28575; attribute :>> experience = 6764; attribute :>> deadly = true; }
            part babyDragon : Monster { attribute :>> name = "Baby Dragon"; attribute :>> weapon = "Dragon Smoke"; attribute :>> level = 8; attribute :>> hitPoints = 2322; attribute :>> strength = 176; attribute :>> gold = 25863; attribute :>> experience = 3675; }
            part deathGnome : Monster { attribute :>> name = "Death Gnome"; attribute :>> weapon = "Touch Of Death"; attribute :>> level = 8; attribute :>> hitPoints = 870; attribute :>> strength = 356; attribute :>> gold = 31638; attribute :>> experience = 2300; }
        }
        part level9 : ForestLevel {
            attribute :>> level = 9;
            part pinkElephant : Monster { attribute :>> name = "Pink Elephant"; attribute :>> weapon = "Stomping"; attribute :>> level = 9; attribute :>> hitPoints = 1232; attribute :>> strength = 434; attribute :>> gold = 33844; attribute :>> experience = 7843; }
            part gwendolensNightmare : Monster { attribute :>> name = "Gwendolens Nightmare"; attribute :>> weapon = "Dreams"; attribute :>> level = 9; attribute :>> hitPoints = 764; attribute :>> strength = 490; attribute :>> gold = 35846; attribute :>> experience = 8232; }
            part flyingCobra : Monster { attribute :>> name = "Flying Cobra"; attribute :>> weapon = "Poison Fangs"; attribute :>> level = 9; attribute :>> hitPoints = 1123; attribute :>> strength = 400; attribute :>> gold = 37694; attribute :>> experience = 8433; }
            part rentakisPet : Monster { attribute :>> name = "Rentakis Pet"; attribute :>> weapon = "Gaping Maw"; attribute :>> level = 9; attribute :>> hitPoints = 987; attribute :>> strength = 556; attribute :>> gold = 37584; attribute :>> experience = 9854; }
            part ernestBrown : Monster { attribute :>> name = "Ernest Brown"; attribute :>> weapon = "Knee"; attribute :>> level = 9; attribute :>> hitPoints = 2488; attribute :>> strength = 432; attribute :>> gold = 34833; attribute :>> experience = 9754; }
            part scallianRap : Monster { attribute :>> name = "Scallian Rap"; attribute :>> weapon = "Way Of Hurting People"; attribute :>> level = 9; attribute :>> hitPoints = 788; attribute :>> strength = 601; attribute :>> gold = 22430; attribute :>> experience = 6784; }
            part apeman : Monster { attribute :>> name = "Apeman"; attribute :>> weapon = "Hairy Hands"; attribute :>> level = 9; attribute :>> hitPoints = 1283; attribute :>> strength = 498; attribute :>> gold = 38955; attribute :>> experience = 7202; }
            part hemoGlob : Monster { attribute :>> name = "Hemo-Glob"; attribute :>> weapon = "Weak Insults"; attribute :>> level = 9; attribute :>> hitPoints = 1232; attribute :>> strength = 212; attribute :>> gold = 27853; attribute :>> experience = 4432; }
            part frankenmoose : Monster { attribute :>> name = "FrankenMoose"; attribute :>> weapon = "Butting Head"; attribute :>> level = 9; attribute :>> hitPoints = 1221; attribute :>> strength = 455; attribute :>> gold = 31221; attribute :>> experience = 5433; }
            part earthShaker : Monster { attribute :>> name = "Earth Shaker"; attribute :>> weapon = "Earthquake"; attribute :>> level = 9; attribute :>> hitPoints = 985; attribute :>> strength = 767; attribute :>> gold = 37565; attribute :>> experience = 7432; attribute :>> deadly = true; }
            part gollumsWrath : Monster { attribute :>> name = "Gollums Wrath"; attribute :>> weapon = "Ring Of Invisibility"; attribute :>> level = 9; attribute :>> hitPoints = 2344; attribute :>> strength = 621; attribute :>> gold = 42533; attribute :>> experience = 13544; }
        }
        part level10 : ForestLevel {
            attribute :>> level = 10;
            part toraksSonKorak : Monster { attribute :>> name = "Toraks Son, Korak"; attribute :>> weapon = "Sword Of Lightning"; attribute :>> level = 10; attribute :>> hitPoints = 1384; attribute :>> strength = 921; attribute :>> gold = 46575; attribute :>> experience = 13877; attribute :>> deadly = true; }
            part brandTheWanderer : Monster { attribute :>> name = "Brand The Wanderer"; attribute :>> weapon = "Fighting Quarter Staff"; attribute :>> level = 10; attribute :>> hitPoints = 2788; attribute :>> strength = 643; attribute :>> gold = 38755; attribute :>> experience = 13744; }
            part theGrimestReaper : Monster { attribute :>> name = "The Grimest Reaper"; attribute :>> weapon = "White Sickle"; attribute :>> level = 10; attribute :>> hitPoints = 1674; attribute :>> strength = 878; attribute :>> gold = 39844; attribute :>> experience = 14237; }
            part deathDealer : Monster { attribute :>> name = "Death Dealer"; attribute :>> weapon = "Stare Of Paralization"; attribute :>> level = 10; attribute :>> hitPoints = 1764; attribute :>> strength = 765; attribute :>> gold = 47333; attribute :>> experience = 13877; }
            part tigerOfTheDeepJungle : Monster { attribute :>> name = "Tiger Of The Deep Jungle"; attribute :>> weapon = "Eye Of The Tiger"; attribute :>> level = 10; attribute :>> hitPoints = 3101; attribute :>> strength = 587; attribute :>> gold = 43933; attribute :>> experience = 9766; }
            part sweetLookingLittleGirl : Monster { attribute :>> name = "Sweet Looking Little Girl"; attribute :>> weapon = "Demon Strike"; attribute :>> level = 10; attribute :>> hitPoints = 1232; attribute :>> strength = 989; attribute :>> gold = 52322; attribute :>> experience = 14534; attribute :>> deadly = true; }
            part floatingEvilEye : Monster { attribute :>> name = "Floating Evil Eye"; attribute :>> weapon = "Evil Stare"; attribute :>> level = 10; attribute :>> hitPoints = 2232; attribute :>> strength = 776; attribute :>> gold = 43233; attribute :>> experience = 13455; attribute :>> deadly = true; }
            part slock : Monster { attribute :>> name = "Slock"; attribute :>> weapon = "Swamp Slime"; attribute :>> level = 10; attribute :>> hitPoints = 1675; attribute :>> strength = 744; attribute :>> gold = 56444; attribute :>> experience = 14333; }
            part adultGoldDragon : Monster { attribute :>> name = "Adult Gold Dragon"; attribute :>> weapon = "Dragon Fire"; attribute :>> level = 10; attribute :>> hitPoints = 3222; attribute :>> strength = 565; attribute :>> gold = 56444; attribute :>> experience = 15364; }
            part killJoy : Monster { attribute :>> name = "Kill Joy"; attribute :>> weapon = "Terrible Stench"; attribute :>> level = 10; attribute :>> hitPoints = 3222; attribute :>> strength = 988; attribute :>> gold = 168844; attribute :>> experience = 25766; attribute :>> deadly = true; }
            part blackSorcerer : Monster { attribute :>> name = "Black Sorcerer"; attribute :>> weapon = "Spell Of Lightning"; attribute :>> level = 10; attribute :>> hitPoints = 25; attribute :>> strength = 86; attribute :>> gold = 2838; attribute :>> experience = 187; }
        }
        part level11 : ForestLevel {
            attribute :>> level = 11;
            part gormaTheLeper : Monster { attribute :>> name = "Gorma The Leper"; attribute :>> weapon = "Contagous Desease"; attribute :>> level = 11; attribute :>> hitPoints = 2766; attribute :>> strength = 1132; attribute :>> gold = 168774; attribute :>> experience = 26333; }
            part shogunWarrior : Monster { attribute :>> name = "Shogun Warrior"; attribute :>> weapon = "Japanese Nortaki"; attribute :>> level = 11; attribute :>> hitPoints = 3878; attribute :>> strength = 1143; attribute :>> gold = 165433; attribute :>> experience = 26555; }
            part apparentlyWeakOldWoman : Monster { attribute :>> name = "Apparently Weak Old Woman"; attribute :>> weapon = "*GODS HAMMER*"; attribute :>> level = 11; attribute :>> hitPoints = 1878; attribute :>> strength = 1543; attribute :>> gold = 173522; attribute :>> experience = 37762; }
            part ablesCreature : Monster { attribute :>> name = "Ables Creature"; attribute :>> weapon = "Bear Hug"; attribute :>> level = 11; attribute :>> hitPoints = 2455; attribute :>> strength = 985; attribute :>> gold = 176775; attribute :>> experience = 28222; }
            part whiteBearOfLore : Monster { attribute :>> name = "White Bear Of Lore"; attribute :>> weapon = "Snow Of Death"; attribute :>> level = 11; attribute :>> hitPoints = 1875; attribute :>> strength = 1344; attribute :>> gold = 65544; attribute :>> experience = 16775; attribute :>> deadly = true; }
            part mountain : Monster { attribute :>> name = "Mountain"; attribute :>> weapon = "Landslide"; attribute :>> level = 11; attribute :>> hitPoints = 1284; attribute :>> strength = 1544; attribute :>> gold = 186454; attribute :>> experience = 38774; attribute :>> deadly = true; }
            part sheenaTheShapechanger : Monster { attribute :>> name = "Sheena The Shapechanger"; attribute :>> weapon = "Deadly Illusions"; attribute :>> level = 11; attribute :>> hitPoints = 1898; attribute :>> strength = 1463; attribute :>> gold = 165755; attribute :>> experience = 26655; }
            part shadowstormwarrior : Monster { attribute :>> name = "ShadowStormWarrior"; attribute :>> weapon = "Mystical Storm"; attribute :>> level = 11; attribute :>> hitPoints = 2767; attribute :>> strength = 1655; attribute :>> gold = 162445; attribute :>> experience = 26181; attribute :>> deadly = true; }
            part madman : Monster { attribute :>> name = "Madman"; attribute :>> weapon = "Chant Of Insanity"; attribute :>> level = 11; attribute :>> hitPoints = 1764; attribute :>> strength = 1265; attribute :>> gold = 149564; attribute :>> experience = 25665; }
            part vegetableCreature : Monster { attribute :>> name = "Vegetable Creature"; attribute :>> weapon = "Pickled Cabbage"; attribute :>> level = 11; attribute :>> hitPoints = 172; attribute :>> strength = 111; attribute :>> gold = 4838; attribute :>> experience = 2187; }
            part cyclopsWarrior : Monster { attribute :>> name = "Cyclops Warrior"; attribute :>> weapon = "Fire Eye"; attribute :>> level = 11; attribute :>> hitPoints = 2899; attribute :>> strength = 1744; attribute :>> gold = 204000; attribute :>> experience = 49299; attribute :>> deadly = true; }
        }
        part level12 : ForestLevel {
            attribute :>> level = 12;
            part corinthianGiant : Monster { attribute :>> name = "Corinthian Giant"; attribute :>> weapon = "De-rooted Tree"; attribute :>> level = 12; attribute :>> hitPoints = 2544; attribute :>> strength = 2400; attribute :>> gold = 336643; attribute :>> experience = 60333; attribute :>> deadly = true; }
            part theScreamingEunich : Monster { attribute :>> name = "The Screaming Eunich"; attribute :>> weapon = "High Pitched Voice"; attribute :>> level = 12; attribute :>> hitPoints = 2877; attribute :>> strength = 1488; attribute :>> gold = 197888; attribute :>> experience = 78884; }
            part blackWarlock : Monster { attribute :>> name = "Black Warlock"; attribute :>> weapon = "Satanic Choruses"; attribute :>> level = 12; attribute :>> hitPoints = 2767; attribute :>> strength = 1366; attribute :>> gold = 168483; attribute :>> experience = 58989; }
            part kalTorak : Monster { attribute :>> name = "Kal Torak"; attribute :>> weapon = "Cthrek Goru"; attribute :>> level = 12; attribute :>> hitPoints = 6666; attribute :>> strength = 876; attribute :>> gold = 447774; attribute :>> experience = 94663; }
            part theMightyShadow : Monster { attribute :>> name = "The Mighty Shadow"; attribute :>> weapon = "Shadow Axe"; attribute :>> level = 12; attribute :>> hitPoints = 2332; attribute :>> strength = 1633; attribute :>> gold = 176333; attribute :>> experience = 51655; }
            part blackUnicorn : Monster { attribute :>> name = "Black Unicorn"; attribute :>> weapon = "Shredding Horn"; attribute :>> level = 12; attribute :>> hitPoints = 1587; attribute :>> strength = 1899; attribute :>> gold = 336693; attribute :>> experience = 41738; }
            part mutatedBlackWidow : Monster { attribute :>> name = "Mutated Black Widow"; attribute :>> weapon = "Venom Bite"; attribute :>> level = 12; attribute :>> hitPoints = 1276; attribute :>> strength = 2575; attribute :>> gold = 434370; attribute :>> experience = 98993; attribute :>> deadly = true; }
            part humongousBlackWyre : Monster { attribute :>> name = "Humongous Black Wyre"; attribute :>> weapon = "Death Talons"; attribute :>> level = 12; attribute :>> hitPoints = 3453; attribute :>> strength = 1166; attribute :>> gold = 653834; attribute :>> experience = 76000; }
            part theWizardOfDarkness : Monster { attribute :>> name = "The Wizard Of Darkness"; attribute :>> weapon = "Chant Of Insanity"; attribute :>> level = 12; attribute :>> hitPoints = 1383; attribute :>> strength = 1497; attribute :>> gold = 224964; attribute :>> experience = 39878; }
            part greatOgreOfTheNorth : Monster { attribute :>> name = "Great Ogre Of The North"; attribute :>> weapon = "Spiked Steel Mace"; attribute :>> level = 12; attribute :>> hitPoints = 2878; attribute :>> strength = 1800; attribute :>> gold = 524838; attribute :>> experience = 112833; }
        }

        part redDragon : Dragon;
        part darkCloakTavern : DarkCloakTavern;
        part fairies : FairyGlade;
    }

    // The bank: a tenth a day on deposits, and a vault a thief with a fairy can crack.
    part def Bank {
        attribute dailyInterestPercent : Integer = 10;
        attribute haulPerLevel : Integer = 1000;
    }

    // The healers charge five gold a hit point per level of the patient.
    part def HealersHut {
        attribute goldPerHitPointPerLevel : Integer = 5;
    }

    // A favour asked of Violet or of the bard: the charm it takes and the experience it costs.
    part def Flirt {
        attribute name : String;
        attribute suitor : Sex;
        attribute charmNeeded : Integer;
        attribute experiencePerLevel : Integer;
        attribute beds : Boolean default = false;
        attribute weds : Boolean default = false;
    }

    // Violet, Turgon's daughter, the barmaid; her favours are a man's to ask.
    part def Barmaid {
        attribute name : String = "Violet";
        part wink : Flirt { attribute :>> name = "Wink"; attribute :>> suitor = Sex::male; attribute :>> charmNeeded = 1; attribute :>> experiencePerLevel = 5; }
        part kissHerHand : Flirt { attribute :>> name = "Kiss Her Hand"; attribute :>> suitor = Sex::male; attribute :>> charmNeeded = 2; attribute :>> experiencePerLevel = 10; }
        part peckHerOnTheLips : Flirt { attribute :>> name = "Peck Her On The Lips"; attribute :>> suitor = Sex::male; attribute :>> charmNeeded = 4; attribute :>> experiencePerLevel = 20; }
        part sitHerOnYourLap : Flirt { attribute :>> name = "Sit Her On Your Lap"; attribute :>> suitor = Sex::male; attribute :>> charmNeeded = 8; attribute :>> experiencePerLevel = 30; }
        part grabHerBackside : Flirt { attribute :>> name = "Grab Her Backside"; attribute :>> suitor = Sex::male; attribute :>> charmNeeded = 16; attribute :>> experiencePerLevel = 40; }
        part carryHerUpstairs : Flirt { attribute :>> name = "Carry Her Upstairs"; attribute :>> suitor = Sex::male; attribute :>> charmNeeded = 32; attribute :>> experiencePerLevel = 40; attribute :>> beds = true; }
        part marryHer : Flirt { attribute :>> name = "Marry Her"; attribute :>> suitor = Sex::male; attribute :>> charmNeeded = 100; attribute :>> experiencePerLevel = 1000; attribute :>> weds = true; }
    }

    // Seth Able the bard; his favours are a woman's to ask, and his songs are for all.
    part def Bard {
        attribute name : String = "Seth Able";
        part wink : Flirt { attribute :>> name = "Wink"; attribute :>> suitor = Sex::female; attribute :>> charmNeeded = 1; attribute :>> experiencePerLevel = 5; }
        part flutterEyelashes : Flirt { attribute :>> name = "Flutter Eyelashes"; attribute :>> suitor = Sex::female; attribute :>> charmNeeded = 2; attribute :>> experiencePerLevel = 10; }
        part dropHanky : Flirt { attribute :>> name = "Drop Hanky"; attribute :>> suitor = Sex::female; attribute :>> charmNeeded = 4; attribute :>> experiencePerLevel = 20; }
        part askForADrink : Flirt { attribute :>> name = "Ask The Bard To Buy You A Drink"; attribute :>> suitor = Sex::female; attribute :>> charmNeeded = 8; attribute :>> experiencePerLevel = 30; }
        part kissTheBard : Flirt { attribute :>> name = "Kiss The Bard Soundly"; attribute :>> suitor = Sex::female; attribute :>> charmNeeded = 16; attribute :>> experiencePerLevel = 40; }
        part seduceTheBard : Flirt { attribute :>> name = "Completely Seduce The Bard"; attribute :>> suitor = Sex::female; attribute :>> charmNeeded = 32; attribute :>> experiencePerLevel = 40; attribute :>> beds = true; }
        part marryHim : Flirt { attribute :>> name = "Marry Him"; attribute :>> suitor = Sex::female; attribute :>> charmNeeded = 125; attribute :>> experiencePerLevel = 0; attribute :>> weds = true; }
    }

    // The Red Dragon Inn: a room by level, the bartender's bribe, gems for stats, and the locals.
    part def Inn {
        attribute roomPricePerLevel : Integer = 400;
        attribute bribePerLevel : Integer = 1600;
        attribute charmForAFreeRoom : Integer = 101;
        attribute gemsPerStatPoint : Integer = 2;
        part violet : Barmaid;
        part sethAble : Bard;
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

    // The same blow through a Light Shield, which halves it.
    calc def Shielded {
        in blow : Integer;
        in shield : Boolean;

        floor(blow / (if shield ? 2 else 1))
    }

    // What the bank adds overnight, in whole gold pieces.
    calc def Interest {
        in bankGold : Integer;
        in percent : Integer;

        floor(bankGold * percent / 100)
    }

    // A tenth of the experience: what death costs, and what a killer takes.
    calc def Tithe {
        in experience : Integer;

        floor(experience / 10)
    }

    // The inn's rates and the healers' rate rise with the warrior's level.
    calc def RoomPrice {
        in level : Integer;

        400 * level
    }

    calc def BribePrice {
        in level : Integer;

        1600 * level
    }

    calc def HealRate {
        in level : Integer;

        5 * level
    }

    // Skill uses for the day: one per so many skill points.
    calc def SkillUses {
        in points : Integer;
        in pointsPerUse : Integer;

        floor(points / pointsPerUse)
    }

    // The menu keys of the game, as the signals a warrior's day accepts.
    attribute def EnterForest;
    attribute def LookForSomethingToKill;
    attribute def SeekTheDragon;
    attribute def MeetYourGuild;
    attribute def MeetTheOldHag;
    attribute def AskTheFairies;
    attribute def CatchAFairy;
    attribute def FindTheDarkCloakTavern;
    attribute def Gamble;
    attribute def ChangeProfession;
    attribute def ReturnToTheForest;
    attribute def ReturnToTown;
    attribute def VisitTheHealer;
    attribute def VisitTheBank;
    attribute def RobTheBank;
    attribute def VisitTheTrainingHall;
    attribute def VisitTheInn;
    attribute def FlirtAtTheInn;
    attribute def ListenToTheBard;
    attribute def BribeTheBartender;
    attribute def TradeGems;
    attribute def BuyARoom;
    attribute def SlaughterOtherPlayers;
    attribute def AttackAWarrior;
    attribute def OtherPlaces;
    attribute def NewDay;

    // A warrior: the stats screen, the deeds the realm offers, and the day itself.
    part def Warrior :> Fighter {
        attribute sex : Sex default = Sex::male;
        attribute class : CharacterClass default = CharacterClass::deathKnight;
        attribute level : Integer default = 1;
        attribute experience : Integer default = 0;
        attribute :>> hitPoints default = 10;
        attribute maxHitPoints : Integer default = 10;
        attribute :>> strength default = 10;
        attribute :>> defense default = 1;
        attribute weaponTier : Integer default = 0;
        attribute weaponStrength : Integer default = 0;
        attribute armourTier : Integer default = 0;
        attribute armourDefense : Integer default = 0;
        attribute gold : Integer default = 500;
        attribute bankGold : Integer default = 0;
        attribute gems : Integer default = 0;
        attribute charm : Integer default = 1;
        attribute deathKnightPoints : Integer default = 0;
        attribute mysticalPoints : Integer default = 0;
        attribute thievingPoints : Integer default = 0;
        attribute deathKnightUses : Integer default = 0;
        attribute mysticalUses : Integer default = 0;
        attribute thievingUses : Integer default = 0;
        attribute lightShield : Boolean default = false;
        // The move a fight opens with when none is named.
        attribute favouredMove : Move default = Move::attack;
        attribute forestFightsLeft : Integer default = 15;
        attribute playerFightsLeft : Integer default = 3;
        attribute alive : Boolean default = true;
        attribute dragonKills : Integer default = 0;
        attribute playerKills : Integer default = 0;
        attribute spouse : Spouse default = Spouse::nobody;
        attribute children : Integer default = 0;
        attribute lays : Integer default = 0;
        attribute horse : Boolean default = false;
        attribute fairy : Boolean default = false;
        attribute innRoom : Boolean default = false;
        attribute bribed : Boolean default = false;
        attribute heardTheBard : Boolean default = false;
        attribute flirtedToday : Boolean default = false;
        attribute trainedToday : Boolean default = false;
        attribute taughtToday : Boolean default = false;
        attribute news : String default = "";

        // What the game never lets happen, asserted of every warrior.
        assert constraint levelInRange { level >= 1 and level <= 12 }
        assert constraint fightsNotOverdrawn { forestFightsLeft >= 0 and forestFightsLeft <= 18 + children }
        assert constraint playerFightsNotOverdrawn { playerFightsLeft >= 0 and playerFightsLeft <= 4 }
        assert constraint hitPointsWithinMaximum { hitPoints <= maxHitPoints and maxHitPoints >= 1 }
        assert constraint theDeadHaveNoHitPoints { alive or hitPoints == 0 }
        assert constraint noDebt { gold >= 0 and bankGold >= 0 }
        assert constraint gemsNotOwed { gems >= 0 }
        assert constraint charmNotNegative { charm >= 0 }
        assert constraint skillsWithinMastery {
            deathKnightPoints >= 0 and deathKnightPoints <= 40
                and mysticalPoints >= 0 and mysticalPoints <= 40
                and thievingPoints >= 0 and thievingPoints <= 40
        }
        assert constraint usesNotOverdrawn { deathKnightUses >= 0 and mysticalUses >= 0 and thievingUses >= 0 }
        assert constraint gearFromTheShops {
            weaponTier >= 0 and weaponTier <= 13 and armourTier >= 0 and armourTier <= 13
        }
        assert constraint familyCounted { children >= 0 and lays >= 0 and playerKills >= 0 and dragonKills >= 0 }
        assert constraint violetTakesGrooms { spouse != Spouse::violet or sex == Sex::male }
        assert constraint theBardTakesBrides { spouse != Spouse::sethAble or sex == Sex::female }

        // One forest fight against a monster of the warrior's level; a swing landing and a gem
        // turning up are the game's dice, left to the schedule. Skills are spent while uses last.
        action fight {
            in foe : Monster default = town.forest.level1.smallThief;
            in move : Move default = favouredMove;

            attribute foeLeft : Integer = 0;
            attribute rounds : Integer = 0;
            attribute incoming : Integer = 0;
            attribute skillReady : Boolean = false;

            first start;
            then decide mayFight;
            if forestFightsLeft > 0 and alive and foe.level == level and not (foe istype Dragon)
                and foe.hitPoints > 0 and foe.strength >= 0 and foe.defense >= 0
                and foe.gold >= 0 and foe.experience >= 0 then engage;
            else done;

            action engage {
                assign foeLeft := foe.hitPoints;
                assign forestFightsLeft := forestFightsLeft - 1;
                assign lightShield := false;
            }

            merge nextRound;

            // A skill is spent while the warrior has the uses for it; otherwise the sword.
            action ready {
                assign skillReady := (move == Move::deathKnight and deathKnightUses >= 1)
                    or (move == Move::thieving and thievingUses >= 1)
                    or (move == Move::pinchRealHard and mysticalUses >= 1)
                    or (move == Move::disappear and mysticalUses >= 4)
                    or (move == Move::heatWave and mysticalUses >= 8)
                    or (move == Move::lightShield and mysticalUses >= 12 and not lightShield)
                    or (move == Move::shatter and mysticalUses >= 16)
                    or (move == Move::mindHeal and mysticalUses >= 20);
                assign rounds := rounds + 1;
            }

            decide choose;
            decide swing;

            action strike {
                assign foeLeft := foeLeft - Blow(strength, foe.defense);
            }

            action miss;

            // The Death Knights' power move and the thieves' sneak attack: three blows in one.
            action powerMove {
                assign foeLeft := foeLeft - 3 * Blow(strength, foe.defense);
                assign deathKnightUses := deathKnightUses - 1;
            }

            action sneakAttack {
                assign foeLeft := foeLeft - 3 * Blow(strength, foe.defense);
                assign thievingUses := thievingUses - 1;
            }

            // The mystic's spells, each for its price in uses.
            action pinch {
                assign foeLeft := foeLeft - 2 * Blow(strength, foe.defense);
                assign mysticalUses := mysticalUses - 1;
            }

            action vanish {
                assign mysticalUses := mysticalUses - 4;
                assign lightShield := false;
            }

            action heat {
                assign foeLeft := foeLeft - 3 * Blow(strength, foe.defense);
                assign mysticalUses := mysticalUses - 8;
            }

            action shield {
                assign lightShield := true;
                assign mysticalUses := mysticalUses - 12;
            }

            action shatterBlow {
                assign foeLeft := foeLeft - 6 * Blow(strength, foe.defense);
                assign mysticalUses := mysticalUses - 16;
            }

            action mend {
                assign hitPoints := maxHitPoints;
                assign mysticalUses := mysticalUses - 20;
            }

            merge landed;
            decide foeStanding;

            action riposte {
                assign incoming := Shielded(Blow(foe.strength, defense), lightShield);
            }

            decide warriorStanding;

            action takeBlow {
                assign hitPoints := hitPoints - incoming;
            }

            // A grown child sneaks up on the foe and takes the killing blow instead.
            action childIntervenes {
                assign children := children - 1;
                assign foeLeft := foeLeft - Blow(strength, foe.defense);
            }

            decide childStanding;

            // A fairy caught today flies out of the pocket and revives the warrior once.
            action fairyRevives {
                assign fairy := false;
                assign hitPoints := maxHitPoints;
            }

            action win {
                assign gold := gold + foe.gold;
                assign experience := experience + foe.experience;
                assign lightShield := false;
            }

            decide spoils;

            action findGem {
                assign gems := gems + 1;
            }

            action noGem;

            // Death in the forest costs the gold on hand, a tenth of the experience, and the day.
            action die {
                assign alive := false;
                assign hitPoints := 0;
                assign gold := 0;
                assign experience := experience - Tithe(experience);
                assign lightShield := false;
            }

            succession first engage then nextRound;
            succession first nextRound then ready;
            succession first ready then choose;
            first choose if not skillReady then swing;
            first choose if skillReady and move == Move::deathKnight then powerMove;
            first choose if skillReady and move == Move::thieving then sneakAttack;
            first choose if skillReady and move == Move::pinchRealHard then pinch;
            first choose if skillReady and move == Move::disappear then vanish;
            first choose if skillReady and move == Move::heatWave then heat;
            first choose if skillReady and move == Move::lightShield then shield;
            first choose if skillReady and move == Move::shatter then shatterBlow;
            first choose if skillReady and move == Move::mindHeal then mend;
            first swing if foeLeft > 0 then strike;
            first swing if foeLeft > 0 then miss;
            succession first strike then landed;
            succession first powerMove then landed;
            succession first sneakAttack then landed;
            succession first pinch then landed;
            succession first heat then landed;
            succession first shatterBlow then landed;
            succession first landed then foeStanding;
            succession first miss then riposte;
            succession first shield then riposte;
            succession first mend then riposte;
            succession first vanish then done;
            first foeStanding if foeLeft <= 0 then win;
            first foeStanding if foeLeft > 0 then riposte;
            succession first riposte then warriorStanding;
            first warriorStanding if incoming < hitPoints then takeBlow;
            first warriorStanding if incoming >= hitPoints and children > 0 then childIntervenes;
            first warriorStanding if incoming >= hitPoints and children <= 0 and fairy then fairyRevives;
            first warriorStanding if incoming >= hitPoints and children <= 0 and not fairy then die;
            succession first takeBlow then nextRound;
            succession first childIntervenes then childStanding;
            first childStanding if foeLeft <= 0 then win;
            first childStanding if foeLeft > 0 then nextRound;
            succession first fairyRevives then nextRound;
            succession first win then spoils;
            first spoils if true then findGem;
            first spoils if true then noGem;
            succession first findGem then done;
            succession first noGem then done;
            succession first die then done;
        }

        // The dragon's lair, open to a level-twelve warrior with a fight left: its breath burns
        // through any armour; slaying it starts the warrior over, keeping skills, charm, gems, kin.
        action fightDragon {
            in dragon : Dragon = town.forest.redDragon;
            in move : Move default = favouredMove;

            attribute dragonLeft : Integer = 0;
            attribute rounds : Integer = 0;
            attribute incoming : Integer = 0;
            attribute skillReady : Boolean = false;

            first start;
            then decide mayFight;
            if forestFightsLeft > 0 and alive and level == 12 and dragon.hitPoints > 0
                and dragon.strength >= 0 and dragon.defense >= 0 and dragon.breath >= 0 then engage;
            else done;

            action engage {
                assign dragonLeft := dragon.hitPoints;
                assign forestFightsLeft := forestFightsLeft - 1;
                assign lightShield := false;
            }

            merge nextRound;

            action ready {
                assign skillReady := (move == Move::deathKnight and deathKnightUses >= 1)
                    or (move == Move::thieving and thievingUses >= 1)
                    or (move == Move::pinchRealHard and mysticalUses >= 1)
                    or (move == Move::heatWave and mysticalUses >= 8)
                    or (move == Move::lightShield and mysticalUses >= 12 and not lightShield)
                    or (move == Move::shatter and mysticalUses >= 16)
                    or (move == Move::mindHeal and mysticalUses >= 20);
                assign rounds := rounds + 1;
            }

            decide choose;
            decide swing;

            action strike {
                assign dragonLeft := dragonLeft - Blow(strength, dragon.defense);
            }

            action miss;

            action powerMove {
                assign dragonLeft := dragonLeft - 3 * Blow(strength, dragon.defense);
                assign deathKnightUses := deathKnightUses - 1;
            }

            action sneakAttack {
                assign dragonLeft := dragonLeft - 3 * Blow(strength, dragon.defense);
                assign thievingUses := thievingUses - 1;
            }

            action pinch {
                assign dragonLeft := dragonLeft - 2 * Blow(strength, dragon.defense);
                assign mysticalUses := mysticalUses - 1;
            }

            action heat {
                assign dragonLeft := dragonLeft - 3 * Blow(strength, dragon.defense);
                assign mysticalUses := mysticalUses - 8;
            }

            action shield {
                assign lightShield := true;
                assign mysticalUses := mysticalUses - 12;
            }

            action shatterBlow {
                assign dragonLeft := dragonLeft - 6 * Blow(strength, dragon.defense);
                assign mysticalUses := mysticalUses - 16;
            }

            action mend {
                assign hitPoints := maxHitPoints;
                assign mysticalUses := mysticalUses - 20;
            }

            merge landed;
            decide dragonStanding;
            merge dragonsTurn;
            decide dragonAttacks;

            action flamingBreath {
                assign incoming := Shielded(dragon.breath, lightShield);
            }

            action stomp {
                assign incoming := Shielded(Blow(dragon.strength, defense), lightShield);
            }

            action hugeClaw {
                assign incoming := Shielded(Blow(floor(dragon.strength / 2), defense), lightShield);
            }

            action swishingTail {
                assign incoming := Shielded(Blow(floor(dragon.strength / 4), defense), lightShield);
            }

            merge struck;
            decide warriorStanding;

            action takeBlow {
                assign hitPoints := hitPoints - incoming;
            }

            action childIntervenes {
                assign children := children - 1;
                assign dragonLeft := dragonLeft - Blow(strength, dragon.defense);
            }

            decide childStanding;

            action fairyRevives {
                assign fairy := false;
                assign hitPoints := maxHitPoints;
            }

            action slay {
                assign dragonKills := dragonKills + 1;
                assign lightShield := false;
            }

            // The quest is not over: the warrior wakes a new person, keeping what was earned.
            action bornAgain {
                assign level := 1;
                assign experience := 0;
                assign maxHitPoints := 10;
                assign hitPoints := 10;
                assign strength := 10;
                assign defense := 1;
                assign weaponTier := 0;
                assign weaponStrength := 0;
                assign armourTier := 0;
                assign armourDefense := 0;
                assign gold := 500;
                assign bankGold := 0;
                assign forestFightsLeft := 0;
            }

            action die {
                assign alive := false;
                assign hitPoints := 0;
                assign gold := 0;
                assign experience := experience - Tithe(experience);
                assign lightShield := false;
            }

            succession first engage then nextRound;
            succession first nextRound then ready;
            succession first ready then choose;
            first choose if not skillReady then swing;
            first choose if skillReady and move == Move::deathKnight then powerMove;
            first choose if skillReady and move == Move::thieving then sneakAttack;
            first choose if skillReady and move == Move::pinchRealHard then pinch;
            first choose if skillReady and move == Move::heatWave then heat;
            first choose if skillReady and move == Move::lightShield then shield;
            first choose if skillReady and move == Move::shatter then shatterBlow;
            first choose if skillReady and move == Move::mindHeal then mend;
            first swing if dragonLeft > 0 then strike;
            first swing if dragonLeft > 0 then miss;
            succession first strike then landed;
            succession first powerMove then landed;
            succession first sneakAttack then landed;
            succession first pinch then landed;
            succession first heat then landed;
            succession first shatterBlow then landed;
            succession first landed then dragonStanding;
            succession first miss then dragonsTurn;
            succession first shield then dragonsTurn;
            succession first mend then dragonsTurn;
            first dragonStanding if dragonLeft <= 0 then slay;
            first dragonStanding if dragonLeft > 0 then dragonsTurn;
            succession first dragonsTurn then dragonAttacks;
            first dragonAttacks if true then flamingBreath;
            first dragonAttacks if true then stomp;
            first dragonAttacks if true then hugeClaw;
            first dragonAttacks if true then swishingTail;
            succession first flamingBreath then struck;
            succession first stomp then struck;
            succession first hugeClaw then struck;
            succession first swishingTail then struck;
            succession first struck then warriorStanding;
            first warriorStanding if incoming < hitPoints then takeBlow;
            first warriorStanding if incoming >= hitPoints and children > 0 then childIntervenes;
            first warriorStanding if incoming >= hitPoints and children <= 0 and fairy then fairyRevives;
            first warriorStanding if incoming >= hitPoints and children <= 0 and not fairy then die;
            succession first takeBlow then nextRound;
            succession first childIntervenes then childStanding;
            first childStanding if dragonLeft <= 0 then slay;
            first childStanding if dragonLeft > 0 then nextRound;
            succession first fairyRevives then nextRound;
            succession first slay then bornAgain;
            succession first bornAgain then done;
            succession first die then done;
        }

        // Slaughter another warrior: three a day, none asleep at the inn unless the bartender
        // is bribed and the sleeper is at most a level below. Power moves need a higher-level
        // foe. The slain lose their gold, gems and a tenth of their experience to the killer;
        // an attacker who dies keeps the gems.
        action attack {
            in foe : Warrior = rival;
            in move : Move default = favouredMove;

            attribute rounds : Integer = 0;
            attribute incoming : Integer = 0;
            attribute skillReady : Boolean = false;

            first start;
            then decide mayAttack;
            if playerFightsLeft > 0 and alive and foe != self and foe.alive and foe.hitPoints > 0
                and foe.strength >= 0 and foe.defense >= 0
                and foe.gold >= 0 and foe.experience >= 0 and foe.gems >= 0
                and (not foe.innRoom or (bribed and foe.level >= level - 1)) then engage;
            else done;

            action engage {
                assign playerFightsLeft := playerFightsLeft - 1;
                assign lightShield := false;
            }

            merge nextRound;

            action ready {
                assign skillReady := foe.level > level
                    and ((move == Move::deathKnight and deathKnightUses >= 1)
                        or (move == Move::thieving and thievingUses >= 1));
                assign rounds := rounds + 1;
            }

            decide choose;
            decide swing;

            action strike {
                assign foe.hitPoints := max(0, foe.hitPoints - Blow(strength, foe.defense));
            }

            action miss;

            action powerMove {
                assign foe.hitPoints := max(0, foe.hitPoints - 3 * Blow(strength, foe.defense));
                assign deathKnightUses := deathKnightUses - 1;
            }

            action sneakAttack {
                assign foe.hitPoints := max(0, foe.hitPoints - 3 * Blow(strength, foe.defense));
                assign thievingUses := thievingUses - 1;
            }

            merge landed;
            decide foeStanding;

            action riposte {
                assign incoming := Blow(foe.strength, defense);
            }

            decide warriorStanding;

            action takeBlow {
                assign hitPoints := hitPoints - incoming;
            }

            action fairyRevives {
                assign fairy := false;
                assign hitPoints := maxHitPoints;
            }

            action slay {
                assign foe.alive := false;
                assign gold := gold + foe.gold;
                assign foe.gold := 0;
                assign gems := gems + foe.gems;
                assign foe.gems := 0;
                assign experience := experience + Tithe(foe.experience);
                assign foe.experience := foe.experience - Tithe(foe.experience);
                assign playerKills := playerKills + 1;
            }

            action die {
                assign alive := false;
                assign hitPoints := 0;
                assign gold := 0;
                assign experience := experience - Tithe(experience);
            }

            succession first engage then nextRound;
            succession first nextRound then ready;
            succession first ready then choose;
            first choose if not skillReady then swing;
            first choose if skillReady and move == Move::deathKnight then powerMove;
            first choose if skillReady and move == Move::thieving then sneakAttack;
            first swing if foe.hitPoints > 0 then strike;
            first swing if foe.hitPoints > 0 then miss;
            succession first strike then landed;
            succession first powerMove then landed;
            succession first sneakAttack then landed;
            succession first landed then foeStanding;
            succession first miss then riposte;
            first foeStanding if foe.hitPoints <= 0 then slay;
            first foeStanding if foe.hitPoints > 0 then riposte;
            succession first riposte then warriorStanding;
            first warriorStanding if incoming < hitPoints then takeBlow;
            first warriorStanding if incoming >= hitPoints and fairy then fairyRevives;
            first warriorStanding if incoming >= hitPoints and not fairy then die;
            succession first takeBlow then nextRound;
            succession first fairyRevives then nextRound;
            succession first slay then done;
            succession first die then done;
        }

        // The healers mend every point, or as many as the gold on hand pays for, at five
        // gold a point per level. The dead are past healing.
        action heal {
            attribute rate : Integer = 0;
            attribute affordable : Integer = 0;

            first start;
            then decide mayHeal;
            if alive and level >= 1 and hitPoints < maxHitPoints then price;
            else done;

            action price {
                assign rate := HealRate(level);
                assign affordable := floor(gold / rate);
            }

            decide canAffordAll;

            action mendAll {
                assign gold := gold - (maxHitPoints - hitPoints) * rate;
                assign hitPoints := maxHitPoints;
            }

            action mendSome {
                assign hitPoints := hitPoints + affordable;
                assign gold := gold - affordable * rate;
            }

            succession first price then canAffordAll;
            first canAffordAll if affordable >= maxHitPoints - hitPoints then mendAll;
            first canAffordAll if affordable < maxHitPoints - hitPoints then mendSome;
            succession first mendAll then done;
            succession first mendSome then done;
        }

        // Gold in the bank earns interest and cannot be taken by another player.
        // The teller moves only gold the warrior actually has.
        action deposit {
            in amount : Integer;

            first start;
            then decide hasGold;
            if alive and amount >= 0 and gold >= amount then move;
            else done;

            action move {
                assign bankGold := bankGold + amount;
                assign gold := gold - amount;
            }
            then done;
        }

        action withdraw {
            in amount : Integer;

            first start;
            then decide hasGold;
            if alive and amount >= 0 and bankGold >= amount then move;
            else done;

            action move {
                assign bankGold := bankGold - amount;
                assign gold := gold + amount;
            }
            then done;
        }

        // A thief with a fairy in the pocket picks the vault's lock: a haul by level.
        action robTheBank {
            first start;
            then decide mayRob;
            if alive and class == CharacterClass::thievingSkills and fairy then pickTheLock;
            else done;

            action pickTheLock {
                assign gold := gold + town.bank.haulPerLevel * level;
                assign fairy := false;
            }
            then done;
        }

        // A shop sells for the full price, the new item's stat replacing the old one's.
        action buyWeapon {
            in weapon : Weapon = town.weapons.stick;

            first start;
            then decide canPay;
            if alive and weapon.price >= 0 and weapon.strength >= 0
                and weapon.tier >= 0 and weapon.tier <= 13 and gold >= weapon.price then pay;
            else done;

            action pay {
                assign gold := gold - weapon.price;
                assign strength := strength - weaponStrength + weapon.strength;
                assign weaponStrength := weapon.strength;
                assign weaponTier := weapon.tier;
            }
            then done;
        }

        action buyArmour {
            in armour : Armour = town.armour.coat;

            first start;
            then decide canPay;
            if alive and armour.price >= 0 and armour.defense >= 0
                and armour.tier >= 0 and armour.tier <= 13 and gold >= armour.price then pay;
            else done;

            action pay {
                assign gold := gold - armour.price;
                assign defense := defense - armourDefense + armour.defense;
                assign armourDefense := armour.defense;
                assign armourTier := armour.tier;
            }
            then done;
        }

        // A bout at the hall, against Halder unless told otherwise, once a day: a win is the
        // next level and a skill point in the warrior's class; a loss only bruises, and the
        // master mends it. Only the master of the next level will fight, and a real one.
        action train {
            in master : Master = town.training.halder;

            attribute masterLeft : Integer = 0;
            attribute promoted : Boolean = false;

            first start;
            then decide ready;
            if alive and not trainedToday and level < 12 and master.teaches == level + 1
                and experience >= master.experienceRequired
                and master.hitPoints > 0 and master.strength >= 0 and master.defense >= 0
                and master.hitPointsTaught >= 0 and master.strengthTaught >= 0
                and master.defenseTaught >= 0 then bout;
            else turnedAway;

            action bout {
                assign trainedToday := true;
                assign masterLeft := master.hitPoints;
                loop {
                    assign masterLeft := masterLeft - Blow(strength, master.defense);
                    assign hitPoints := if masterLeft > 0 ? max(1, hitPoints - Blow(master.strength, defense)) else hitPoints;
                } until masterLeft <= 0 or hitPoints <= 1;
            }

            action turnedAway;

            decide verdict;

            action levelUp {
                assign level := level + 1;
                assign maxHitPoints := maxHitPoints + master.hitPointsTaught;
                assign hitPoints := hitPoints + master.hitPointsTaught;
                assign strength := strength + master.strengthTaught;
                assign defense := defense + master.defenseTaught;
                assign deathKnightPoints := if class == CharacterClass::deathKnight and deathKnightPoints < 40 ? deathKnightPoints + 1 else deathKnightPoints;
                assign mysticalPoints := if class == CharacterClass::mysticalSkills and mysticalPoints < 40 ? mysticalPoints + 1 else mysticalPoints;
                assign thievingPoints := if class == CharacterClass::thievingSkills and thievingPoints < 40 ? thievingPoints + 1 else thievingPoints;
                assign promoted := true;
            }

            action beaten {
                assign hitPoints := maxHitPoints;
            }

            succession first bout then verdict;
            first verdict if masterLeft <= 0 then levelUp;
            first verdict if masterLeft > 0 then beaten;
            succession first levelUp then done;
            succession first beaten then done;
            succession first turnedAway then done;
        }

        // A skill lesson in the forest, from the warrior's own guild: the Death Knights'
        // judgement is a coin toss, the mystic's number a guess, the thieves' lesson a gem.
        // Forty points is ultra mastery, and the end of the lessons.
        action learnSkill {
            first start;
            then decide guild;
            if alive and not taughtToday and class == CharacterClass::deathKnight and deathKnightPoints < 40 then castle;
            if alive and not taughtToday and class == CharacterClass::mysticalSkills and mysticalPoints < 40 then hut;
            if alive and not taughtToday and class == CharacterClass::thievingSkills and thievingPoints < 40 and gems >= 1 then lesson;
            else done;

            action castle {
                assign taughtToday := true;
            }
            decide judgement;

            action judgedRightly {
                assign deathKnightPoints := deathKnightPoints + 1;
            }

            action judgedWrongly;

            action hut {
                assign taughtToday := true;
            }
            decide guess;

            action guessed {
                assign mysticalPoints := mysticalPoints + 1;
            }

            action missed;

            action lesson {
                assign taughtToday := true;
                assign gems := gems - 1;
                assign thievingPoints := thievingPoints + 1;
            }

            succession first castle then judgement;
            first judgement if true then judgedRightly;
            first judgement if true then judgedWrongly;
            succession first judgedRightly then done;
            succession first judgedWrongly then done;
            succession first hut then guess;
            first guess if true then guessed;
            first guess if true then missed;
            succession first guessed then done;
            succession first missed then done;
            succession first lesson then done;
        }

        // The Old Hag heals a warrior and adds a hit point, for a gem.
        action meetTheOldHag {
            first start;
            then decide hasGem;
            if alive and gems >= 1 then bargain;
            else done;

            action bargain {
                assign gems := gems - 1;
                assign maxHitPoints := maxHitPoints + 1;
                assign hitPoints := maxHitPoints;
            }
            then done;
        }

        // The fairies grant a blessing for the asking: a healing kiss, a horse, a gem for a
        // sad story, or lore worth experience by level.
        action askTheFairies {
            in blessing : Blessing = Blessing::kiss;

            first start;
            then decide mayAsk;
            if alive then ask;
            else done;

            action ask;
            decide granted;

            action kissed {
                assign hitPoints := maxHitPoints;
            }

            action mounted {
                assign horse := true;
            }

            action tearToGem {
                assign gems := gems + 1;
            }

            action lore {
                assign experience := experience + town.forest.fairies.loreExperiencePerLevel * level;
            }

            succession first ask then granted;
            first granted if blessing == Blessing::kiss then kissed;
            first granted if blessing == Blessing::horse then mounted;
            first granted if blessing == Blessing::sadStory then tearToGem;
            first granted if blessing == Blessing::lore then lore;
            succession first kissed then done;
            succession first mounted then done;
            succession first tearToGem then done;
            succession first lore then done;
        }

        // Grab for a fairy: caught, it revives the warrior once today; missed, the
        // thornberry bush leaves one hit point.
        action catchAFairy {
            first start;
            then decide mayGrab;
            if alive and not fairy then grab;
            else done;

            action grab;
            decide luck;

            action caught {
                assign fairy := true;
            }

            action thornberries {
                assign hitPoints := 1;
            }

            succession first grab then luck;
            first luck if true then caught;
            first luck if true then thornberries;
            succession first caught then done;
            succession first thornberries then done;
        }

        // At the Dark Cloak Tavern, Chance changes a warrior's profession; the skills
        // already learned are kept.
        action changeProfession {
            in profession : CharacterClass = CharacterClass::thievingSkills;

            first start;
            then decide mayChange;
            if alive then change;
            else done;

            action change {
                assign class := profession;
            }
            then done;
        }

        // The old man's games: a wager the warrior can cover, won or lost on the dice.
        action gamble {
            in wager : Integer = 100;

            first start;
            then decide mayBet;
            if alive and wager > 0 and gold >= wager then bet;
            else done;

            action bet;
            decide dice;

            action won {
                assign gold := gold + wager;
            }

            action lost {
                assign gold := gold - wager;
            }

            succession first bet then dice;
            first dice if true then won;
            first dice if true then lost;
            succession first won then done;
            succession first lost then done;
        }

        // A room for the night keeps a warrior off the slaughter list; charm enough
        // and the bartender waives the price.
        action buyRoom {
            attribute price : Integer = 0;

            first start;
            then action quote {
                assign price := if charm >= town.inn.charmForAFreeRoom ? 0 else RoomPrice(level);
            }
            then decide mayStay;
            if alive and not innRoom and gold >= price then stay;
            else done;

            action stay {
                assign gold := gold - price;
                assign innRoom := true;
            }
            then done;
        }

        // The bartender, bribed, lends the room keys; he will not talk to a first-level warrior.
        action bribeTheBartender {
            first start;
            then decide mayBribe;
            if alive and level >= 2 and not bribed and gold >= BribePrice(level) then pay;
            else done;

            action pay {
                assign gold := gold - BribePrice(level);
                assign bribed := true;
            }
            then done;
        }

        // Two gems buy a point of the stat named, from the bartender, from level two.
        action tradeGems {
            in stat : Stat = Stat::defense;

            first start;
            then decide mayTrade;
            if alive and level >= 2 and gems >= town.inn.gemsPerStatPoint then pay;
            else done;

            action pay {
                assign gems := gems - town.inn.gemsPerStatPoint;
            }

            decide which;

            action moreStrength {
                assign strength := strength + 1;
            }

            action moreDefense {
                assign defense := defense + 1;
            }

            action moreHitPoints {
                assign maxHitPoints := maxHitPoints + 1;
                assign hitPoints := hitPoints + 1;
            }

            succession first pay then which;
            first which if stat == Stat::strength then moreStrength;
            first which if stat == Stat::defense then moreDefense;
            first which if stat == Stat::hitPoints then moreHitPoints;
            succession first moreStrength then done;
            succession first moreDefense then done;
            succession first moreHitPoints then done;
        }

        // A favour asked of Violet or the bard, once a day: it takes the charm, costs the
        // experience, and wins a charm point; a night upstairs may leave a child; the last
        // favour is marriage. The married and the wrong sex are turned away.
        action flirt {
            in favour : Flirt = town.inn.violet.wink;

            attribute cost : Integer = 0;

            first start;
            then action reckon {
                assign cost := favour.experiencePerLevel * level;
            }
            then decide welcome;
            if alive and not flirtedToday and favour.suitor == sex and spouse == Spouse::nobody
                and favour.charmNeeded >= 0 and favour.experiencePerLevel >= 0
                and charm >= favour.charmNeeded and experience >= cost then pay;
            else done;

            action pay {
                assign experience := experience - cost;
                assign flirtedToday := true;
            }

            decide favourKind;

            action charmed {
                assign charm := charm + 1;
            }

            action wed {
                assign spouse := if favour.suitor == Sex::male ? Spouse::violet else Spouse::sethAble;
            }

            action bed {
                assign lays := lays + 1;
                assign charm := charm + 1;
            }

            decide cradle;

            action childBorn {
                assign children := children + 1;
            }

            action noChild;

            succession first pay then favourKind;
            first favourKind if favour.weds then wed;
            first favourKind if favour.beds and not favour.weds then bed;
            first favourKind if not favour.beds and not favour.weds then charmed;
            succession first charmed then done;
            succession first wed then done;
            succession first bed then cradle;
            first cradle if true then childBorn;
            first cradle if true then noChild;
            succession first childBorn then done;
            succession first noChild then done;
        }

        // A divorce sets charm back to fifty, or to thirty for a bride the bard leaves.
        action divorce {
            first start;
            then decide married;
            if alive and spouse != Spouse::nobody then separate;
            else done;

            action separate {
                assign charm := if spouse == Spouse::sethAble ? 30 else 50;
                assign spouse := Spouse::nobody;
            }
            then done;
        }

        // Seth Able's song, once a day: which one he sings is the dice, and each does
        // something for the listener.
        action listenToTheBard {
            first start;
            then decide mayListen;
            if alive and not heardTheBard then listen;
            else done;

            action listen {
                assign heardTheBard := true;
            }

            decide song;

            action threeFights {
                assign forestFightsLeft := forestFightsLeft + 3;
            }

            action twoFights {
                assign forestFightsLeft := forestFightsLeft + 2;
            }

            action oneFight {
                assign forestFightsLeft := forestFightsLeft + 1;
            }

            action userBattle {
                assign playerFightsLeft := playerFightsLeft + 1;
            }

            action hitPointsMaxed {
                assign hitPoints := maxHitPoints;
            }

            action hitPointUp {
                assign maxHitPoints := maxHitPoints + 1;
                assign hitPoints := hitPoints + 1;
            }

            action bankDoubled {
                assign bankGold := 2 * bankGold;
            }

            action charmPoint {
                assign charm := charm + 1;
            }

            succession first listen then song;
            first song if true then threeFights;
            first song if true then twoFights;
            first song if true then oneFight;
            first song if true then userBattle;
            first song if true then hitPointsMaxed;
            first song if true then hitPointUp;
            first song if true then bankDoubled;
            first song if true then charmPoint;
            succession first threeFights then done;
            succession first twoFights then done;
            succession first oneFight then done;
            succession first userBattle then done;
            succession first hitPointsMaxed then done;
            succession first hitPointUp then done;
            succession first bankDoubled then done;
            succession first charmPoint then done;
        }

        // Midnight: the fights come back, one more per child; the skills' uses are dealt;
        // the bank pays (never charges); the slain rise; the fairy escapes; the inn empties;
        // and the town crier has the day's news, whichever the dice pick.
        action newDay {
            in interestPercent : Integer = 10;

            first start;
            then action reset {
                assign forestFightsLeft := 15 + children;
                assign playerFightsLeft := 3;
                assign deathKnightUses := SkillUses(deathKnightPoints, 4);
                assign thievingUses := SkillUses(thievingPoints, 4);
                assign mysticalUses := mysticalPoints;
                assign bankGold := bankGold + Interest(bankGold, max(0, interestPercent));
                assign hitPoints := if alive ? hitPoints else maxHitPoints;
                assign alive := true;
                assign lightShield := false;
                assign fairy := false;
                assign innRoom := false;
                assign bribed := false;
                assign heardTheBard := false;
                assign flirtedToday := false;
                assign trainedToday := false;
                assign taughtToday := false;
            }

            decide crier;

            action moreChildrenMissing { assign news := "More children are missing today."; }
            action smallGirlMissing { assign news := "A small girl was missing today."; }
            action townInGrief { assign news := "The town is in grief.  Several children didn't come home today."; }
            action dragonSighting { assign news := "Dragon sighting reported today by a drunken old man."; }
            action bloodyRemains { assign news := "Despair covers the land - more bloody remains have been found today."; }
            action natureWalk { assign news := "A group of children did not return from a nature walk today."; }
            action landInChaos { assign news := "The land is in chaos today.  Will the abductions ever stop?"; }
            action dragonScales { assign news := "Dragon scales have been found in the forest today..Old or new?"; }
            action missingCattle { assign news := "Several farmers report missing cattle today."; }
            action childFound { assign news := "A Child was found today!  But scared deaf and dumb."; }

            succession first reset then crier;
            first crier if true then moreChildrenMissing;
            first crier if true then smallGirlMissing;
            first crier if true then townInGrief;
            first crier if true then dragonSighting;
            first crier if true then bloodyRemains;
            first crier if true then natureWalk;
            first crier if true then landInChaos;
            first crier if true then dragonScales;
            first crier if true then missingCattle;
            first crier if true then childFound;
            succession first moreChildrenMissing then done;
            succession first smallGirlMissing then done;
            succession first townInGrief then done;
            succession first dragonSighting then done;
            succession first bloodyRemains then done;
            succession first natureWalk then done;
            succession first landInChaos then done;
            succession first dragonScales then done;
            succession first missingCattle then done;
            succession first childFound then done;
        }

        // The game's menu: the town square, the places its keys lead to, the forest and
        // what hides in it, the inn's room, and the grave a slain warrior waits in.
        exhibit state day {
            entry; then townSquare;

            state townSquare;

            state forest;

            state darkCloakTavern;

            state healersHut;

            state bank;

            state trainingHall;

            state inn;

            // Abed at the inn, a warrior waits for midnight.
            state asleep;

            state slaughter;

            // The town's "Other Places" are the game's plug-in modules; the model has none.
            state otherPlaces;

            state slain;

            transition townSquare_forest
                first townSquare
                accept EnterForest
                then forest;

            // While fights remain, the first monster of the warrior's level is fought (the game
            // rolled among the level's eleven); the warrior stays in the forest or goes to the grave.
            transition forest_fight
                first forest
                accept LookForSomethingToKill
                if forestFightsLeft > 0 and alive
                do action fighting {
                    first start;
                    then decide byLevel;
                    if level == 1 then level1;
                    if level == 2 then level2;
                    if level == 3 then level3;
                    if level == 4 then level4;
                    if level == 5 then level5;
                    if level == 6 then level6;
                    if level == 7 then level7;
                    if level == 8 then level8;
                    if level == 9 then level9;
                    if level == 10 then level10;
                    if level == 11 then level11;
                    if level == 12 then level12;
                    else done;

                    action level1 { first start; then perform fight { in foe = town.forest.level1.smallThief; } then done; }
                    then done;
                    action level2 { first start; then perform fight { in foe = town.forest.level2.greenPython; } then done; }
                    then done;
                    action level3 { first start; then perform fight { in foe = town.forest.level3.lazyBum; } then done; }
                    then done;
                    action level4 { first start; then perform fight { in foe = town.forest.level4.deathDog; } then done; }
                    then done;
                    action level5 { first start; then perform fight { in foe = town.forest.level5.pandionKnight; } then done; }
                    then done;
                    action level6 { first start; then perform fight { in foe = town.forest.level6.ironWarrior; } then done; }
                    then done;
                    action level7 { first start; then perform fight { in foe = town.forest.level7.emporerLen; } then done; }
                    then done;
                    action level8 { first start; then perform fight { in foe = town.forest.level8.screechingWitch; } then done; }
                    then done;
                    action level9 { first start; then perform fight { in foe = town.forest.level9.pinkElephant; } then done; }
                    then done;
                    action level10 { first start; then perform fight { in foe = town.forest.level10.toraksSonKorak; } then done; }
                    then done;
                    action level11 { first start; then perform fight { in foe = town.forest.level11.gormaTheLeper; } then done; }
                    then done;
                    action level12 { first start; then perform fight { in foe = town.forest.level12.corinthianGiant; } then done; }
                    then done;
                }
                then forest;

            transition forest_dragon
                first forest
                accept SeekTheDragon
                if forestFightsLeft > 0 and alive and level == 12
                do action braving { perform fightDragon; }
                then forest;

            transition forest_guild
                first forest
                accept MeetYourGuild
                if alive
                do action learning { perform learnSkill; }
                then forest;

            transition forest_hag
                first forest
                accept MeetTheOldHag
                if alive and gems >= 1
                do action bargaining { perform meetTheOldHag; }
                then forest;

            transition forest_fairies
                first forest
                accept AskTheFairies
                if alive
                do action asking { perform askTheFairies; }
                then forest;

            transition forest_catch
                first forest
                accept CatchAFairy
                if alive and not fairy
                do action grabbing { perform catchAFairy; }
                then forest;

            transition forest_tavern
                first forest
                accept FindTheDarkCloakTavern
                if alive
                then darkCloakTavern;

            transition tavern_gamble
                first darkCloakTavern
                accept Gamble
                if alive and gold >= 100
                do action betting { perform gamble; }
                then darkCloakTavern;

            transition tavern_profession
                first darkCloakTavern
                accept ChangeProfession
                if alive
                do action retraining { perform changeProfession; }
                then darkCloakTavern;

            transition tavern_forest
                first darkCloakTavern
                accept ReturnToTheForest
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

            transition bank_rob
                first bank
                accept RobTheBank
                if alive and class == CharacterClass::thievingSkills and fairy
                do action robbing { perform robTheBank; }
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
                accept VisitTheInn
                then inn;

            transition inn_flirt
                first inn
                accept FlirtAtTheInn
                if alive and not flirtedToday
                do action flirting { perform flirt; }
                then inn;

            transition inn_bard
                first inn
                accept ListenToTheBard
                if alive and not heardTheBard
                do action listening { perform listenToTheBard; }
                then inn;

            transition inn_bribe
                first inn
                accept BribeTheBartender
                if alive and level >= 2 and not bribed and gold >= BribePrice(level)
                do action bribing { perform bribeTheBartender; }
                then inn;

            transition inn_gems
                first inn
                accept TradeGems
                if alive and level >= 2 and gems >= town.inn.gemsPerStatPoint
                do action trading { perform tradeGems; }
                then inn;

            transition inn_room
                first inn
                accept BuyARoom
                if alive and not innRoom
                    and (charm >= town.inn.charmForAFreeRoom or gold >= RoomPrice(level))
                do action lodging { perform buyRoom; }
                then asleep;

            transition inn_town
                first inn
                accept ReturnToTown
                then townSquare;

            transition asleep_midnight
                first asleep
                accept NewDay
                do action midnight { perform newDay; }
                then townSquare;

            transition townSquare_slaughter
                first townSquare
                accept SlaughterOtherPlayers
                then slaughter;

            transition slaughter_attack
                first slaughter
                accept AttackAWarrior
                if playerFightsLeft > 0 and alive
                do action attacking { perform attack; }
                then slaughter;

            transition slaughter_slain
                first slaughter
                if not alive
                then slain;

            transition slaughter_town
                first slaughter
                accept ReturnToTown
                then townSquare;

            transition townSquare_otherPlaces
                first townSquare
                accept OtherPlaces
                then otherPlaces;

            transition otherPlaces_town
                first otherPlaces
                accept ReturnToTown
                then townSquare;

            // Midnight comes to a warrior in the square too, without the inn's protection.
            transition townSquare_midnight
                first townSquare
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

    // A warrior of the other sex, charming enough for the bard's hand.
    part heroine : Warrior {
        attribute :>> name = "Dame Devin";
        attribute :>> sex = Sex::female;
        attribute :>> class = CharacterClass::mysticalSkills;
        attribute :>> charm = 125;
    }

    // Whom the slaughter menu offers unless told otherwise: a first-level warrior
    // with a day's takings and a gem, out in the open.
    part rival : Warrior {
        attribute :>> name = "Bran the Braggart";
        attribute :>> experience = 50;
        attribute :>> gold = 120;
        attribute :>> gems = 1;
    }

    // A warrior at the top of the ladder, kitted out from both shops and a Death Knight master.
    part champion : Warrior {
        attribute :>> name = "Turgon's Equal";
        attribute :>> level = 12;
        attribute :>> experience = 10000000;
        attribute :>> hitPoints = 4500;
        attribute :>> maxHitPoints = 4500;
        attribute :>> strength = 1100;
        attribute :>> defense = 600;
        attribute :>> weaponTier = 13;
        attribute :>> weaponStrength = 800;
        attribute :>> armourTier = 13;
        attribute :>> armourDefense = 400;
        attribute :>> deathKnightPoints = 40;
        attribute :>> deathKnightUses = 10;
        attribute :>> favouredMove = Move::deathKnight;
    }
}

// What holds of a warrior, and what a solver can say about the dragon, the shops,
// the skills and the inn.
package LordOdds {
    private import ScalarValues::*;
    private import RealFunctions::floor;
    private import IntegerFunctions::max;
    private import TradeStudies::*;
    private import Lord::*;
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

    // A straight brawl, the dragon stomping every round and the warrior swinging first:
    // the warrior must survive one blow fewer than it takes to fell the dragon.
    requirement def OutlastTheDragon {
        subject warrior : Warrior;
        attribute dragonHitPoints : Integer = 15000;
        attribute dragonStrength : Integer = 2000;

        require constraint {
            warrior.hitPoints + Blow(dragonStrength, warrior.defense)
                > Blow(dragonStrength, warrior.defense) * BlowsToKill(warrior.strength, 0, dragonHitPoints)
        }
    }

    // The same brawl against the flaming breath alone, which no armour blunts.
    requirement def OutlastTheBreath {
        subject warrior : Warrior;
        attribute dragonHitPoints : Integer = 15000;
        attribute breath : Integer = 1000;

        require constraint {
            warrior.hitPoints + breath > breath * BlowsToKill(warrior.strength, 0, dragonHitPoints)
        }
    }

    requirement readyForTheDragon : ReadyForTheDragon;
    requirement outlastTheDragon : OutlastTheDragon;
    requirement outlastTheBreath : OutlastTheBreath;

    // The Death Knights' way: three blows a round while the uses last, and the dragon's
    // hardest answer every round, breath or stomp, whichever hurts more.
    requirement def SlayWithPowerMoves {
        subject warrior : Warrior;
        attribute dragonHitPoints : Integer = 15000;
        attribute dragonStrength : Integer = 2000;
        attribute breath : Integer = 1000;

        require constraint {
            warrior.deathKnightUses >= BlowsToKill(3 * warrior.strength, 0, dragonHitPoints)
        }
        require constraint {
            warrior.hitPoints + max(breath, Blow(dragonStrength, warrior.defense))
                > max(breath, Blow(dragonStrength, warrior.defense))
                    * BlowsToKill(3 * warrior.strength, 0, dragonHitPoints)
        }
    }

    requirement slayWithPowerMoves : SlayWithPowerMoves;

    part dragonFights {
        assert satisfy readyForTheDragon by champion;
        assert satisfy readyForTheDragon by hero;
        assert satisfy outlastTheDragon by champion;
        assert satisfy outlastTheBreath by champion;
        assert satisfy slayWithPowerMoves by champion;
    }

    // A hand a warrior may ask for: the right sex, the charm, and nobody's spouse yet.
    requirement def MayWed {
        subject warrior : Warrior;
        ref part favour : Flirt;

        require constraint { favour.weds }
        require constraint { warrior.sex == favour.suitor }
        require constraint { warrior.charm >= favour.charmNeeded }
        require constraint { warrior.spouse == Spouse::nobody }
    }

    requirement mayWedViolet : MayWed { ref part :>> favour = town.inn.violet.marryHer; }
    requirement mayWedTheBard : MayWed { ref part :>> favour = town.inn.sethAble.marryHim; }

    part weddings {
        assert satisfy mayWedViolet by hero;
        assert satisfy mayWedViolet by heroine;
        assert satisfy mayWedTheBard by heroine;
    }

    // A sleeper at the inn may be attacked only by a bribed warrior at most a level above.
    requirement def MaySlaughter {
        subject attacker : Warrior;
        ref part sleeper : Warrior;

        require constraint { attacker.alive and attacker.playerFightsLeft > 0 }
        require constraint { sleeper.alive }
        require constraint { attacker != sleeper }
        require constraint { not sleeper.innRoom or (attacker.bribed and sleeper.level >= attacker.level - 1) }
    }

    requirement maySlaughterTheRival : MaySlaughter { ref part :>> sleeper = rival; }

    part slaughter {
        assert satisfy maySlaughterTheRival by hero;
        assert satisfy maySlaughterTheRival by champion;
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

    // The same question the Death Knights' way: three blows a round, the stomp answering.
    analysis def ToughEnoughWithPowerMoves {
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
            require constraint { 3 * strength * roundsToKill >= 15000 }
            require constraint { 3 * strength * (roundsToKill - 1) < 15000 }
            require constraint { hitPoints + (2000 - defense) > (2000 - defense) * roundsToKill }

            in calc :>> eval { hitPoints }
        }
    }

    // The fewest Death Knight lessons for a day's worth of power moves, a level a day
    // being the other way to a point: four points buy one use.
    analysis def PowerMoves {
        attribute usesWanted : Integer;
        attribute levelsGained : Integer;
        attribute lessons : Integer;

        objective fewestLessons : MinimizeObjective {
            subject :>> selectedAlternative;
            require constraint { usesWanted == 3 }
            require constraint { levelsGained >= 0 and levelsGained <= 11 }
            require constraint { lessons >= 0 }
            require constraint { levelsGained + lessons <= 40 }
            require constraint { levelsGained + lessons >= 4 * usesWanted }
            require constraint { levelsGained <= 5 }

            in calc :>> eval { lessons }
        }
    }

    // The fewest gems that raise defense by the points asked, two gems a point at the inn.
    analysis def GemsForDefense {
        attribute pointsWanted : Integer;
        attribute gems : Integer;

        objective fewestGems : MinimizeObjective {
            subject :>> selectedAlternative;
            require constraint { pointsWanted == 5 }
            require constraint { gems >= 0 }
            require constraint { gems >= 2 * pointsWanted }

            in calc :>> eval { gems }
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

    // The most a warrior of a given level pays for a night's rest and the bartender's silence.
    calc def NightAtTheInn {
        in level : Integer;

        RoomPrice(level) + BribePrice(level)
    }
}

// How the game is shown: the menu as a state diagram, the fights as flows, the town
// as a tree, and the news screen as a document with the shops', masters' and forest's lists.
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

    view dragonFight : ActionFlowView {
        expose LordPlay::Warrior::fightDragon;
    }

    view slaughter : ActionFlowView {
        expose LordPlay::Warrior::attack;
    }

    view flirting : ActionFlowView {
        expose LordPlay::Warrior::flirt;
    }

    view theTown {
        expose Lord::Town;
        render asTreeDiagram;
    }

    view theForest {
        expose Lord::Forest;
        render asTreeDiagram;
    }

    // The document's query roots: a table's query is bound to a usage.
    part kingArthurs : Lord::WeaponShop;
    part abduls : Lord::ArmourShop;
    part turgons : Lord::TrainingHall;
    part violets : Lord::Barmaid;
    part seths : Lord::Bard;
    part woods : Lord::Forest;

    // A part's items, one row each, as its definition declares them.
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

    // The forest's monsters of one level, one row each.
    calc def Haunting :> Query {
        in forest : Element;
        in level : String;
        WhereFeature(
            source = WhereFeature(
                source = WhereType(
                    source = Descendants(
                        source = RelatedElements(
                            source = forest,
                            relationshipKind = "typing",
                            direction = "outgoing",
                            maxDepth = 1
                        ),
                        maxDepth = 2
                    ),
                    type = "PartUsage"
                ),
                'feature' = "hitPoints",
                operator = ">",
                value = "0"
            ),
            'feature' = "level",
            operator = "==",
            value = level
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

    calc def MasterList :> Query {
        in hall : Element;
        Project(
            source = Stock(shop = hall),
            properties = ("teaches", "experienceRequired", "hitPoints", "strength", "defense"),
            columns = (Column(name = "master", expression = Lord::Master::name))
        )
    }

    calc def FavourList :> Query {
        in local : Element;
        Project(
            source = Stock(shop = local),
            properties = ("charmNeeded", "experiencePerLevel"),
            columns = (Column(name = "favour", expression = Lord::Flirt::name))
        )
    }

    calc def MonsterList :> Query {
        in forest : Element;
        in level : String;
        Project(
            source = OrderBy(
                source = Haunting(forest = forest, level = level),
                property = "experience",
                direction = "ascending",
                missing = "last",
                multiple = "error"
            ),
            properties = ("weapon", "strength", "hitPoints", "gold", "experience"),
            columns = (Column(name = "monster", expression = Lord::Monster::name))
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

    // The Daily Happenings: the hero's standing as the session holds it, and the realm's lists.
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
        part masters : Table {
            attribute redefines caption = "Turgon's Warrior Training";
            calc rows : MasterList {
                in hall = turgons;
            }
        }
        part violet : Table {
            attribute redefines caption = "Violet's favours";
            calc rows : FavourList {
                in local = violets;
            }
        }
        part sethAble : Table {
            attribute redefines caption = "Seth Able's favours";
            calc rows : FavourList {
                in local = seths;
            }
        }
        part forestOne : Table {
            attribute redefines caption = "The forest, level one";
            calc rows : MonsterList {
                in forest = woods;
                in level = "1";
            }
        }
        part forestTwelve : Table {
            attribute redefines caption = "The forest, level twelve";
            calc rows : MonsterList {
                in forest = woods;
                in level = "12";
            }
        }
    }
}
