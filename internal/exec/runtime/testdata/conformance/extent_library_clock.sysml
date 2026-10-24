// The library's documents are part of the model, so `all Clock` reaches the one Clock the
// standard library declares at namespace level, `Time::universalClock : Clock[1]` (Time.sysml),
// and `all Rendering` the six `[1]` renderings it names (`Views::asTextualNotation`,
// `asTreeDiagram`, `asInterconnectionDiagram`, `asElementTable`, `OpenSysMLRenderings::asCaseDiagram`,
// `asMixedDiagram`). Those are the Items too: the library's `[0..*]` collections — `Items::items`,
// `Parts::parts`, `Views::renderings` — hold nothing of their own, so they neither count nor refuse the extent.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    private import Time::*;
    private import Items::*;
    private import Views::*;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Clock) == 1,
            (all Clock)#(1) === universalClock,
            size(all Rendering) == 6,
            size(all Item) == size(all Clock) + size(all Rendering),
            includes(all Item, universalClock)
        );
    }
}
