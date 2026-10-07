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
    end b : B {
        attribute memberKey : Key;
    }
}
