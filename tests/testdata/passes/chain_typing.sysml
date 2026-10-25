package ChainTyping {
    port def Link;
    part def Wheel;
    part def Rig {
        part wheel : Wheel;
        port socket : Link;
    }
    part rig : Rig;
    ref spare : rig.wheel;
    part mounted : rig.wheel;
    attribute size : rig.wheel;
    action spin : rig.wheel;
}
