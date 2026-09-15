- **A transition's `accept` trigger payload is a member of the transition.** The parameter an
  accept trigger declares (`transition t first a accept p : Payload then b;`) was catalogued in a
  scope of its own, so it had no owner, no qualified name and — for the one such parameter in the
  standard library, `Actions::AcceptAction::aState::aTransition::apayload` — no normative id, the
  last named library element whose id differed from the pilot's XMI. The symbol index now defines
  it in the transition's own scope beside the effect and body members, so `t::p` names it, the
  normative catalog derives its id under the transition, and `TestPilotLibraryXMI` lists no
  pilot-only element. The guard, effect and body still reach it as before; any other reference
  to it reports `Must be an accessible feature`, as the pilot does.
