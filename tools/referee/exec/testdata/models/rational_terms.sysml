package RationalTerms {
    private import ScalarValues::*;

    // A Rational is a binary64 on both sides; rat is the quotient `/` computes,
    // numer/denom the exact terms of that binary64.
    attribute third = RationalFunctions::rat(1, 3);
    attribute sixQuarters = RationalFunctions::rat(6, 4);
    attribute undefined = RationalFunctions::rat(1, 0);
    attribute numerOfThreeQuarters = RationalFunctions::numer(0.75);
    attribute denomOfThreeQuarters = RationalFunctions::denom(0.75);
    attribute numerOfSixQuarters = RationalFunctions::numer(RationalFunctions::rat(6, 4));
    attribute denomOfThird = RationalFunctions::denom(1.0 / 3.0);
    attribute numerOfTwo = RationalFunctions::numer(2);
    attribute denomOfTwo = RationalFunctions::denom(2);
    attribute numerOfTenth = RationalFunctions::numer(0.1);
    attribute denomOfTenth = RationalFunctions::denom(0.1);
    attribute thirdIsQuotient = RationalFunctions::rat(1, 3) == 1.0 / 3.0;
    attribute sixQuartersByOperator = 6 / 4;
}
