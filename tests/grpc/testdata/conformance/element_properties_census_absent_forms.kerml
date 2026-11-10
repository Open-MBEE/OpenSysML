// The relationships the API element form states as owner text (unioning, intersecting,
// differencing, cross subsetting, type featuring), so the census sees them become elements.
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
