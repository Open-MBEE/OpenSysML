package Model {
    package Blocks {
        part def Controller {
            part sensors : Sensor;
        }
        part def Sensor {
            ref part controller : Controller;
        }
        connection def SensorToController {
            end sensors : Sensor crosses controller.sensors;
            end controller : Controller crosses sensors.controller;
        }
        part sensor : Sensor;
        part controller : Controller;
        connection 'sensor to controller' : SensorToController connect sensor to controller;
        view 'Pair BDD' {
            expose Controller;
            expose Sensor;
            expose SensorToController;
            @DiagramLayout::Canvas { unit = "px"; width = 400; height = 300; }
            metadata DiagramLayout::Layout about Controller { x = 40; y = 20; width = 120; height = 50; }
            metadata DiagramLayout::Layout about Sensor { x = 40; y = 200; width = 120; height = 50; }
            metadata DiagramLayout::Route about SensorToController { points = (100, 200, 100, 130, 100, 70); }
            render Views::asTreeDiagram;
        }
        view 'Sensor BDD' {
            expose Controller;
            expose Sensor;
            expose SensorToController;
            @DiagramLayout::Canvas { unit = "px"; width = 400; height = 300; }
            metadata DiagramLayout::Layout about Controller { x = 240; y = 20; width = 120; height = 50; }
            metadata DiagramLayout::Layout about Sensor { x = 40; y = 200; width = 120; height = 50; }
            metadata DiagramLayout::Route about SensorToController { points = (160, 225, 300, 225, 300, 70); }
            render Views::asTreeDiagram;
        }
        metadata MigrationMetadata::SynthesizedName about SensorToController, 'sensor to controller';
    }
}
