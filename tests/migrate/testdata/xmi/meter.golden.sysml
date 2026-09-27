enum def Mode {
    off;
    on;
}
part def Gauge {
    attribute reading : ScalarValues::Real default = 3.0;
    action def Tick {
        first start then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
part def Needle :> Gauge;
part def Bank {
    attribute level : ScalarValues::Real default = 0.0;
}
part def Meter {
    attribute mode : Mode;
    part banks : Bank;
    part dial : Gauge;
    part hand : Needle;
    part cells : Gauge[2];
    attribute count : ScalarValues::Integer default = 0;
    attribute label : ScalarValues::String;
    attribute total : ScalarValues::Real default = 0.0;
    attribute ticks : ScalarValues::Integer default = 7;
    attribute half : ScalarValues::Integer default = 0;
    attribute quarter : ScalarValues::Integer default = 0;
    attribute eighth : ScalarValues::Real default = 0.0;
    attribute ratio : ScalarValues::Real default = 0.0;
    attribute floored : ScalarValues::Real default = 0.0;
    attribute rounded : ScalarValues::Integer default = 0;
    attribute peak : ScalarValues::Real default = 0.0;
    attribute gain : ScalarValues::Real {
        /* default value not migrated: {JavaScript} total(count) — the call "total" is not in the translated function table */
    }
    attribute tag : ScalarValues::String {
        /* default value not migrated: {JavaScript} label + "!" — the construct "+" is outside the translated subset: string concatenation has no v2 form in the subset */
    }
    attribute square : ScalarValues::Real {
        /* default value not migrated: {JavaScript} -total ** 2 — the construct "-total **" is outside the translated subset: JavaScript parenthesizes a unary operand of `**` */
    }
    attribute cube : ScalarValues::Real default = -(total ** 3);
    ref part shown : Gauge default = if (count > 0) ? dial else hand;
    ref part pointer : Needle {
        /* default value not migrated: {JavaScript} count > 0 ? dial : hand — the types at "count > 0 ? dial : hand" disagree: the expression is a Gauge, not the Needle wanted */
    }
    attribute ready : ScalarValues::Boolean default = true;
    attribute enabled : ScalarValues::Boolean default = false;
    attribute flag : ScalarValues::Boolean default = false;
    attribute lit : ScalarValues::Boolean {
        /* default value not migrated: {JavaScript} ready or enabled — the text "or" is not expression syntax: text follows the expression */
    }
    attribute top : ScalarValues::Real {
        /* default value not migrated: {JavaScript} cells.reading — the types at "cells.reading" disagree: the expression is a collection, not the one value wanted */
    }
    action def Measure {
        in ref context : Meter;
        first start then sense;
        action sense {
            in x : ScalarValues::Real = 1.5;
            out y : ScalarValues::Real;
            assign y := x * 2;
        }
        first sense then record;
        action record {
            in v : ScalarValues::Real;
            assign context.total := v + 1;
            assign context.peak := v;
        }
        first record then split;
        action split {
            assign context.half := OpenSysMLMathFunctions::quotient(context.ticks, 2);
            assign context.ratio := context.total / 2;
            assign context.floored := RealFunctions::floor(context.total) / 8;
            assign context.rounded := OpenSysMLMathFunctions::quotient(RealFunctions::floor(context.total + 0.5), 8);
        }
        first split then quarterly;
        action quarterly {
            assign context.quarter := OpenSysMLMathFunctions::quotient(context.ticks, 4);
        }
        first quarterly then cc;
        action cc {
            assign context.eighth := context.ticks / 8;
        }
        first cc then judge;
        action judge {
            in ok : ScalarValues::Boolean;
            in n : ScalarValues::Integer = RealFunctions::floor(context.total);
            in m : ScalarValues::Integer = 2;
            assign context.count := n + m;
        }
        first judge then log;
        action log {
            assign context.ticks := context.ticks + 1;
        }
        first log then shout;
        action shout;
        first shout then final;
        action final terminate;
        flow sense.y to record.v;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def Skip {
        in ref context : Meter;
        first start then idle;
        action idle {
            out z : ScalarValues::Real;
            assign context.total := context.total * 1;
        }
        first idle then sink;
        action sink {
            in w : ScalarValues::Real[0..1];
            if w->SequenceFunctions::notEmpty() { assign context.total := w; }
        }
        first sink then dark;
        action dark {
            out q : ScalarValues::Real;
            /* body not migrated (the statement assigns q, which nothing visible from Meter::Skip::dark is called (language Python)) {Python}:
             * q = 1
             */
        }
        first dark then drain;
        action drain {
            in w : ScalarValues::Real[0..1];
            if w->SequenceFunctions::notEmpty() { assign context.total := w; }
        }
        first drain then final;
        action final terminate;
        /* flow idle.z to sink.w not written: the body of 'idle' never assigns idle.z */
        /* flow dark.q to drain.w not written: 'dark' is not migrated and produces no value */
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def Flip {
        in ref context : Meter;
        first start then bump;
        action bump {
            /* body not migrated (the types at "+" disagree: an operand is a Mode, not a number) {JavaScript}:
             * count = mode + 1;
             */
        }
        first bump then pick;
        action pick {
            /* body not migrated (the types at "count =" disagree: a Mode is assigned to the Integer count holds) {JavaScript}:
             * count = mode;
             */
        }
        first pick then match;
        action match {
            /* body not migrated (the types at "mode =" disagree: a Gauge is assigned to the Mode mode holds) {JavaScript}:
             * mode = dial;
             */
        }
        first match then widen;
        action widen {
            assign context.dial := context.hand;
        }
        first widen then narrow;
        action narrow {
            /* body not migrated (the types at "hand =" disagree: a Gauge is assigned to the Needle hand holds) {JavaScript}:
             * hand = dial;
             */
        }
        first narrow then huge;
        action huge {
            /* body not migrated (the construct "9007199254740993" is outside the translated subset: a script rounds a whole number beyond 9007199254740991 to the nearest floating-point value) {JavaScript}:
             * count = 9007199254740993;
             */
        }
        first huge then smile;
        action smile {
            assign context.label := "😀";
            assign context.count := 9007199254740991;
        }
        first smile then same;
        action same {
            assign context.lit := context.label == "😀";
        }
        first same then 'alias';
        action 'alias' {
            /* body not migrated (the construct "==" is outside the translated subset: Java compares strings by identity with `==`, which a comparison of their values does not reproduce; `equals` compares their content) {Java}:
             * lit = label == "\uD83D\uDE00";
             */
        }
        first 'alias' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def Arm {
        in armed : ScalarValues::Boolean {
            /* default value not migrated: {JavaScript} Math.sqrt(total) — the types at "Math.sqrt(total)" disagree: the expression is a Real, not the Boolean wanted */
        }
        in limit : ScalarValues::Integer default = context.count + 1;
        in ref context : Meter;
        first start then fuse;
        action fuse {
            /* body not migrated (the text "and" is not expression syntax: a statement ends at `;` or a newline) {JavaScript}:
             * flag = ready and enabled
             */
        }
        /* guard not migrated: [{JavaScript} ready and enabled] — the text "and" is not expression syntax: text follows the expression */
        first fuse then latch;
        action latch {
            assign context.flag := context.ready and context.enabled;
        }
        first latch then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def Sweep {
        in ref context : Meter;
        first start then spread;
        action spread {
            /* body not migrated (the types at "+" disagree: an operand is a collection, not a number) {JavaScript}:
             * total = cells.reading + 1;
             */
        }
        first spread then reset;
        action reset {
            /* body not migrated (the construct "cells.reading" is outside the translated subset: context.cells is a collection, so the assignment would write through several objects) {JavaScript}:
             * cells.reading = 1;
             */
        }
        first reset then peak;
        action peak {
            assign context.total := context.cells.reading->ControlFunctions::reduce { in x; in y; RealFunctions::max(x, y) };
        }
        first peak then calib;
        action calib {
            /* body not migrated (the construct "reading" is outside the translated subset: this.cells is a collection, so the assignment would write through several objects) {JavaScript}:
             * reading = 2;
             */
        }
        first calib then span;
        action span {
            /* body not migrated (the types at "total =" disagree: one side is a collection and the other a single value) {JavaScript}:
             * total = reading;
             */
        }
        first span then ticking;
        action ticking {
            /* not migrated: CallBehaviorAction 'ticking' — the behavior acts on a Gauge through its parameter context, which is left unbound: the caller is a Meter, which is no Gauge and has 5 parts that are one, so no one of them is chosen; v1 runs Gauge::Tick on the caller's object, which lacks the ports it goes through, so the action carries the token and performs nothing */
        }
        first ticking then tally;
        action tally {
            /* body not migrated (the construct "banks.level" is outside the translated subset: the multiplicity of Meter::banks is not written in numbers, so whether it holds one value cannot be told) {JavaScript}:
             * total = banks.level + 1;
             */
        }
        first tally then charge;
        action charge {
            /* body not migrated (the name "level" resolves to nothing readable: nothing visible from Meter::Sweep::charge is called level) {JavaScript}:
             * level = 1;
             */
        }
        first charge then settle;
        action settle {
            /* body not migrated (the name "level" resolves to nothing readable: nothing visible from Meter::Sweep::settle is called level) {JavaScript}:
             * level = 0;
             */
        }
        first settle then final;
        action final terminate;
        /* partition 'cells' represents Meter::cells: calib, span, ticking */
        /* partition 'banks' represents Meter::banks: charge */
        /* partition 'bank' represents Bank: settle */
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
