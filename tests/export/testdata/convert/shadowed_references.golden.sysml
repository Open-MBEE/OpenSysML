package Shadowing {
    attribute def Field;
    attribute 'packet data field' : Field;
    part def Packet {
        attribute 'data field' : Field;
        part payload;
    }
    part def 'Thermal Packet' :> Packet {
        attribute 'packet data field' :>> Shadowing::'packet data field';
        attribute 'data field' :>> Packet::'data field' {
            attribute 'data field' : Field;
        }
    }
    part payload;
    part def Bus {
        part payload :> Shadowing::payload;
        part cargo :> Shadowing::payload;
        ref part carried references payload;
        part def Packet;
        part wrapped : Shadowing::Packet;
        part raw : Packet;
        connection link connect wrapped.payload to Shadowing::payload;
        requirement def Budget;
        assert satisfy requirement budgeted : Shadowing::Budget by payload;
    }
    requirement def Budget;
    requirement budget : Budget;
    satisfy budget by payload;
    part bus : Bus;
    analysis def Study { subject vehicle : Bus; }
    analysis study : Study {
        subject :>> vehicle :> bus;
    }
    analysis deeper :> study {
        attribute load = vehicle.cargo;
    }
    part def Frame {
        part def Frame;
        attribute 'packet data field' : Shadowing::Field;
        ref part outer : Shadowing::Frame;
        ref part inner : Frame;
        part def Field;
    }
    package Lib {
        part def Cell;
    }
    package Consumer {
        package Lib {
            part def Cell;
        }
        private import Shadowing::Lib::Cell;
        part cell : Cell;
        part inner : Lib::Cell;
    }
    package Exposed {
        package Lib;
        private import Shadowing::Lib::*;
    }
}
