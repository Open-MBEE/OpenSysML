library package DocumentQueries {
	/* NON-NORMATIVE OpenSysML document-query vocabulary;
	 * not part of the OMG SysML or KerML standard library. */

	private import KerML::Root::Element;
	private import ScalarValues::*;

	abstract calc def Query {
		return result : Element[0..*] ordered;
	}

	calc def OwnedElements {
		in source : Element[0..*] ordered;
		return result : Element[0..*] ordered;
	}

	calc def Descendants {
		in source : Element[0..*] ordered;
		in maxDepth : Integer[1];
		return result : Element[0..*] ordered;
	}

	calc def Ancestors {
		in source : Element[0..*] ordered;
		in maxDepth : Integer[1];
		return result : Element[0..*] ordered;
	}

	/* Objects: the objects the session holds that are of the named type,
	 * each nested object under its path (`car.wheels[2]`). Requires a session. */
	calc def Objects {
		in type : String[1];
		return result : Element[0..*] ordered;
	}

	/* Verdicts: one row per assertion checked on each source row's object and the
	 * objects it holds (a session object as it stands, an element as declared);
	 * kind selects all, constraint, requirement, satisfaction or verification. */
	calc def Verdicts {
		in source : Element[0..*] ordered;
		in kind : String[1] = "all";
		return result : Element[0..*] ordered;
	}

	/* The properties of a Verdicts row, for Project and column expressions
	 * (`Verdict::verdict`); a row also answers its assertion's own properties by
	 * name. The assertion and the carrier it was checked on are elements, so a
	 * Column expression cannot read them; project them by name instead. */
	attribute def Verdict {
		ref assertion : Element[1];
		attribute kind : String[1];
		ref carrier : Element[0..1];
		attribute path : String[1];
		attribute verdict : String[1];
		attribute condition : String[0..1];
		attribute reason : String[0..1];
		attribute 'verification' : String[0..*] ordered;
	}

	/* States: one row per active leaf state of each source row's object (a
	 * session object, or an element whose objects the session holds), machines
	 * in the object's order and leaves in region order; an object with no state
	 * machine is refused. Requires a session. */
	calc def States {
		in source : Element[0..*] ordered;
		return result : Element[0..*] ordered;
	}

	/* InState: the objects the session holds whose state machine is in the
	 * named state, a leaf or a state enclosing one, by name or dotted path
	 * (`on.run`); a name no machine declares is refused. Requires a session. */
	calc def InState {
		in name : String[1];
		return result : Element[0..*] ordered;
	}

	/* The properties of a States row, for Project and column expressions
	 * (`State::name`); a row also answers its state declaration's own
	 * properties. object and 'state' are elements: project them by name. */
	attribute def State {
		ref object : Element[1];
		attribute path : String[1];
		attribute machine : String[1];
		attribute name : String[1];
		attribute statePath : String[1];
		ref 'state' : Element[0..1];
		attribute region : String[1];
		attribute enclosing : String[0..*] ordered;
	}

	/* Events: the session's trace as a relation in the order recorded, one row
	 * per accept, send, transition fired, state entry, exit or do step, choice
	 * point drawn or guard left unevaluated; source keeps the rows of the given
	 * objects (every object when null), kind the kinds named (`all`, or kinds
	 * separated by `,`), since and before the instants in [since, before) as
	 * durations or bare numbers of the clock's seconds. Requires a session
	 * recording a trace. */
	calc def Events {
		in source : Element[0..*] ordered = null;
		in kind : String[1] = "all";
		in since : ScalarValue[0..1] = null;
		in before : ScalarValue[0..1] = null;
		return result : Element[0..*] ordered;
	}

	/* The properties of an Events row, for Project and column expressions
	 * (`Event::time`, `Event::'event'`); a row also answers the properties of
	 * the behavior that made it. object and target are elements: project them
	 * by name. */
	attribute def Event {
		attribute kind : String[1];
		attribute time : ScalarValue[1];
		ref object : Element[0..1];
		attribute path : String[0..1];
		attribute machine : String[0..1];
		attribute 'state' : String[0..1];
		attribute 'from' : String[0..1];
		attribute 'to' : String[0..1];
		ref target : Element[0..1];
		attribute 'event' : String[0..1];
		attribute payload : String[0..*] ordered;
		attribute alternatives : String[0..*] ordered;
		attribute taken : String[0..1];
		attribute text : String[1];
	}

	calc def RelatedElements {
		in source : Element[0..*] ordered;
		/* relationshipKind: "specialization", "subsetting", "redefinition",
		 * "typing", "connection", "allocation", "satisfaction" or
		 * "verification"; direction: "outgoing" or "incoming". */
		in relationshipKind : String[1];
		in direction : String[1];
		in maxDepth : Integer[1];
		return result : Element[0..*] ordered;
	}

	calc def WhereType {
		in source : Element[0..*] ordered;
		in type : String[1];
		return result : Element[0..*] ordered;
	}

	calc def WhereMetadata {
		in source : Element[0..*] ordered;
		in 'metadata' : String[1];
		return result : Element[0..*] ordered;
	}

	calc def WhereName {
		in source : Element[0..*] ordered;
		in operator : String[1];
		in value : String[1];
		return result : Element[0..*] ordered;
	}

	calc def WhereFeature {
		in source : Element[0..*] ordered;
		in 'feature' : String[1];
		in operator : String[1];
		in value : String[1];
		return result : Element[0..*] ordered;
	}

	calc def OrderBy {
		in source : Element[0..*] ordered;
		in property : String[1];
		in direction : String[1];
		in missing : String[1];
		in multiple : String[1];
		return result : Element[0..*] ordered;
	}

	/* One computed table column: an explicit name plus a calc expression
	 * evaluated on each projected row element. */
	attribute def ColumnSpec;

	/* A column expression references row properties by their declared
	 * feature (e.g. Subsystem::mass), supports +, -, *, / arithmetic,
	 * string concatenation with +, and ?? for absent-value defaults.
	 * A row not conforming to the feature's declaring type reads it as
	 * absent; a failing expression, an absent result not defaulted
	 * with ??, or a multi-valued result fails the query with a typed
	 * error naming the row and column. */
	calc def Column {
		in name : String[1];
		in expression : ScalarValue[0..1];
		return result : ColumnSpec[1];
	}

	calc def Project {
		in source : Element[0..*] ordered;
		in properties : String[0..*] ordered = null;
		in columns : ColumnSpec[0..*] ordered = null;
		return result : Element[0..*] ordered;
	}

	abstract part def Document {
		attribute title : String[1];
	}

	abstract part def Section {
		attribute title : String[1];
	}

	abstract part def ContentBlock;

	part def Paragraph :> ContentBlock {
		attribute text : String[0..1];
	}

	/* Inline runs compose a paragraph in declaration order, joined by
	 * single spaces. */
	abstract part def Run;

	part def Span :> Run {
		attribute text : String[1];
		/* style: "plain", "emphasis", "strong", "code" or "math"; a math
		 * span's text is LaTeX, typeset inline. */
		attribute style : String[0..1];
	}

	part def Link :> Run {
		attribute text : String[1];
		attribute target : String[1];
	}

	/* Ref links to a named content block of this or another document, or to
	 * another document itself; text defaults to the target's title, caption
	 * or name. */
	part def Ref :> Run {
		attribute text : String[0..1];
		ref target : Element[1];
	}

	/* Column runs style a query-backed paragraph's or list's text: each
	 * result row renders one run per column run, in declaration order. */
	abstract part def ColumnRun;

	part def SpanColumn :> ColumnRun {
		/* column: the projected column whose values become the run text. */
		attribute column : String[1];
		/* style: "plain", "emphasis", "strong", "code" or "math". */
		attribute style : String[0..1];
		/* styleColumn: a projected column supplying each row's style. */
		attribute styleColumn : String[0..1];
	}

	part def LinkColumn :> ColumnRun {
		/* column: the projected column whose values become the link text. */
		attribute column : String[1];
		/* targetColumn: a projected column supplying each row's one link
		 * destination. */
		attribute targetColumn : String[1];
	}

	part def Table :> ContentBlock {
		attribute caption : String[0..1];
		/* groupBy: a projected column name; rows are grouped by its value
		 * into subtables in order of first appearance. */
		attribute groupBy : String[0..1];
	}

	part def List :> ContentBlock {
		attribute style : String[0..1];
	}

	/* Definitions renders each row of its query as one term with its
	 * description(s): prose from the model rather than a table of it. */
	part def Definitions :> ContentBlock {
		/* term: the projected column whose value names each entry. */
		attribute term : String[1];
		/* description: the projected column whose values describe it. */
		attribute description : String[1];
	}

	/* Formula typesets one LaTeX expression as display mathematics on a line
	 * of its own; a named formula is a Ref target. */
	part def Formula :> ContentBlock {
		attribute source : String[1];
		attribute caption : String[0..1];
	}

	part def Diagram :> ContentBlock {
		attribute caption : String[0..1];
		/* kind: a supported rendering kind ("tree", "interconnection",
		 * "state", "action", "table" or "sequence"); required when source
		 * is a plain element, stated by the view when source is a view. */
		attribute kind : String[0..1];
		/* direction: "TB", "LR", "RL" or "BT", for kinds drawn as graphs. */
		attribute direction : String[0..1];
		/* palette: "okabe-ito", "tol-bright", "tol-muted", "tol-light",
		 * "brewer-set2", "brewer-dark2", "viridis" or "cividis"; fills the
		 * nodes of a DOT or PlantUML diagram by keyword family, black and white
		 * when absent. */
		attribute palette : String[0..1];
		ref source : Element[0..1];
	}

	/* WhereRelated: the source rows with at least one element reachable by the
	 * relationship kind and direction of RelatedElements within maxDepth
	 * (exists = true), or with none (exists = false). */
	calc def WhereRelated {
		in source : Element[0..*] ordered;
		in relationshipKind : String[1];
		in direction : String[1];
		in maxDepth : Integer[1];
		in exists : Boolean[1] = true;
		return result : Element[0..*] ordered;
	}

	/* Except: the source rows whose element, object, verdict, state or event
	 * is not among exclude, once each in source order. */
	calc def Except {
		in source : Element[0..*] ordered;
		in exclude : Element[0..*] ordered;
		return result : Element[0..*] ordered;
	}

	/* Union: the source rows, then the rows of other not already present, in
	 * order with duplicates removed. */
	calc def Union {
		in source : Element[0..*] ordered;
		in other : Element[0..*] ordered;
		return result : Element[0..*] ordered;
	}
}
