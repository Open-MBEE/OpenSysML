package ToasterDemo {
    private import ISQ::*;
    private import SI::*;
    private import VerificationCases::*;
    metadata def Safety;
    part def Toaster { attribute cycleTime : ISQ::DurationValue; }
    requirement def TimelyToast {
        subject toaster : Toaster;
        require constraint { toaster.cycleTime <= 180.0 [SI::s] }
    }
    requirement timely : TimelyToast;
    verification def TimelyToastTest {
        subject toaster : Toaster;
    }
}
