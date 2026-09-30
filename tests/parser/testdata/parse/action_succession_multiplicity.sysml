action def A {
    action a;
    then [1] action b;
    [0..1] then c;
    action c;
    [2] then d { action nested; }
    action d;
    then [0..1] e;
    action e;
    then [1] f { action nested; }
    action f;
    [1] then [0..1] g;
    action g;
    then [*] done;
}
