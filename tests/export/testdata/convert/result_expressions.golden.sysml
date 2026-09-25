package Results {
    attribute def Real;
    attribute def Boolean;
    calc def Double {
        in x : Real;
        x * 2
    }
    calc def Only {
        42
    }
    calc def AfterMembers {
        in x : Real;
        doc /* the answer */
        attribute y : Real = x + 1;
        y * y
    }
    calc def Body {
        in x : Real;
        { in y : Real; y + x }
    }
    calc def Quoted {
        in x : Real;
        { in 'the input' : Real; 'the input' + x }
    }
    calc def Nested {
        in x : Real;
        { in y : Real; Double(x = { in z : Real; z + y }) }
    }
    calc def Documented {
        in x : Real;
        { doc /* the parameter, unchanged */ in y : Real; y }
    }
    calc def Passes {
        in ok : Boolean;
        ok
    }
    calc def Says {
        "say \"hi\"\n\\ \t done"
    }
    calc def Empty {
        in x : Real;
        {}
    }
    calc def Named {
        in x : Real;
        Double('the value' = x)
    }
    calc def Scaled {
        in x : Real;
        x * 1.5E3
    }
    calc def Predicate {
        in x : Real;
        in expr keep : Boolean { in v : Real; v > x }
        in expr always { true }
        keep(x)
    }
    calc def Chained {
        in x : Real;
        Double(x = Double(x)).result
    }
    calc def Conditional {
        in x : Real;
        if x > 0 ? x else -x
    }
    calc def Widened {
        in x : Real;
        (as Real[2])
    }
    calc def Ranged {
        in x : Real;
        (as Real[1..*])
    }
    part def Vehicle {
        attribute mass : Real;
    }
    analysis def MassAnalysis {
        subject v : Vehicle;
        v.mass
    }
    verification def MassCheck {
        subject v : Vehicle;
        Passes(v.mass > 0)
    }
    case def MassCase {
        subject v : Vehicle;
        v.mass * 2
    }
    part rover : Vehicle {
        calc twice {
            in x = mass;
            x + x
        }
    }
}
