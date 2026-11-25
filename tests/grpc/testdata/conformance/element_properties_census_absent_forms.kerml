// Unioning, intersecting, differencing, cross subsetting and type featuring, each of which the
// API element form states as an element, so the census has a carrier for their rules.
package CensusAbsentForms {
    classifier Adult;
    classifier Child;
    classifier Person unions Adult, Child {
        feature dependents : Person[*];
        feature offspring : Person[*];
        feature grownOffspring : Person[*] differences offspring, dependents;
        feature youngDependents : Person[*] intersects dependents, offspring;
    }
    classifier Asset {
        feature owningEntities : Person[*];
    }
    assoc Ownership {
        end feature ownedAsset : Asset[*];
        end feature owner : Person[*] crosses ownedAsset.owningEntities;
    }
    classifier Vehicle;
    classifier Engine;
    feature engine : Engine featured by Vehicle;
}
