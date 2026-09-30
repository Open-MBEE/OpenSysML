// A control-node keyword after `then` followed by a name declares a node of
// that kind, whether or not the body declares a member with the keyword's
// name; a multiplicity ahead of it is the source end's. Only the bare
// `then fork;` references such a member, and `then done;` the member `done`.
action def BesideMembersNamedAfterTheKeywords {
    action a;
    action fork;
    action join;
    action merge;
    action decide;
    action done;
    first start;
    then a;
    then fork F;
    then [0..1] join J;
    then merge M { doc /* m */ }
    then decide D;
    then fork G { action inner; }
    then fork { action anonymous; }
    then fork;
    then done;
}

action def WithoutSuchMembers {
    action a;
    first start;
    then a;
    then fork F;
    then [0..1] join J;
    then merge M { doc /* m */ }
    then decide D;
    then fork G { action inner; }
    then fork { action anonymous; }
    then fork;
    then done;
}

action def BesideAQuotedMember {
    action a;
    action 'fork';
    first start;
    then a;
    then fork F;
    then 'fork';
    then done;
}
