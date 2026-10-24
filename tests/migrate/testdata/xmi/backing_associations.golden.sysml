package Model {
    package Optics {
        constraint def Gain {
            in attribute input : ScalarValues::Real[1];
            in attribute output : ScalarValues::Real[1];
            output == 2 * input
        }
        attribute def Mass;
        part def Sensor {
            attribute mass : Mass;
            part amplifier : Amplifier;
        }
        part def Amplifier;
        connection def AmplifierToSensor {
            end amplifier : Amplifier crosses sensor.amplifier;
            end sensor : Sensor;
            metadata MigrationMetadata::SynthesizedName about sensor;
        }
        part amplifier : Amplifier;
        part sensor : Sensor;
        connection 'amplifier to sensor' : AmplifierToSensor connect amplifier to sensor;
        view Sensors {
            expose Sensor;
            expose Mass;
            expose Amplifier;
            expose AmplifierToSensor;
            @DiagramLayout::Canvas { unit = "px"; width = 400; height = 300; }
            metadata DiagramLayout::Layout about Sensor { x = 140; y = 20; width = 120; height = 50; }
            metadata DiagramLayout::Layout about Mass { x = 20; y = 180; width = 120; height = 50; }
            metadata DiagramLayout::Layout about Amplifier { x = 260; y = 180; width = 120; height = 50; }
            metadata DiagramLayout::Route about Optics::Sensor::mass { points = (160, 70, 80, 180); }
            metadata DiagramLayout::Route about AmplifierToSensor { points = (320, 180, 240, 70); }
            render Views::asTreeDiagram;
        }
        metadata MigrationMetadata::SynthesizedName about AmplifierToSensor, 'amplifier to sensor';
    }
}
