package W6D {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    private import BaseFunctions::*;
    private import ControlFunctions::*;
    private import NumericalFunctions::*;
    private import VectorFunctions::*;
    private import ComplexFunctions::*;
    private import ISQ::*;
    private import SI::*;

    // Instantiation and feature values: a multi-valued default on an
    // undeclared multiplicity, a required lower bound with no value, and a
    // valueless value-typed feature.
    attribute undeclared = (1.0, 2.0);
    attribute undeclaredTyped : Real = (1.0, 2.0);
    attribute lowerThree : Real[3];
    attribute unboundedLower : Real[*..*];
    attribute starOnly : Real[*];
    attribute valueless : Real;

    // The same shapes reached through an instantiated part, which is where the
    // assumed 1..1 and eager materialization of a required lower bound apply.
    part def Holder {
        attribute xs = (1.0, 2.0);
        attribute ys : Real[2] = (1.0, 2.0);
        attribute zs : Real[3];
        attribute one : Real = 1.0;
    }
    part holder : Holder;

    // A subsetting part is one of the objects the subsetted feature holds.
    part def Sub { attribute mass : Real default 1.0; }
    part def Sys {
        part subsystem : Sub[*];
        part a : Sub :> subsystem;
        attribute total : Real default sum(subsystem.mass);
    }
    part sys : Sys;

    // A body on a redefining declaration valuing features the value it
    // inherits would supply: `v` is valued in the body, `w` is not.
    attribute def Cost {
        attribute v : Real default 1.0;
        attribute w : Real default 2.0;
    }
    attribute template : Cost {
        attribute :>> v default 5.0;
        attribute :>> w default 6.0;
    }
    part def Ring { attribute cost : Cost default template; }
    part r : Ring { attribute :>> cost { attribute :>> v default 11.0; } }
    part plain : Ring;

    // Sequence indexing and collection operations.
    calc def IncAt { return : Integer[*] = (1, 2, 3)->includingAt(2, 9); }
    calc def IncAtAppends { return : Integer[*] = (1, 2, 3)->includingAt(4, 9); }
    calc def IncAtOutOfRange { return : Integer[*] = (1, 2, 3)->includingAt(7, 9); }
    calc def ExclAt { return : Integer[*] = (1, 2, 3)->excludingAt(2); }
    calc def ExclAtOutOfRange { return : Integer[*] = (1, 2, 3)->excludingAt(4); }
    calc def Subseq { return : Integer[*] = (1, 2, 3)->subsequence(2, 3); }
    calc def SubseqOutOfRange { return : Integer[*] = (1, 2, 3)->subsequence(1, 4); }
    calc def SubseqEmptyRange { return : Integer[*] = (1)->subsequence(2, 1); }

    // Quantity aggregation.
    attribute qs : MassValue[*] = (3.0 [SI::kg], 1.0 [SI::kg]);
    attribute qsMixed : MassValue[*] = (3.0 [SI::kg], 500.0 [SI::g]);
    calc def SumQ { return : MassValue = sum(W6D::qs); }
    calc def SumQMixed { return : MassValue = sum(W6D::qsMixed); }
    calc def SumEmpty { return : Real = sum(()); }
    calc def AddBareToMeasured { return : MassValue = 3.0 [SI::kg] + 1.0; }
    attribute qsNone : MassValue[*];
    calc def SumEmptyQuantity { return : MassValue = sum(W6D::qsNone); }
    calc def AddEmptyQuantitySum { return : MassValue = 3.0 [SI::kg] + sum(W6D::qsNone); }

    // A `default null` collection holds the parts that subset it; the pilot
    // folds the default instead, and counts any undefaulted collection as one.
    part def SysDefaulted {
        part subsystem : Sub[*] default null;
        part a : Sub :> subsystem;
        part b : Sub :> subsystem;
        attribute n : Natural = size(subsystem);
    }
    part def SysTwo {
        part subsystem : Sub[*];
        part a : Sub :> subsystem;
        part b : Sub :> subsystem;
        attribute n : Natural = size(subsystem);
    }
    part def SysNone {
        part subsystem : Sub[*];
        attribute n : Natural = size(subsystem);
    }
    part sysDefaulted : SysDefaulted;
    part sysTwo : SysTwo;
    part sysNone : SysNone;

    // KerML function library: VectorFunctions and ComplexFunctions.
    calc def VecElements { return : Real[*] nonunique = CartesianThreeVectorOf((1.0, 2.0, 2.0)).elements; }
    calc def VecIsZero { return : Boolean = isCartesianZeroVector(CartesianThreeVectorOf((0.0, 0.0, 0.0))); }
    calc def VecNormCartesian { return : Real = cartesianNorm(CartesianThreeVectorOf((1.0, 2.0, 2.0))); }
    calc def VecNormAbstract { return : Real = norm(CartesianThreeVectorOf((1.0, 2.0, 2.0))); }
    calc def VecInner { return : Real = cartesianInner(CartesianThreeVectorOf((1.0, 2.0, 3.0)), CartesianThreeVectorOf((4.0, 5.0, 6.0))); }
    calc def VecAdd { return : Real[*] = 'cartesian+'(CartesianThreeVectorOf((1.0, 2.0, 3.0)), CartesianThreeVectorOf((4.0, 5.0, 6.0))).elements; }
    calc def CxRe { return : Real = re(rect(3.0, 4.0)); }
    calc def CxAbs { return : Real = abs(rect(3.0, 4.0)); }
    calc def CxMulRe { return : Real = re(rect(0.0, 1.0) * rect(0.0, 1.0)); }
    calc def CxIsZero { return : Boolean = isZero(rect(0.0, 0.0)); }
    calc def CxAbsQualified { return : Real = ComplexFunctions::abs(rect(3.0, 4.0)); }
    calc def CxIsZeroQualified { return : Boolean = ComplexFunctions::isZero(rect(0.0, 0.0)); }
    calc def CxMulQualified { return : Real = re(ComplexFunctions::'*'(rect(0.0, 1.0), rect(0.0, 1.0))); }

    // A calc usage's outputs.
    calc def Add { in x : Real; in y : Real; return : Real = x + y; }
    calc addUse : Add { in x = 1.0; in y = 2.0; }

    // Classification: `istype` conformance against `hastype` identity.
    attribute n7 : Integer = 7;
    attribute nat3 : Natural = 3;
    attribute r25 : Real = 2.5;
    attribute str : String = "s";
    attribute seqInt : Integer[*] = (1, 2, 3);
    attribute none : Integer[0..1];

    calc def IsIntInteger { return : Boolean = W6D::n7 istype Integer; }
    calc def HasIntInteger { return : Boolean = W6D::n7 hastype Integer; }
    calc def IsIntReal { return : Boolean = W6D::n7 istype Real; }
    calc def HasIntReal { return : Boolean = W6D::n7 hastype Real; }
    calc def IsIntNatural { return : Boolean = W6D::n7 istype Natural; }
    calc def IsNatInteger { return : Boolean = W6D::nat3 istype Integer; }
    calc def HasNatInteger { return : Boolean = W6D::nat3 hastype Integer; }
    calc def IsRealInteger { return : Boolean = W6D::r25 istype Integer; }
    calc def IsStrString { return : Boolean = W6D::str istype String; }
    calc def IsIntString { return : Boolean = W6D::n7 istype String; }
    calc def IsLiteralInteger { return : Boolean = 3 istype Integer; }
    calc def IsLiteralReal { return : Boolean = 3 istype Real; }
    calc def IsSeqInteger { return : Boolean = W6D::seqInt istype Integer; }
    calc def IsMixedSeqInteger { return : Boolean = (1, "a") istype Integer; }
    calc def IsEmptyInteger { return : Boolean = W6D::none istype Integer; }
    calc def IsCarCar { return : Boolean = W6D::car istype Car; }
    calc def IsCarVehicle { return : Boolean = W6D::car istype Vehicle; }
    calc def HasCarVehicle { return : Boolean = W6D::car hastype Vehicle; }
    calc def HasCarCar { return : Boolean = W6D::car hastype Car; }
    calc def IsSysCar { return : Boolean = W6D::sys istype Car; }
    part def Vehicle;
    part def Car :> Vehicle;
    part car : Car;
}
