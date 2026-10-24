package test {
    private import ScalarValues::Integer;
    private import ScalarValues::Boolean;
    private import SequenceFunctions::size;
    private import SequenceFunctions::isEmpty;

    // A loop an action performs is a LoopAction subaction held under its `loops`,
    // and every iteration performs the loop's body as an occurrence held under the
    // loop's `body`, with the body's own nested actions as its subperformances.
    action def G {
        attribute i : Integer = 0;
        first start;
        then loop action body {
            first start;
            then action inc assign i := i + 1;
            then done;
        } until i >= 2;
        then done;
    }
    action def S :> G;

    part def K {
        attribute n : Integer = 0;
        attribute m : Integer = 0;
        attribute t : Integer = 0;

        perform action loopy {
            first start;
            then loop action body {
                first start;
                then action inc assign n := n + 1;
                then done;
            } until n >= 3;
            then done;
        }
        perform action plain { action s1; action s2; first s1 then s2; }
        perform action zero {
            first start;
            then while false { action never; }
            then done;
        }
        perform action fore {
            first start;
            then for i in 1..2 { action add assign m := m + i; }
            then done;
        }
        perform action nest {
            first start;
            then loop action outer {
                first start;
                then loop action inner {
                    first start;
                    then action tick assign t := t + 1;
                    then done;
                } until t % 2 == 0;
                then done;
            } until t >= 4;
            then done;
        }
        perform action inherited : S;

        attribute loopySubperformanceCount : Integer = size(loopy.subperformances);
        attribute loopySuboccurrenceCount : Integer = size(loopy.suboccurrences);
        attribute loopyEnclosedCount : Integer = size(loopy.enclosedPerformances);
        attribute loopyTimeEnclosedCount : Integer = size(loopy.timeEnclosedOccurrences);
        attribute loopyLoopCount : Integer = size(loopy.loops);
        attribute loopIsTheSubperformance : Boolean = loopy.subperformances#(1) == loopy.loops#(1);
        attribute loopContextIsLoopy : Boolean = loopy.loops#(1).this == loopy;
        attribute bodyCount : Integer = size(loopy.loops#(1).body);
        attribute loopHasNoSubperformances : Boolean = isEmpty(loopy.loops#(1).subperformances);
        attribute bodiesAreDistinct : Boolean = loopy.loops#(1).body#(1) != loopy.loops#(1).body#(2);
        attribute bodySubperformanceCount : Integer = size(loopy.loops#(1).body#(1).subperformances);
        attribute incContextIsItsBody : Boolean = loopy.loops#(1).body#(3).subperformances#(1).this == loopy.loops#(1).body#(3);
        attribute incsAreDistinct : Boolean = loopy.loops#(1).body#(1).subperformances#(1) != loopy.loops#(1).body#(2).subperformances#(1);
        attribute plainSuboccurrenceCount : Integer = size(plain.suboccurrences);

        attribute zeroLoopCount : Integer = size(zero.loops);
        attribute zeroBodyIsEmpty : Boolean = isEmpty(zero.loops#(1).body);

        attribute foreLoopCount : Integer = size(fore.loops);
        attribute foreBodyCount : Integer = size(fore.loops#(1).body);

        attribute nestLoopCount : Integer = size(nest.loops);
        attribute outerBodyCount : Integer = size(nest.loops#(1).body);
        attribute innerLoopsOfFirstOuterBody : Integer = size(nest.loops#(1).body#(1).loops);
        attribute innerBodiesOfFirstOuterBody : Integer = size(nest.loops#(1).body#(1).loops#(1).body);
        attribute innerLoopsAreDistinct : Boolean = nest.loops#(1).body#(1).loops#(1) != nest.loops#(1).body#(2).loops#(1);

        attribute inheritedCount : Integer = inherited.i;
        attribute inheritedBodyCount : Integer = size(inherited.loops#(1).body);
        attribute inheritedBodySubperformanceCount : Integer = size(inherited.loops#(1).body#(1).subperformances);
        attribute inheritedIncContextIsItsBody : Boolean = inherited.loops#(1).body#(2).subperformances#(1).this == inherited.loops#(1).body#(2);
    }
    part k : K;
}
