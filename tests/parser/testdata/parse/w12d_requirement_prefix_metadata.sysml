// A subject, actor or stakeholder takes prefix metadata after its keyword
// (SysML.xtext SubjectUsage, ActorUsage, StakeholderUsage).
package W12DRequirementPrefixMetadata {
	metadata def B;

	requirement def R {
		subject #B s;
		actor #B a;
		stakeholder #B st;
	}
}
