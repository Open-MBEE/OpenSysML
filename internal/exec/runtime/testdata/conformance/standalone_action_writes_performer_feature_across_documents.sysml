// A part performing an action definition declared on its own, in another document, does
// not lend its features to that body: the performing object is not a namespace the body's
// names resolve in, so the write of `touched` is refused as it is when both are declared in
// one document — the owner-scope reach of `part_owned_action_writes_owner_across_documents`
// is for a body the owning definition holds, not for any body an object performs.
package test {
    private import ScalarValues::*;

    part def Host {
        attribute touched : Integer = 0;
    }

    part host : Host;
}
