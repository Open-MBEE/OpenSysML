// Two objects of one type each bind the same variation interface to a different
// variant, in the `interface :>> link = link::direct;` form.
package variantRouting {
    part def Router {
        port outPort;
        port inPort;
        port bypass;

        variation interface link {
            variant interface direct connect outPort to inPort;
            variant interface indirect connect outPort to bypass;
        }
    }

    part alpha : Router {
        interface :>> link = link::direct;
    }

    part beta : Router {
        interface :>> link = link::indirect;
    }
}
