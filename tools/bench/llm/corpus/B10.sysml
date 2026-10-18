package Vending {
    private import ScalarValues::*;

    attribute def CoinInserted {
        attribute value : Real;
    }
    attribute def ProductSelected {
        attribute code : String;
    }
    attribute def Dispensed;

    action def DispenseProduct {
        in code : String;
    }
    action def ReturnChange {
        in amount : Real;
    }

    part def VendingMachine {
        attribute credit : Real;

        state def MachineStates {
            entry; then waitingForCoins;
            state waitingForCoins;
            state waitingForSelection;
            state dispensing {
                entry action dispense : DispenseProduct;
                exit action refund : ReturnChange;
            }
            transition coins_to_selection first waitingForCoins accept CoinInserted then waitingForSelection;
            transition more_coins first waitingForSelection accept CoinInserted then waitingForSelection;
            transition selection_to_dispensing first waitingForSelection accept ProductSelected then dispensing;
            transition dispensing_to_coins first dispensing accept Dispensed then waitingForCoins;
        }
        exhibit state machineBehavior : MachineStates;
    }
}
