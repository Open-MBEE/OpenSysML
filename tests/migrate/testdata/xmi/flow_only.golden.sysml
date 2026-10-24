package Model {
    package Power {
        part def Pwr;
        part def Cmd;
        part def Tlm;
        port def iPower {
            out item pwr : Pwr[1];
        }
        port def iBus {
            out item cmd : Cmd[1];
            in item tlm : Tlm[1];
        }
        part def Supply {
            port 'pwr out' : iPower;
        }
        part def Load {
            port 'pwr in' : ~iPower;
        }
        part def Ctrl {
            port bus : iBus;
        }
        part def Unit {
            port bus : ~iBus;
        }
        part def Sys {
            part supply : Supply;
            part load : Load;
            part spare : Load;
            part aux : Load;
            part ctrl : Ctrl;
            part unit : Unit;
            flow 'supply.pwr out to load.pwr in' of Pwr from supply.'pwr out'.pwr to load.'pwr in'.pwr;
            flow cmd of Cmd from ctrl.bus.cmd to unit.bus.cmd;
            flow tlm of Tlm from unit.bus.tlm to ctrl.bus.tlm;
            connection 'spare feed' connect supply.'pwr out' to spare.'pwr in';
            flow 'spare pwr' of Pwr from supply.'pwr out'.pwr to spare.'pwr in'.pwr;
            connection 'supply.pwr out to aux.pwr in' connect supply.'pwr out' to aux.'pwr in';
            /* item flow of Cmd from supply.'pwr out' to aux.'pwr in' not migrated: no flow property typed by Cmd on both ends of the realizing connector */
            /* not migrated: «ItemFlow» InformationFlow 'aux' — no flow property typed by Cmd on both ends of the realizing connector; applied stereotypes «ItemFlow» */
            view 'Sys Internals' {
                expose supply;
                expose load;
                expose spare;
                expose aux;
                expose ctrl;
                expose unit;
                expose 'supply.pwr out to load.pwr in';
                expose cmd;
                expose tlm;
                expose 'spare feed';
                expose 'supply.pwr out to aux.pwr in';
                @DiagramLayout::Canvas { :>> DiagramLayout::Canvas::unit = "px"; width = 600; height = 300; }
                metadata DiagramLayout::Layout about supply { x = 20; y = 120; width = 100; height = 60; }
                metadata DiagramLayout::Layout about load { x = 260; y = 20; width = 100; height = 60; }
                metadata DiagramLayout::Layout about spare { x = 260; y = 120; width = 100; height = 60; }
                metadata DiagramLayout::Layout about aux { x = 260; y = 220; width = 100; height = 60; }
                metadata DiagramLayout::Layout about ctrl { x = 420; y = 20; width = 100; height = 60; }
                metadata DiagramLayout::Layout about unit { x = 420; y = 220; width = 100; height = 60; }
                metadata DiagramLayout::Route about 'supply.pwr out to load.pwr in' { points = (120, 140, 190, 140, 190, 50, 260, 50); }
                metadata DiagramLayout::Route about cmd { points = (470, 80, 470, 220); }
                metadata DiagramLayout::Route about tlm { points = (470, 220, 470, 80); }
                metadata DiagramLayout::Route about 'spare feed' { points = (120, 150, 260, 150); }
                metadata DiagramLayout::Route about 'supply.pwr out to aux.pwr in' { points = (120, 160, 190, 160, 190, 250, 260, 250); }
                render Views::asInterconnectionDiagram;
            }
            metadata MigrationMetadata::SynthesizedName about 'supply.pwr out to load.pwr in', 'supply.pwr out to aux.pwr in';
        }
    }
}
