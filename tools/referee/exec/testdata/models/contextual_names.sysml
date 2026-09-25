package Names {
    private import ScalarValues::*;

    attribute chain : Integer = 1;
    attribute pt = chain + 1;

    part def Link {
        attribute chain : Integer = 2;
        attribute next = chain * 3;
    }
    part link : Link;
    attribute viaPart = link.next;
}
