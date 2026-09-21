part def Recorder {
    action def Tally {
        first start then count;
        action count {
            out result : ScalarValues::Integer = 3;
        }
        first count then toString;
        action toString {
            in x : ScalarValues::Integer;
            out result : ScalarValues::String = IntegerFunctions::ToString(x);
        }
        first toString then label;
        action label {
            in x : ScalarValues::String = "n=";
            in y : ScalarValues::String;
            out result : ScalarValues::String = StringFunctions::'+'(x, y);
        }
        first label then 'fork';
        fork 'fork';
        first 'fork' then size;
        first 'fork' then 'join';
        action size {
            in list : ScalarValues::String[0..*] ordered nonunique;
            out result : ScalarValues::Integer = SequenceFunctions::size(list);
        }
        first size then 'join';
        join 'join';
        first 'join' then get;
        action get {
            in list : ScalarValues::String[0..*] ordered nonunique;
            in index : ScalarValues::Integer = 1;
            out result : ScalarValues::String = SequenceFunctions::'#'(list, index);
        }
        first get then print;
        action print {
            in value : ScalarValues::String;
            /* not migrated: CallBehaviorAction 'print' — the behavior fUML BasicInputOutput::WriteLine it calls has no v2 library function: writes a line to the standard output channel, which the v2 library has no function for; the behavior is known by the referentPath fUML_Library::BasicInputOutput::WriteLine recorded beside its href into the library module fUML-Library.mdzip */
        }
        first print then final;
        action final terminate;
        flow count.result to toString.x;
        flow toString.result to label.y;
        flow label.result to size.list;
        flow label.result to get.list;
        flow get.result to print.value;
    }
}
