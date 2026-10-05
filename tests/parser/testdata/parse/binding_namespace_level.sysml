// A binding connector owned by a package: its ends are names and feature chains of
// the namespace's own usages, connector and end multiplicities included.
package P {
    part def Car;
    part a : Car;
    part b : Car;
    bind a = b;
    binding named bind a = b;
    part c : Car;
    part d : Car { part w2 : Car; }
    bind c = d.w2;
    binding [1] bind [0..1] a = [0..1] b;
}
