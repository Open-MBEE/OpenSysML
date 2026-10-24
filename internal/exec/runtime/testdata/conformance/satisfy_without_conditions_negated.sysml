// A requirement stating no condition holds vacuously, so `assert not satisfy`
// of it is violated while the plain assertion holds; one stating only an
// assumption is judged the same way, its assumption binding nothing.
package test {
    requirement def Documented {
        doc /* stated in prose only */
    }

    requirement documented : Documented;

    requirement def Assumed {
        subject craft : Lander;
        assume constraint { craft.ready }
    }

    requirement assumed : Assumed;

    part def Lander {
        attribute ready = true;
    }

    part lander : Lander;

    part analysisContext {
        assert satisfy documented by lander;
        assert not satisfy documented by lander;
        assert satisfy assumed by lander;
        assert not satisfy assumed by lander;
    }
}
