package Prefixed {
    metadata def <safe> Safety;
    metadata def Reviewed;
    #Safety part def Car;
    abstract #Reviewed #Safety part def Vehicle;
    #safe part car : Car;
    private ref #Safety part spare : Car;
    variant #Reviewed part option : Car;
    end #Safety part wheel : Car;
    #Reviewed package Notes;
    #Safety dependency from Car to Vehicle;
    constraint def Stopped;
    action def Move;
    part context : Car {
        #Safety assert constraint stopped : Stopped;
        #Reviewed assert not constraint moving : Stopped;
        #Safety perform action go : Move;
    }
    requirement def Braking {
        subject #Safety brakes : Car;
        assume #Reviewed constraint { true }
        require #Safety constraint { true }
        assume #Reviewed constraint stopped : Stopped;
        require #Safety constraint braked[1] = true;
    }
}
