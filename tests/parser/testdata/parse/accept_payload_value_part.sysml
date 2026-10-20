package Triggered {
    private import ScalarValues::*;
    private import Observation::*;
    private import Triggers::*;
    part controller {
        in level : Real;
        attribute threshold : Real;
        action {
            accept : ChangeSignal = Triggers::TriggerWhen({ level > threshold });
            accept changed : ChangeSignal = TriggerWhen({ level > threshold });
            accept when level > threshold;
        }
    }
}
