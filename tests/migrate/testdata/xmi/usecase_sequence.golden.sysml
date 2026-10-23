package Model {
    part Operator;
    item def PumpDown {
        attribute target : ScalarValues::Real;
    }
    part def 'Sample Stage' {
        attribute position : ScalarValues::Real default = 0.0;
        abstract action move {
            in 'to' : ScalarValues::Real[1];
        }
    }
    part def 'Vacuum Subsystem' {
        attribute pressure : ScalarValues::Real;
    }
    part def Image;
    part def 'Display System';
    use case 'Acquire Images' {
        occurrence def 'Acquire Images' {
            ref part operator :> Operator {
                event occurrence moveSent;
            }
            ref part stage : 'Sample Stage' {
                event occurrence moveReceived {
                    perform action moveCall ::> 'Sample Stage'::move { in 'to'[1] = 2.5; }
                }
                then event occurrence pumpDownSent;
            }
            ref part vacuum : 'Vacuum Subsystem' {
                event occurrence pumpDownReceived;
                then event occurrence newSent;
            }
            ref part image : Image {
                event occurrence newReceived;
                then event occurrence recordSent;
                then event occurrence showSent;
            }
            part log {
                event occurrence recordReceived;
            }
            ref part 'display System' : 'Display System' {
                event occurrence showReceived;
                then event occurrence confirmSent;
            }
            ref part operator2 :> Operator {
                event occurrence confirmReceived;
            }
            message move from operator.moveSent to stage.moveReceived;
            message pumpDown of pumpDownPayload : PumpDown = new PumpDown(target = 0.1) from stage.pumpDownSent to vacuum.pumpDownReceived;
            message 'new' from vacuum.newSent to image.newReceived;
            message record from image.recordSent to log.recordReceived;
            message show from image.showSent to 'display System'.showReceived;
            message confirm from 'display System'.confirmSent to operator2.confirmReceived;
            view 'Acquire Images' : StandardViewDefinitions::SequenceView {
                expose Model::'Acquire Images'::'Acquire Images';
                @DiagramLayout::Canvas { unit = "px"; width = 1240; height = 520; }
                metadata DiagramLayout::Layout about operator { x = 20; y = 20; width = 120; height = 50; }
                metadata DiagramLayout::Layout about stage { x = 200; y = 20; width = 120; height = 50; }
                metadata DiagramLayout::Layout about vacuum { x = 380; y = 20; width = 140; height = 50; }
                metadata DiagramLayout::Layout about image { x = 580; y = 20; width = 100; height = 50; }
                metadata DiagramLayout::Layout about log { x = 740; y = 20; width = 100; height = 50; }
                metadata DiagramLayout::Layout about 'display System' { x = 900; y = 20; width = 140; height = 50; }
                metadata DiagramLayout::Layout about operator2 { x = 1100; y = 20; width = 120; height = 50; }
                metadata DiagramLayout::Route about move { points = (80, 120, 260, 120); }
                metadata DiagramLayout::Route about pumpDown { points = (260, 180, 450, 180); }
                metadata DiagramLayout::Route about 'new' { points = (450, 240, 630, 240); }
                metadata DiagramLayout::Route about record { points = (630, 300, 790, 300); }
                metadata DiagramLayout::Route about show { points = (630, 360, 970, 360); }
                metadata DiagramLayout::Route about confirm { points = (970, 420, 1160, 420); }
            }
            metadata MigrationMetadata::SynthesizedName about 'display System', operator2;
        }
    }
}
