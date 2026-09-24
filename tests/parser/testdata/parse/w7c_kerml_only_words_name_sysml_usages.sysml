// SysML.xtext extends KerMLExpressions.xtext, not KerML.xtext, so a literal
// only KerML declares is an ordinary name here (matched: pinned validator clean).
package W7CContextualNames {
    part def T;

    part chains : T;
    part differences : T;
    part disjoint : T;
    part inverse : T;
    part unions : T;
    part intersects : T;
    part conjugates : T;
    part type : T;
    part namespace : T;
    part multiplicity : T;
    part step : T;
    part class : T;
    part struct : T;
    part behavior : T;
    part feature : T;
    part function : T;
    part predicate : T;
    part specialization : T;
    part subsets2 : T;

    part def Holder {
        // A kindless member keeps the name it is written with.
        chains : Real;
        ref differences : Real;
        attribute redefinition : Real;
        attribute inverting : Real;
        attribute disjoining : Real;
        attribute typing : Real;
        attribute portion : Real;
        attribute member : Real;
    }

    // A relationship word still states the relationship it always did when it
    // stands where one belongs, which is how KerML notation stays parsed here.
    part chained : T subsets chains;
}
