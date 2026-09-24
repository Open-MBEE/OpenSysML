package FeaturePrefixes {
    class A;
    class B {
        const feature k : A;
        var const feature vk : A;
        portion feature p : A;
        var portion feature pv : A;
        portion feature all slices : A[1..*] subsets p;
    }
    class C specializes B {
        portion feature redefines p [1];
        portion redefines pv = p;
        derived const feature dk : A;
    }
}
