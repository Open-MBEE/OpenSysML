package UnnamedChildren {
    port def CmdPort {
        in attribute cmd : ScalarValues::Integer;
    }
    part def Logic {
        port cmdIn : CmdPort;
        port cmdOut : ~CmdPort;
    }
    part def Controller {
        port cmdIn : CmdPort;
        part logic : Logic;
        part monitor : Logic;
        bind cmdIn = logic.cmdIn;
        binding toMonitor bind cmdIn = monitor.cmdIn;
        bind logic.cmdOut = monitor.cmdIn;
        connect logic.cmdOut to monitor.cmdIn;
        part : Logic;
    }
}
