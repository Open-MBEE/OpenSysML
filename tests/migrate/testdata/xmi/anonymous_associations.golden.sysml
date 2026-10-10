package Fleet {
    part def Car {
        ref part wheels : Wheel[4];
        ref part driver : Driver;
    }
    part def Wheel {
        ref part car : Car;
    }
    connection def WheelToCar {
        end wheels : Wheel crosses car.wheels;
        end car : Car crosses wheels.car;
    }
    part def Driver;
    connection def DriverToCar {
        end driver : Driver crosses car.driver;
        end [0..*] ref car : Car;
        metadata MigrationMetadata::SynthesizedName about car;
    }
    part def Sensor {
        ref part reading : Reading[0..*];
    }
    part def Reading;
    connection def ReadingToSource {
        end [0..*] ref reading : Reading;
        end source;
    }
    part def Logger {
        attribute sink;
    }
    connection def SinkToLogger {
        end sink;
        end logger : Logger;
        metadata MigrationMetadata::SynthesizedName about logger;
    }
    part def Tank {
        ref part Pump : Fleet::Pump;
    }
    part def Pump;
    connection def PumpToTank {
        end Pump : Fleet::Pump crosses tank.Pump;
        end tank : Tank;
        metadata MigrationMetadata::SynthesizedName about tank;
    }
    part def Console {
        ref part operator :> Operator;
    }
    part Operator;
    connection def Operates {
        end operator :> Operator;
        end console : Console;
        metadata MigrationMetadata::SynthesizedName about console;
    }
    part console : Console;
    connection operates : Operates connect Operator to console;
    part wheel : Wheel;
    part car : Car;
    connection 'wheel to car' : WheelToCar connect wheel to car;
    part driver : Driver;
    connection 'driver to car' : DriverToCar connect driver to car;
    part pump : Pump;
    part tank : Tank;
    connection 'pump to tank' : PumpToTank connect pump to tank;
    view Wheels {
        expose Car;
        expose Wheel;
        expose WheelToCar;
        render Views::asTreeDiagram;
    }
    metadata MigrationMetadata::SynthesizedName about WheelToCar, DriverToCar, ReadingToSource, SinkToLogger, PumpToTank;
}
