package P {
    constraint c;
    part def Holder {
        constraint c;
    }
    part h : Holder;

    assert $::P::c;
    assert not $::P::c;
    assert $::P::h.c;
}
