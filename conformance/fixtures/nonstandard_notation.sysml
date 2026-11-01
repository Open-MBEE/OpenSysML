package P {
  state def S {
    entry; then a;
    state a;
    state b;
    choice c;
    transition a_c first a then c;
    transition c_b first c then b;
  }
}
