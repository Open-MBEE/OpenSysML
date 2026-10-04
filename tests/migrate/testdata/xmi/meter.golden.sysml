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
    action measure {
        first start then sense;
        action sense {
            in x : ScalarValues::Real[1] = 1.5;
            out y : ScalarValues::Real[1];
            rep language "JavaScript" /* y = x * 2; */
            assign y := x * 2;
        }
        action record {
            in v : ScalarValues::Real[1];
            rep language "JavaScript" /* total = v + 1; peak = Math.max(v); */
            assign total := v + 1;
            assign peak := v;
        }
        first record then split;
        action split {
            rep language "Java" /* half = ticks / 2; ratio = total / 2; floored = Math.floor(total) / 8; rounded = Math.round(total) / 8; */
            assign half := OpenSysMLMathFunctions::quotient(ticks, 2);
            assign ratio := total / 2;
            assign floored := RealFunctions::floor(total) / 8;
            assign rounded := OpenSysMLMathFunctions::quotient(RealFunctions::floor(total + 0.5), 8);
        }
        first split then quarterly;
        action quarterly {
            rep language "Java 1.8.0_202" /* quarter = ticks / 4; */
            assign quarter := OpenSysMLMathFunctions::quotient(ticks, 4);
        }
        first quarterly then cc;
        action cc {
            rep language "JavaCC" /* eighth = ticks / 8; */
            assign eighth := ticks / 8;
        }
        first cc then judge;
        action judge {
            in ok : ScalarValues::Boolean[1];
            in n : ScalarValues::Integer[1] = RealFunctions::floor(total);
            in m : ScalarValues::Integer[1] = 2;
            rep language "JavaScript" /* count = n + m; */
            assign count := n + m;
        }
        first judge then log;
        action log {
            rep language "JavaScript" /* print("total so far: " + (total + 1));
             * ticks = ticks + 1;
             * println(ticks);
             */
            assign ticks := ticks + 1;
        }
        first log then shout;
        action shout {
            rep language "JavaScript" /* println("done"); */
        }
        first shout then final;
        action final terminate;
        succession flow of ScalarValues::Real from sense.y to record.v;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action skip {
        first start then idle;
        action idle {
            out z : ScalarValues::Real[1];
            rep language "JavaScript" /* total = total * 1; */
            assign total := total * 1;
        }
        first idle then sink;
        action sink {
            in w : ScalarValues::Real[0..1];
            rep language "JavaScript" /* total = w; */
            if w->SequenceFunctions::notEmpty() { assign total := w; }
        }
        first sink then dark;
        action dark {
            out q : ScalarValues::Real[1];
            rep language "Python" /* q = 1 */
        }
        first dark then drain;
        action drain {
            in w : ScalarValues::Real[0..1];
            rep language "JavaScript" /* total = w; */
            if w->SequenceFunctions::notEmpty() { assign total := w; }
        }
        first drain then final;
        action final terminate;
        /* flow idle.z to sink.w not written: the body of 'idle' never assigns idle.z */
        /* flow dark.q to drain.w not written: 'dark' is not migrated and produces no value */
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action flip {
        first start then bump;
        action bump {
            rep language "JavaScript" /* count = mode + 1; */
        }
        first bump then pick;
        action pick {
            rep language "JavaScript" /* count = mode; */
        }
        first pick then match;
        action match {
            rep language "JavaScript" /* mode = dial; */
        }
        first match then widen;
        action widen {
            rep language "JavaScript" /* dial = hand; */
            assign dial := hand;
        }
        first widen then narrow;
        action narrow {
            rep language "JavaScript" /* hand = dial; */
        }
        first narrow then huge;
        action huge {
            rep language "JavaScript" /* count = 9007199254740993; */
        }
        first huge then smile;
        action smile {
            rep language "JavaScript" /* label = "\uD83D\uDE00"; count = 9007199254740991; */
            assign label := "😀";
            assign count := 9007199254740991;
        }
        first smile then same;
        action same {
            rep language "Java" /* lit = label.equals("\uD83D\uDE00"); */
            assign lit := label == "😀";
        }
        first same then 'alias';
        action 'alias' {
            rep language "Java" /* lit = label == "\uD83D\uDE00"; */
        }
        first 'alias' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action arm {
        in armed : ScalarValues::Boolean[1] {
            /* default value not migrated: {JavaScript} Math.sqrt(total) — the types at "Math.sqrt(total)" disagree: the expression is a Real, not the Boolean wanted */
        }
        in limit : ScalarValues::Integer[1] default = count + 1;
        first start then fuse;
        action fuse {
            rep language "JavaScript" /* flag = ready and enabled */
        }
        /* guard not migrated: [{JavaScript} ready and enabled] — the text "and" is not expression syntax: text follows the expression */
        first fuse then latch;
        action latch {
            rep language "JavaScript" /* flag = ready && enabled */
            assign flag := ready and enabled;
        }
        first latch then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action sweep {
        first start then spread;
        action spread {
            rep language "JavaScript" /* total = cells.reading + 1; */
        }
        first spread then reset;
        action reset {
            rep language "JavaScript" /* cells.reading = 1; */
        }
        first reset then peak;
        action peak {
            rep language "JavaScript" /* total = java.util.Collections.max(cells.reading); */
            assign total := cells.reading->ControlFunctions::reduce { in x; in y; RealFunctions::max(x, y) };
        }
        first peak then calib;
        action calib {
            rep language "JavaScript" /* reading = 2; */
        }
        first calib then span;
        action span {
            rep language "JavaScript" /* total = reading; */
        }
        first span then ticking;
        action ticking : Gauge::Tick;
        first ticking then tally;
        action tally {
            rep language "JavaScript" /* total = banks.level + 1; */
        }
        first tally then charge;
        action charge {
            rep language "JavaScript" /* level = 1; */
        }
        first charge then settle;
        action settle {
            rep language "JavaScript" /* level = 0; */
        }
        first settle then final;
        action final terminate;
        /* partition 'cells' represents Meter::cells: calib, span, ticking */
        /* partition 'banks' represents Meter::banks: charge */
        /* partition 'bank' represents Bank: settle */
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
