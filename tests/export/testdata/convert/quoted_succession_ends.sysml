package QuotedSuccessionEnds {
    action def 'provide power' {
        action engineStarted;
        action 'generate torque';
        action 'amplify torque';
        first start then engineStarted;
        first engineStarted then 'generate torque';
        first 'generate torque' then 'amplify torque';
        succession s first 'amplify torque' then done;
    }
    action def 'drive' {
        action 'enter vehicle';
        then 'start engine';
        action 'start engine';
        succession first 'start engine' then 'drive vehicle';
        action 'drive vehicle';
    }
}
