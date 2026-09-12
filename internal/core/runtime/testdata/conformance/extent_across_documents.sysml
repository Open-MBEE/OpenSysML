// An extent reaches the usages every document of the model declares, in document-name then
// declaration order: `extent_across_documents.depot.sysml`, indexed beside this file, sorts
// before it, so its `van` comes first, then `coupe` and `sedan` in the order this file declares
// them. The trace pins the far usage materializing before the near ones.
package fleet {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Car;

    part coupe : Car;
    part sedan : Car;

    calc extents {
        return : Boolean[*] nonunique = (
            size(all Car) == 3,
            (all Car)#(1) === depot::van,
            (all Car)#(2) === coupe,
            (all Car)#(3) === sedan
        );
    }
}
