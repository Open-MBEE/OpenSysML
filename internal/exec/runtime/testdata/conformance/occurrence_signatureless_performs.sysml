package Instrument {
    part Operator;
    package Blocks {
        part def 'Sample Stage' { action detectCharging; }
        part def Pump;
        part def Display;
        part def Imager { action selectMode; action focus; }
        part def Image { action data; }
    }
    package Signals { item def PositionStage; item def ShowImage; }

    // A migrated use case: an untyped lifeline subsetting a part, signatureless
    // messages, signal payloads, a message to a lifeline not yet reached (a
    // create), and events performing body-less actions of the lifeline's type.
    occurrence def Acquire {
        ref part operator :> Operator {
            event occurrence positionSent;
            then event occurrence stageSent;
            then event occurrence pumpSent;
            then event occurrence modeSent;
            then event occurrence focusSent;
            then event occurrence chargingSent;
        }
        ref part stage : Blocks::'Sample Stage' {
            event occurrence positionReceived;
            then event occurrence stageReceived;
            then event occurrence chargingReceived {
                perform action detectChargingCall ::> Blocks::'Sample Stage'::detectCharging;
            }
        }
        ref part pump : Blocks::Pump {
            event occurrence pumpReceived;
        }
        ref part display : Blocks::Display {
            event occurrence showReceived;
            then event occurrence dataSent;
        }
        ref part imager : Blocks::Imager {
            event occurrence modeReceived {
                perform action selectModeCall ::> Blocks::Imager::selectMode;
            }
            then event occurrence createSent;
            then event occurrence showSent;
            then event occurrence focusReceived {
                perform action focusCall ::> Blocks::Imager::focus;
            }
        }
        ref part image : Blocks::Image {
            event occurrence createReceived;
            then event occurrence dataReceived {
                perform action dataCall ::> Blocks::Image::data;
            }
        }
        message position from operator.positionSent to stage.positionReceived;
        message positionStage of Signals::PositionStage[1] from operator.stageSent to stage.stageReceived;
        message pumpDown from operator.pumpSent to pump.pumpReceived;
        message selectMode from operator.modeSent to imager.modeReceived;
        message create from imager.createSent to image.createReceived;
        message show of Signals::ShowImage[1] from imager.showSent to display.showReceived;
        message data from display.dataSent to image.dataReceived;
        message focus from operator.focusSent to imager.focusReceived;
        message detectCharging from operator.chargingSent to stage.chargingReceived;
    }
}
