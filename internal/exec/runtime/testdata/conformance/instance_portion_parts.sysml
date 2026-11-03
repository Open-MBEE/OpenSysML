// A timeslice or snapshot part nested in a part is a portion of it: the whole's `subparts`,
// `subitems` and `suboccurrences` list the portion, while the portion's own list the whole's
// parts without the whole's portions — itself and its sibling slices — so nothing is ever held
// as a portion of itself. A plain nested part is the control.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Node { attribute id : Natural; }

    part def Bare {
        timeslice part cruise;
    }

    part def Sliced {
        part hull : Node { attribute :>> id = 1; }
        timeslice part cruise;
        snapshot part launch;
    }

    part def Nested {
        timeslice part cruise {
            snapshot part mid;
        }
    }

    part def Plain {
        part hull : Node { attribute :>> id = 2; }
        part cruise;
    }

    part def Sys {
        part bare : Bare;
        part sliced : Sliced;
        part nested : Nested;
        part plain : Plain;

        assert constraint bareSubparts { size(bare.subparts) == 1 }
        assert constraint bareListsCruise { includes(bare.subparts, bare.cruise) }
        assert constraint bareSubitems { size(bare.subitems) == 1 }
        assert constraint bareCruiseSubparts { isEmpty(bare.cruise.subparts) }
        assert constraint bareCruiseSubitems { isEmpty(bare.cruise.subitems) }
        assert constraint bareCruiseSuboccurrences { isEmpty(bare.cruise.suboccurrences) }

        assert constraint slicedSubparts { size(sliced.subparts) == 3 }
        assert constraint slicedSubitems { size(sliced.subitems) == 3 }
        assert constraint slicedListsCruise { includes(sliced.subparts, sliced.cruise) }
        assert constraint slicedListsLaunch { includes(sliced.subparts, sliced.launch) }
        assert constraint cruiseSubparts { size(sliced.cruise.subparts) == 1 }
        assert constraint cruiseListsHull { includes(sliced.cruise.subparts, sliced.hull) }
        assert constraint cruiseNotItself { not includes(sliced.cruise.subparts, sliced.cruise) }
        assert constraint cruiseNotSibling { not includes(sliced.cruise.subparts, sliced.launch) }
        assert constraint cruiseSubitems { size(sliced.cruise.subitems) == 1 }
        assert constraint cruiseSuboccurrences { size(sliced.cruise.suboccurrences) == 1 }
        assert constraint launchSubparts { size(sliced.launch.subparts) == 1 }
        assert constraint launchListsHull { includes(sliced.launch.subparts, sliced.hull) }
        assert constraint launchSubitems { size(sliced.launch.subitems) == 1 }

        assert constraint nestedSubparts { size(nested.subparts) == 1 }
        assert constraint nestedCruiseSubparts { size(nested.cruise.subparts) == 1 }
        assert constraint nestedCruiseListsMid { includes(nested.cruise.subparts, nested.cruise.mid) }
        assert constraint nestedMidSubparts { isEmpty(nested.cruise.mid.subparts) }
        assert constraint nestedMidSubitems { isEmpty(nested.cruise.mid.subitems) }

        assert constraint plainSubparts { size(plain.subparts) == 2 }
        assert constraint plainCruiseSubparts { isEmpty(plain.cruise.subparts) }
        assert constraint plainCruiseSubitems { isEmpty(plain.cruise.subitems) }
    }

    part sys : Sys;
}
