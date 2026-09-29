package DeferIsAName {
    // `defer` is not reserved: SysML v2 has no deferral notation, and the
    // OpenSysML `defer <event>;` extension that once used the word was removed.
    attribute def Setting {
        attribute defer : ScalarValues::Boolean;
    }

    action def Plan {
        action defer;
        then done;
    }

    state def Controller {
        attribute defer : ScalarValues::Integer;
        entry; then defer;
        state defer;
        state ready;
        transition first defer if defer == 1 then ready;
        transition first ready accept Ping do assign defer := defer + 1 then defer;
    }

    state def Monitor {
        attribute setting : ScalarValues::Integer;
        defer references setting;
    }

    part def Holder {
        part defer : Setting;
        attribute flag = defer.defer;
    }
}
