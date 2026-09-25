package ConnectorEnds {
    class Engine;
    class Tank {
        feature main : Engine;
        feature x;
    }
    class Trigger;
    class Vehicle {
        feature eng : Engine;
        feature tanks : Tank;
        feature a : Tank;
        feature b;
        feature trigger : Trigger;
        feature transitionLink;
        connector eng to tanks.main;
        connector a ::> a.x to b;
        connector [1] bead ::> tanks.x to [0..1] b;
        connector r references eng to tanks.main;
        private connector [0..1] transitionLink to [1..*] trigger;
        connector link from eng to tanks.main;
        connector all from eng to tanks.main;
        connector all eng to tanks.main;
        connector from eng to tanks.main;
        connector (eng, tanks.main, trigger);
    }
}
