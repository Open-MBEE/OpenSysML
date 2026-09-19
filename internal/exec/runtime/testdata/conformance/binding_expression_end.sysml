package test {
    part def Sys {
        attribute a = 5;
        attribute b;
        attribute sibling = 8;
        attribute b2 = a + 1;
        binding bind b = b2;
    }
}
