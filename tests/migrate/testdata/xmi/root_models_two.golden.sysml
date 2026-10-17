package Alpha {
    part def Sensor;
}
package Beta {
    part def Station {
        part Sensor : $::Alpha::Sensor;
        part Alpha : $::Alpha::Sensor;
    }
    part def Relay {
        part sensor : Alpha::Sensor;
    }
}
