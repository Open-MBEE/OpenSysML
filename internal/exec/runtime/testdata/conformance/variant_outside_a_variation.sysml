// A member declared `variant` whose owner is not a variation offers no choice to
// anything, so it stays an ordinary feature: it materializes a slot holding its
// own value rather than vanishing from the object.
package test {
    private import ScalarValues::Real;

    part def Widget {
        attribute plain : Real = 2.0;
        variant attribute misplaced : Real = 1.0;
    }

    part widget : Widget {
        attribute total : Real = plain + misplaced;
    }
}
