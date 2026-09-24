package AssertNegated {
    requirement touchdown;
    part lander;

    part context {
        assert not constraint { 1 < 2 }
        assert not satisfy touchdown by lander;
        assert constraint { 1 < 2 }
    }
}
