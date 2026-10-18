package CoffeeMaker {
    action def Brew {
        action heatWater;
        then action grindBeans;
        then action pourWater;
    }
}
