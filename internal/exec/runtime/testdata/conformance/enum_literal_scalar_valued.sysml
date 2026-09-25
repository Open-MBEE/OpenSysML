// An enumeration specializing a scalar type gives its literals values of that
// type, so such a literal is that value and computes as one. A literal declaring
// no value keeps its own identity, and an inherited one stays a literal of the
// enumeration declaring it.
package test {
    private import ScalarValues::Real;

    enum def GradePoints :> Real { A = 4.0; B = 3.0; }

    enum def Color { red; green; }

    enum def PaintColor :> Color { white; }

    part def Report {
        attribute best : GradePoints = GradePoints::A;
        attribute margin = GradePoints::A - GradePoints::B;
        attribute passing = GradePoints::B == 3.0;
        attribute paint : PaintColor = PaintColor::white;
        attribute inherited : Color = PaintColor::red;
        attribute inheritedIsItsOwn = PaintColor::red == Color::red;
    }
}
