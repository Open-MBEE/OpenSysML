part def Recorder {
    action def Label {
        first start then count;
        action count {
            out result : ScalarValues::Integer = 42;
        }
        first count then toString;
        action toString {
            in x : ScalarValues::Integer;
            out result : ScalarValues::String = IntegerFunctions::ToString(x);
        }
        first toString then concat;
        action concat {
            in x : ScalarValues::String = "n=";
            in y : ScalarValues::String;
            out result : ScalarValues::String = StringFunctions::'+'(x, y);
        }
        first concat then 'fork';
        fork 'fork';
        first 'fork' then 'first';
        first 'fork' then join2;
        action 'first' {
            in seq : ScalarValues::String[0..*] ordered nonunique;
            in element : ScalarValues::String;
            out result : ScalarValues::String[0..*] ordered nonunique = SequenceFunctions::including(seq, element);
        }
        first 'first' then second;
        action second {
            in seq : ScalarValues::String[0..*] ordered nonunique;
            in element : ScalarValues::String = "n=42";
            out result : ScalarValues::String[0..*] ordered nonunique = SequenceFunctions::including(seq, element);
        }
        first second then fork2;
        fork fork2;
        first fork2 then size;
        first fork2 then 'join';
        action size {
            in seq : ScalarValues::String[0..*] ordered nonunique;
            out result : ScalarValues::Integer = SequenceFunctions::size(seq);
        }
        first size then flag;
        action flag {
            in x : ScalarValues::Boolean = true;
            out result : ScalarValues::String = BooleanFunctions::ToString(x);
        }
        first flag then 'join';
        join 'join';
        first 'join' then position;
        action position {
            in seq : ScalarValues::String[0..*] ordered nonunique;
            in element : ScalarValues::String = "n=";
            out result : ScalarValues::Integer[0..1];
            /* not migrated: CallBehaviorAction 'position' — the behavior Alf SequenceFunctions::IndexOf it calls has no v2 library function: the v2 library has no function giving the position of an element in a sequence; the behavior is known by its OMG href http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-IndexOf */
        }
        first position then 'after';
        action 'after' {
            in x : ScalarValues::Integer;
            in y : ScalarValues::Integer = 1;
            out result : ScalarValues::Integer;
            /* not migrated: CallBehaviorAction 'after' — the pin 'x' it passes for the parameter x of fUML IntegerFunctions::plus, which must hold a value, receives none: 'position', which feeds it, produces no value; v1 never fires the call, so the action carries the token and performs nothing */
        }
        first 'after' then join2;
        join join2;
        first join2 then print;
        action print {
            in value : ScalarValues::String;
            /* not migrated: CallBehaviorAction 'print' — the behavior fUML BasicInputOutput::WriteLine it calls has no v2 library function: writes a line to the standard output channel, which the v2 library has no function for; the behavior is known by its OMG href http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#BasicInputOutput-WriteLine */
        }
        first print then final;
        action final terminate;
        flow count.result to toString.x;
        flow toString.result to concat.y;
        flow concat.result to 'first'.element;
        flow 'first'.result to second.seq;
        flow second.result to size.seq;
        flow second.result to position.seq;
        /* flow position.result to 'after'.x not written: 'position' is not migrated and produces no value */
        flow concat.result to print.value;
    }
}
