package CrossEnds {
    part def Device;
    part def Hub { part mainSwitch : Device; }
    part def Node { part sensorFeed : Device; }
    connection def DeviceConnection {
        end hub : Hub;
        end device : Node;
    }
    part def A;
    part def B;
    part def C;
    connection def Assoc {
        end a_cross[1] ref a[1] : A;
        end b_cross[0..2] ref b[1] : B;
        end c_cross[*] nonunique ordered ref c[1] : C;
    }
    part system {
        part h : Hub;
        part n : Node;
        connection link : DeviceConnection connect ([1] hub ::> h.mainSwitch, [1] device ::> n.sensorFeed);
    }
}
