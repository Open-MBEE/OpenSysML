package Documented {
    part def Wheel {
        doc /* one wheel */
        attribute radius : Real;
        comment /* about the hub */
        port hub : Hub;
    }
    port def Hub {
        doc /* the hub port */
        out item stuff;
    }
    part wheel : Wheel {
        doc /* a wheel */
        attribute serial;
    }
}
