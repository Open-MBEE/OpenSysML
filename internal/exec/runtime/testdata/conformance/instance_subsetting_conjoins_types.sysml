// `front : Panel :> frames : Framed` holds a Panel that is a Framed too (KerML 1.0 §7.3.4.3–§7.3.4.4): it carries
// both's members, a shared name and an inherited redefinition are one feature, and `both : Panel, Framed` is every stated type.
package test {
    private import ScalarValues::*;

    item def Panel {
        attribute area : Real = 2.0;
        attribute weight : Real = 1.5;
    }
    item def Lined {
        attribute thickness : Real;
        attribute doubled : Real = thickness * 2.0;
    }
    item def Framed :> Lined {
        attribute depth :>> thickness = 0.5;
        attribute border : Real = 3.0;
        attribute weight : Real;
        item trim : Strip [1];
    }
    item def Strip {
        attribute length : Real = 9.0;
    }

    part def Holder {
        item frames : Framed [0..*];
        item front : Panel [1] :> frames;
        attribute frontArea : Real = front.area;
        attribute frontBorder : Real = front.border;
        attribute frontWeight : Real = front.weight;
        attribute trimLength : Real = front.trim.length;
        item spare : Strip [1];
        binding front.trim = spare;
        attribute frontDoubled : Real = front.doubled;
        item both : Panel, Framed [1];
        attribute bothArea : Real = both.area;
        attribute bothBorder : Real = both.border;
    }
    part holder : Holder;
}
