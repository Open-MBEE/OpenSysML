// The library's documents are part of the model, so `all Clock` reaches the one Clock the
// standard library declares at namespace level, `Time::universalClock : Clock[1]` (Time.sysml).
// It is the one Item too: the library's `[0..*]` collections — `Items::items`,
// `Parts::parts` — hold nothing of their own, so they neither count nor refuse the extent.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    private import Time::*;
    private import Items::*;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Clock) == 1,
            (all Clock)#(1) === universalClock,
            size(all Item) == 1,
            (all Item)#(1) === universalClock
        );
    }
}
