// A part declared in one document performs the action usage its definition, in another
// document, owns. The body's names resolve through the owning definition — `runs` is its
// own attribute, `tank.volume` a nested part's — so the performance on the object writes
// that object's features, as it does when both are declared in one document: the owning
// object is the `this` of the performances it owns (`Objects::Object::ownedPerformances`
// redefines `this default that`; `Parts::Part::ownedActions` fixes `this` to the owning Part).
package test {
    private import plantDefs::*;

    part plant : Plant;
}
