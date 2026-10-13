attribute def Key;
occurrence def A {
    ref occurrence b : B {
        attribute memberKey : Key;
    }
}
occurrence def B;
connection def Link {
    end a : A {
        attribute ownedKey : Key;
    }
    end b : B crosses a.b;
}
occurrence a : A;
occurrence b : B;
connection link : Link connect a to b;
