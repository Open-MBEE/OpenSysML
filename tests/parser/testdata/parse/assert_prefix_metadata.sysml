// An assertion's prefix metadata ends the occurrence prefix ahead of `assert`,
// before its `not` (SysML.xtext AssertConstraintUsage `OccurrenceUsagePrefix
// 'assert' ( isNegated ?= 'not' )?`), unlike `assume`/`require`, which take it
// after the keyword (RequirementConstraintUsage).
package AssertPrefixMetadata {
    metadata def Safety;
    constraint def Stopped;
    requirement touchdown;
    part lander;

    part context {
        #Safety assert constraint ok : Stopped;
        #Safety assert not constraint bad : Stopped;
        ref #Safety assert not constraint { 1 < 2 }
        #Safety assert not satisfy touchdown by lander;
    }
    requirement def R {
        assume #Safety constraint { true }
        require #Safety constraint braked : Stopped;
    }
}
