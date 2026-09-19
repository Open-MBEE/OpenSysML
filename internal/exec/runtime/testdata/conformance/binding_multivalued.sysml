package test {
    part def Sys {
        attribute a : Integer[*] = (1, 2);
        attribute b : Integer[*];
        binding bind b = a;
    }
}
