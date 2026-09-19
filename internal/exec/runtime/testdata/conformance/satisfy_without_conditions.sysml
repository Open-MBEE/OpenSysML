// A requirement that states no condition carries no verdict, so satisfying it
// reports that rather than claiming a check that never ran.
package test {
    requirement def Documented {
        doc /* stated in prose only */
    }

    requirement documented : Documented;

    part lander;

    part analysisContext {
        assert satisfy documented by lander;
    }
}
