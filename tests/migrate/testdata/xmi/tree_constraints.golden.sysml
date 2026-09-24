package Tanks {
    enum def Mode {
        Idle;
        Filling;
    }
    part def Tank {
        attribute level : ScalarValues::Real;
        attribute capacity : ScalarValues::Real default = 100.0;
        attribute reserve : ScalarValues::Real default = 0.1 * capacity;
        attribute mode : Mode;
        attribute label : ScalarValues::String {
            /* default value not migrated: (tank-, A) — the UML StringExpression tree has no v2 form: the construct "+" is outside the translated subset: string concatenation has no v2 form in the subset */
        }
        constraint 'filling below capacity' { mode != Mode::Filling or level < capacity - reserve }
        /* not migrated: Constraint 'unnamed operand' ==(mode, <InstanceValue>) — the UML Expression tree has no v2 form: the construct "<InstanceValue>" is outside the translated subset: the instance value names no instance */
        /* not migrated: Constraint 'sealed' xor(level, capacity) — the UML Expression tree has no v2 form: the call "xor" is not in the translated function table */
    }
    constraint def WithinCapacity {
        in attribute level : ScalarValues::Real;
        in attribute capacity : ScalarValues::Real;
        in attribute margin : ScalarValues::Real;
        RealFunctions::max(level, 0.0) <= capacity and margin * 2 % 7 >= -0.5 and not (level == capacity / 2)
    }
    constraint def Settled {
        in attribute level : ScalarValues::Real;
        /* not migrated: Constraint (_sRule) >(level, {English} the settling level) — the UML Expression tree has no v2 form: the language "English" is not translated: an opaque operand is read only in a script language */
    }
    constraint def Diagonal {
        in attribute width : ScalarValues::Real;
        in attribute height : ScalarValues::Real;
        in attribute span : ScalarValues::Real;
        width ** 2 + height ** 2 == span ** 2
    }
    individual part def dayTank :> Tank {
        attribute :>> level = 40 + 2.5;
        attribute :>> mode = Mode::Idle;
    }
}
