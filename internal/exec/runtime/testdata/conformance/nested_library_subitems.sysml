// The nested usages a library declares subset the collection their kind nests
// under like a model's own (SysML v2 §8.3): the Geometry library's Triangle
// declares its three edges and six edge ends as items with no written
// subsetting, and they are the triangle's `subitems`.
package Sign {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    private import ShapeItems::*;
    private import SI::*;

    item plate : Triangle {
        attribute :>> length = 3 [m];
        attribute :>> width = 4 [m];
        attribute :>> xoffset = 0 [m];
        item bolt[2];
        attribute itemCount : Natural = subitems->size();
        attribute edgeCount : Natural = edges->size();
        attribute vertexCount : Natural = vertices->size();
    }
}
