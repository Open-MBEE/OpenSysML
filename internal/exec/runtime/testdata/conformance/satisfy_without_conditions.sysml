// A requirement that states no condition holds vacuously: its result is
// `allTrue(assumptions()) implies allTrue(constraints())` over no constraint,
// which is true, so satisfying it holds.
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
