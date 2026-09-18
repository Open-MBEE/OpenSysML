package Modifiers {
    private part def Hidden;
    public part def Shown;
    abstract part def Base;
    part def Derived :> Base;
    variation part def Choice;
}
