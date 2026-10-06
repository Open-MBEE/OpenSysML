// House rules for -self-check-package: each constraint def's first in parameter
// is typed by a metaclass, so it applies to every element that metaclass
// conforms to. See README.md.
package Acme {
	package ModelingRules {
		private import SequenceFunctions::*;

		constraint def partDefinitionHasDocumentation {
			doc /* Every part definition carries documentation. */
			in pd : SysML::PartDefinition;
			not pd.documentation->isEmpty()
		}

		constraint def portUsageDeclaresDirection {
			doc /* Every port usage declares a direction. */
			in p : SysML::PortUsage;
			not p.direction->isEmpty()
		}
	}
}
