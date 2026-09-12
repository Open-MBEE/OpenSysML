// A feature whose value chooses at run time between an object of a declaration on the path
// and another is judged by what reading it makes: `tine`, making a Wheel, is read and its
// wheel is in the extent; `stem`, making another Fork, is left as if unread, so the walk ends.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Wheel;
    part def Fork {
        part tine = if false ? new Fork() else new Wheel();
        part stem = if true ? new Fork() else new Wheel();
    }

    part rig {
        part fork : Fork;

        attribute wheelCount : Natural = size(all Wheel);
        attribute forkCount : Natural = size(all Fork);

        assert constraint tineFound { (all Wheel)#(1) === fork.tine }
    }
}
