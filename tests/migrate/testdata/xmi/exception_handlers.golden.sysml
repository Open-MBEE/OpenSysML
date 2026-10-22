package Model {
    package Imaging {
        item def CaptureFault;
        occurrence def Camera {
            action def Capture {
                first start then 'expose';
                action 'expose';
                first 'expose' then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about final;
            }
            action def Inspect {
                view Inspect : StandardViewDefinitions::ActionFlowView {
                    expose Imaging::Camera::Inspect;
                    @DiagramLayout::Canvas { unit = "px"; width = 620; height = 360; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Inspect::start { x = 85; y = 40; width = 15; height = 15; }
                    metadata DiagramLayout::Layout about capture { x = 21; y = 100; width = 142; height = 33; }
                    metadata DiagramLayout::Layout about store { x = 21; y = 200; width = 142; height = 33; }
                    metadata DiagramLayout::Layout about final { x = 82; y = 300; width = 20; height = 20; }
                    metadata DiagramLayout::Layout about 'Image Capture Fails' { x = 240; y = 40; width = 180; height = 130; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Inspect::'Image Capture Fails'::retry { x = 260; y = 70; width = 142; height = 33; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Inspect::'Image Capture Fails'::'log failure' { x = 260; y = 120; width = 142; height = 33; }
                    metadata DiagramLayout::Layout about 'Position Not Correct' { x = 240; y = 200; width = 180; height = 130; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Inspect::'Position Not Correct'::adjust { x = 260; y = 230; width = 142; height = 33; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Inspect::'Position Not Correct'::recapture { x = 260; y = 280; width = 142; height = 33; }
                    metadata DiagramLayout::Route about 'start to capture' { points = (92, 55, 92, 100); }
                    metadata DiagramLayout::Route about 'capture to store' { points = (92, 133, 92, 200); }
                    metadata DiagramLayout::Route about 'store to final' { points = (92, 233, 92, 300); }
                    metadata DiagramLayout::Route about Imaging::Camera::Inspect::'Image Capture Fails'::'retry to log failure' { points = (331, 103, 331, 120); }
                    metadata DiagramLayout::Route about Imaging::Camera::Inspect::'Position Not Correct'::'adjust to recapture' { points = (331, 263, 331, 280); }
                    render Views::asInterconnectionDiagram;
                }
                in ref context : Camera[1];
                succession 'start to capture' first start then capture;
                action capture : Capture;
                succession 'capture to store' first capture then store;
                action store;
                succession 'store to final' first store then final;
                action final terminate;
                /* not migrated: ExceptionHandler (_eh_h1) — SysML v2 has no exception handler: UML runs the handler body 'Image Capture Fails' only when its protected node 'capture' raises a matching exception of type 'CaptureFault'; the body is written as a reference action usage the activity does not perform */
                ref action 'Image Capture Fails' {
                    first start then retry;
                    action retry;
                    succession 'retry to log failure' first retry then 'log failure';
                    action 'log failure';
                    metadata MigrationMetadata::SynthesizedName about 'retry to log failure';
                }
                /* not migrated: ExceptionHandler (_eh_h2) — SysML v2 has no exception handler: UML runs the handler body 'Position Not Correct' only when its protected node 'capture' raises a matching exception; no exception type: catches nothing in UML */
                ref action 'Position Not Correct' {
                    first start then adjust;
                    action adjust;
                    succession 'adjust to recapture' first adjust then recapture;
                    action recapture;
                    metadata MigrationMetadata::SynthesizedName about 'adjust to recapture';
                }
                metadata MigrationMetadata::SynthesizedName about 'start to capture', 'capture to store', 'store to final', final;
            }
        }
    }
}
