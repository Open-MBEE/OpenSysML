action def Charger {
    action begin;
    first begin;
    then loop action { action tick; } until battery >= 100;
    then done;
}
