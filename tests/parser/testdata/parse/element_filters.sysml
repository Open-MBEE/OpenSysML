package Filters {
    metadata def Safety {
        attribute isMandatory : Boolean;
    }

    part vehicle {
        part seatBelt {@Safety{isMandatory = true;}}
        part keylessEntry;
    }

    package 'Safety Features' {
        public import vehicle::**;
        filter @Safety;
    }

    package 'Mandatory Safety Features' {
        public import vehicle::**;
        filter @Safety and (as Safety).isMandatory;
    }

    package 'Metaclassified' {
        public import vehicle::**;
        filter @@SysML::PartUsage;
        filter not (@Safety) or @Safety xor false implies true;
    }

    package 'Filtered Imports' {
        public import vehicle::*[@Safety];
        public import vehicle::**[@Safety and (as Safety).isMandatory];
    }

    view 'safety view' {
        expose vehicle::**[@Safety];
        expose vehicle::**[not (@Safety)];
    }
}
