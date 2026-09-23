package Control {
    part def Controller {
        action def Run {
            view Run : StandardViewDefinitions::ActionFlowView {
                expose Control::Controller::Run;
                render Views::asInterconnectionDiagram;
            }
            succession 'start to fork' first start then 'fork';
            fork 'fork';
            succession 'fork to heat' first 'fork' then heat;
            succession 'fork to cool' first 'fork' then cool;
            action heat {
                /* body not migrated (the body is empty) */
            }
            succession 'heat to join' first heat then 'join';
            action cool {
                /* body not migrated (the body is empty) */
            }
            succession 'cool to join' first cool then 'join';
            join 'join';
            succession 'join to check' first 'join' then check;
            decide check;
            succession 'check to final' first check then final;
            succession 'check to done' first check if false then done;
            action final terminate;
        }
    }
}
