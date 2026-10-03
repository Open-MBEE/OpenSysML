part def Recorder {
    action def Label {
        first start then count;
        action count {
            out result : ScalarValues::Integer[1] = 42;
        }
        first count then toString;
        action toString {
            in x : ScalarValues::Integer[1];
            out result : ScalarValues::String[1] = IntegerFunctions::ToString(x);
        }
        first toString then concat;
        action concat {
            in x : ScalarValues::String[1] = "n=";
            in y : ScalarValues::String[1];
            out result : ScalarValues::String[1] = StringFunctions::'+'(x, y);
        }
        first concat then 'first';
        action 'first' {
            in seq : ScalarValues::String[0..*] ordered nonunique;
            in element : ScalarValues::String[1];
            out result : ScalarValues::String[0..*] ordered nonunique = SequenceFunctions::including(seq, element);
        }
        first 'first' then second;
        action second {
            in seq : ScalarValues::String[0..*] ordered nonunique;
            in element : ScalarValues::String[1] = "n=42";
            out result : ScalarValues::String[0..*] ordered nonunique = SequenceFunctions::including(seq, element);
        }
        first second then size;
        action size {
            in seq : ScalarValues::String[0..*] ordered nonunique;
            out result : ScalarValues::Integer[1] = SequenceFunctions::size(seq);
        }
        first size then flag;
        action flag {
            in x : ScalarValues::Boolean[1] = true;
            out result : ScalarValues::String[1] = BooleanFunctions::ToString(x);
        }
        first flag then position;
        action position {
            in seq : ScalarValues::String[0..*] ordered nonunique;
            in element : ScalarValues::String[1] = "n=";
            out result : ScalarValues::Integer[0..1];
            /* not migrated: CallBehaviorAction 'position' — the behavior Alf SequenceFunctions::IndexOf it calls has no v2 library function: the v2 library has no function giving the position of an element in a sequence; the behavior is known by its OMG href http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-IndexOf */
        }
        first position then 'after';
        action 'after' {
            in x : ScalarValues::Integer[0..1];
            in y : ScalarValues::Integer[1] = 1;
            out result : ScalarValues::Integer[0..1];
            /* not migrated: CallBehaviorAction 'after' — the pin 'x' it passes for the parameter x of fUML IntegerFunctions::plus, which must hold a value, receives none: 'position', which feeds it, produces no value; v1 never fires the call, so the action carries the token and performs nothing */
        }
        first 'after' then print;
        action print {
            in value : ScalarValues::String[1];
            /* not migrated: CallBehaviorAction 'print' — the behavior fUML BasicInputOutput::WriteLine it calls has no v2 library function: writes a line to the standard output channel, which the v2 library has no function for; the behavior is known by its OMG href http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#BasicInputOutput-WriteLine */
        }
        first print then final;
        action final terminate;
        succession flow of ScalarValues::Integer from count.result to toString.x;
        succession flow of ScalarValues::String from toString.result to concat.y;
        succession flow of ScalarValues::String from concat.result to 'first'.element;
        succession flow of ScalarValues::String from 'first'.result to second.seq;
        succession flow of ScalarValues::String from second.result to size.seq;
        succession flow of ScalarValues::String from second.result to position.seq;
        flow of ScalarValues::Integer from position.result to 'after'.x;
        succession flow of ScalarValues::String from concat.result to print.value;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
