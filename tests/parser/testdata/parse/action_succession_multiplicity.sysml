action def A {
    action a;
    then [1] action b;
    [0..1] then c;
    action c;
    [2] then d { action nested; }
    action d;
}
