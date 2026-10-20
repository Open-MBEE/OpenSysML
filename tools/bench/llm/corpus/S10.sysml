package Robot {
    private import ScalarValues::*;

    item def MotorCommand;
    item def SensorReading;

    port def CommandPort {
        in item command : MotorCommand;
    }

    port def SensorPort {
        out item reading : SensorReading;
    }

    part def Controller {
        port motorCommands : ~CommandPort;
        port sensorInput : ~SensorPort;
    }

    part def Motor {
        attribute maxTorqueNewtonMeters : Real;
        port commands : CommandPort;
    }

    part def DistanceSensor {
        port readings : SensorPort;
    }

    part def Robot {
        part controller : Controller;
        part leftMotor : Motor;
        part rightMotor : Motor;
        part frontSensor : DistanceSensor;

        connect controller.motorCommands to leftMotor.commands;
        connect controller.motorCommands to rightMotor.commands;
        connect frontSensor.readings to controller.sensorInput;
    }
}
