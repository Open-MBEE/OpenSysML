package 'Quoted Names' {
    attribute def 'Mass (kg)';
    part def <'1'> 'Rover Model' {
        attribute 'dry mass' : 'Mass (kg)';
        constraint 'mass limit' {
            'dry mass' > 0
        }
        assert constraint 'is light' : 'mass limit';
    }
    alias 'the rover' for 'Rover Model';
    part 'my rover' : 'Rover Model' :> 'Rover Model';
    part 'off model' : 'Not Declared Here';
    part def 'state';
    part 'a state' : 'state';
}
package Consumer {
    private import 'Quoted Names'::*;
    part r : 'Quoted Names'::'Rover Model';
    dependency from r to 'Quoted Names'::'Rover Model';
}
