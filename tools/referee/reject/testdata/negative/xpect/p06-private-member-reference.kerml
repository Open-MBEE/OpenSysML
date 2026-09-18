// Invalid: a private member is not visible outside its namespace (KerML 8.2.3.5 name resolution; pilot Import_All.kerml.xt).
package P06PrivateMemberReference {
    package A {
        private class X;
    }
    class Y specializes A::X;
}
