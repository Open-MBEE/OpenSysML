package InheritedMembers {
    port def CmdPort {
        in attribute cmd : ScalarValues::Integer;
    }

    part def Logic {
        port cmdIn : CmdPort;
    }

    part def Controller {
        attribute gain : ScalarValues::Real;
        private attribute secret : ScalarValues::Real;
        port cmdIn : CmdPort;
        part logic : Logic;
        binding delegate bind cmdIn = logic.cmdIn;
    }

    part system {
        part controller : Controller;
        part tuned : Controller {
            attribute :>> gain = 2.0;
        }
    }
}
