package Model {
    package Imaging {
        occurrence def Camera {
            action def Acquire {
                view Acquire : StandardViewDefinitions::ActionFlowView {
                    expose Imaging::Camera::Acquire;
                    @DiagramLayout::Canvas { unit = "px"; width = 500; height = 360; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Acquire::start { x = 85; y = 90; width = 15; height = 15; }
                    metadata DiagramLayout::Layout about position { x = 21; y = 117; width = 142; height = 33; }
                    metadata DiagramLayout::Layout about final { x = 82; y = 200; width = 20; height = 20; }
                    metadata DiagramLayout::Layout about 'Position Not Correct' { x = 300; y = 100; width = 180; height = 160; }
                    metadata DiagramLayout::Layout about 'Position Not Correct'::start { x = 379; y = 127; width = 15; height = 15; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Acquire::'Position Not Correct'::adjust { x = 320; y = 152; width = 142; height = 33; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Acquire::'Position Not Correct'::final { x = 381; y = 220; width = 20; height = 20; }
                    metadata DiagramLayout::Layout about check { x = 21; y = 260; width = 142; height = 33; }
                    metadata DiagramLayout::Route about 'start to position' { points = (92, 105, 92, 117); }
                    metadata DiagramLayout::Route about 'position to check' { points = (120, 150, 120, 260); }
                    metadata DiagramLayout::Route about 'check to Position Not Correct' { points = (163, 276, 300, 240); }
                    metadata DiagramLayout::Route about 'Position Not Correct to final' { points = (300, 210, 102, 210); }
                    metadata DiagramLayout::Route about Imaging::Camera::Acquire::'Position Not Correct'::'start to adjust' { points = (386, 142, 386, 152); }
                    metadata DiagramLayout::Route about Imaging::Camera::Acquire::'Position Not Correct'::'adjust to final' { points = (391, 185, 391, 220); }
                    metadata DiagramLayout::Route about 'position.result to check.value' { points = (40, 150, 40, 260); }
                    render Views::asInterconnectionDiagram;
                }
                succession 'start to position' first start then position;
                action position {
                    out result[1] = 2.5;
                }
                succession 'position to check' first position then check;
                action check {
                    in value[1];
                    rep language "English" /* check the value*/
                }
                succession 'check to Position Not Correct' first check then 'Position Not Correct';
                action final terminate;
                action 'Position Not Correct' {
                    succession 'start to adjust' first start then adjust;
                    action adjust;
                    succession 'adjust to final' first adjust then final;
                    action final terminate;
                    metadata MigrationMetadata::SynthesizedName about 'start to adjust', 'adjust to final', final;
                }
                succession 'Position Not Correct to final' first 'Position Not Correct' then final;
                flow 'position.result to check.value' from position.result to check.value;
                metadata MigrationMetadata::SynthesizedName about 'start to position', 'position to check', 'check to Position Not Correct', final, 'Position Not Correct to final', 'position.result to check.value';
            }
            state def Modes {
                view Modes : StandardViewDefinitions::StateTransitionView {
                    expose Imaging::Camera::Modes;
                    @DiagramLayout::Canvas { unit = "px"; width = 240; height = 340; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Modes::start { x = 50; y = 50; width = 20; height = 20; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Modes::Idle { x = 10; y = 100; width = 100; height = 40; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Modes::done { x = 50; y = 300; width = 20; height = 20; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Modes::Active { x = 10; y = 160; width = 220; height = 120; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Modes::Active::regions::b::start { x = 150; y = 180; width = 16; height = 16; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Modes::Active::regions::a::a1 { x = 20; y = 200; width = 80; height = 40; }
                    metadata DiagramLayout::Layout about Imaging::Camera::Modes::Active::regions::b::b1 { x = 130; y = 220; width = 80; height = 40; }
                    metadata DiagramLayout::Route about 'start then Idle' { points = (60, 70, 60, 100); }
                    metadata DiagramLayout::Route about 'Active then done' { points = (60, 280, 60, 300); }
                    render Views::asInterconnectionDiagram;
                }
                /* not migrated: Pseudostate (_ps_dangling) — no transition leaves the initial pseudostate */
                /* not migrated: Pseudostate (_ps_dup) — a region has one initial pseudostate; (_ps_init) is written as it */
                transition 'start then Idle' first start then Idle;
                state Idle;
                state Active {
                    entry; then regions;
                    state regions parallel {
                        state a {
                            /* every initial pseudostate of the region enters another region: nothing enters this one */
                            state a1;
                        }
                        state b {
                            transition 'start then b1' first start then b1;
                            state b1;
                            metadata MigrationMetadata::SynthesizedName about 'start then b1';
                        }
                    }
                    transition first regions then done;
                    metadata MigrationMetadata::SynthesizedName about regions;
                }
                transition first Idle then Active;
                transition 'Active then done' first Active then done;
                metadata MigrationMetadata::SynthesizedName about 'start then Idle', 'Active then done';
            }
        }
    }
}
