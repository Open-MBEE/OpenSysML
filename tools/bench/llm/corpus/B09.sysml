package TrafficLight {
    attribute def TimerExpired;
    attribute def PedestrianRequest;

    state def LightStates {
        entry; then operating;

        state operating {
            entry; then red;
            state red;
            state green;
            state yellow;
            transition red_to_green first red accept TimerExpired then green;
            transition green_to_yellow first green accept TimerExpired then yellow;
            transition yellow_to_red first yellow accept TimerExpired then red;
        }

        state pedestrianCrossing;

        transition operating_to_crossing first operating accept PedestrianRequest then pedestrianCrossing;
        transition crossing_to_operating first pedestrianCrossing accept TimerExpired then operating;
    }
}
