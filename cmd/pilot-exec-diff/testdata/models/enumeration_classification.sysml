package Enums {
    private import ScalarValues::*;

    enum def Level :> Integer { low = 1; high = 3; }
    enum def Color { red; green; blue; }
    attribute def Even :> Integer;

    attribute three : Integer = 3;
    attribute lvl : Level = Level::high;
    attribute held : Level = three;
    attribute c : Color = Color::red;

    attribute threeIsLevel = 3 istype Level;
    attribute twoIsLevel = 2 istype Level;
    attribute threeAtLevel = 3 @ Level;
    attribute threeHasLevel = 3 hastype Level;
    attribute threeHasInteger = 3 hastype Integer;
    attribute threeAsLevel = 3 as Level;
    attribute twoAsLevel = 2 as Level;
    attribute castHasLevel = (3 as Level) hastype Level;
    attribute highIsLevel = Level::high istype Level;
    attribute highHasLevel = Level::high hastype Level;
    attribute highHasInteger = Level::high hastype Integer;
    attribute highIsInteger = Level::high istype Integer;
    attribute highEqThree = Level::high == 3;
    attribute highPlusOne = Level::high + 1;
    attribute lvlHasLevel = lvl hastype Level;
    attribute lvlHasInteger = lvl hastype Integer;
    attribute heldHasLevel = held hastype Level;
    attribute redIsColor = Color::red istype Color;
    attribute redHasColor = Color::red hastype Color;
    attribute cHasColor = c hastype Color;
    attribute threeIsColor = 3 istype Color;
    attribute redIsLevel = Color::red istype Level;
    attribute fiveAsEven = 5 as Even;
}
