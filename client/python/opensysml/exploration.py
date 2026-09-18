"""What exploring a behavior under the ``explore`` scheduling policy found.

A behavior with more than one valid execution — two tokens the library does not
order, two guards holding at once, two transitions enabled by one event — has a
choice point wherever the executor picked. Under ``explore`` the service replays
the behavior from the start once per linearization, within a budget of runs and
of choice points per run, and answers with every distinct :class:`Outcome` the
runs reached rather than with one run's result. Hitting a budget makes the
:class:`Exploration` incomplete, naming the budget; it is never an error.
"""

from opensysml.errors import ExecutionError


class Outcome:
    """One distinct outcome an exploration reached.

    Two runs agreeing on their observables — an action's output values; the state
    a machine rests in, the states it entered and the values it holds; a case's
    outputs and verdicts — are one outcome, however differently they got there.
    A run that failed is an outcome of its own, carrying :attr:`error`.

    Attributes:
        outputs (dict): The values the behavior holds at the end, by name; a
            value the wire format cannot represent is an UnsupportedValueError in
            its place. Empty for a failed run
        final_state (str): The state a state machine rests in; empty for an action
        states_visited (list[str]): The states a state machine entered, in order
        error (str): Why the run failed; empty for a run that completed
        linearizations (int): How many of the explored orders reached this outcome
        witness (list[str]): The choices one run reaching it made, in run order;
            empty when the behavior had no choice point
        diagnostics (list[Diagnostic]): What the witness run reported, its choice
            points among them
    """

    def __init__(self, outputs, final_state, states_visited, error,
                 linearizations, witness, diagnostics):
        self.outputs = dict(outputs or {})
        self.final_state = final_state
        self.states_visited = list(states_visited or [])
        self.error = error
        self.linearizations = linearizations
        self.witness = list(witness or [])
        self.diagnostics = list(diagnostics or [])

    @property
    def failed(self):
        """Whether the runs reaching this outcome failed rather than completed."""
        return bool(self.error)

    def raise_for_error(self):
        """Raise the run's failure as an :class:`~opensysml.errors.ExecutionError`, if it failed."""
        if self.error:
            raise ExecutionError(self.error, diagnostics=self.diagnostics)

    def __str__(self):
        if self.error:
            return f"error: {self.error}"
        parts = []
        if self.final_state:
            parts.append(f"finalState {self.final_state}")
        if self.states_visited:
            parts.append("visits " + ", ".join(self.states_visited))
        parts.extend(f"{name} = {self.outputs[name]}" for name in sorted(self.outputs))
        return "; ".join(parts) if parts else "no outputs"

    def __repr__(self):
        return (
            f"Outcome({self!s}, linearizations={self.linearizations}, "
            f"witness={self.witness!r})"
        )


class Exploration:
    """Every distinct outcome a behavior reached under ``explore``, and how the exploration ended.

    Iterating an exploration yields its outcomes, in the service's canonical
    order, so the same model explores to the same sequence every time.

    Attributes:
        outcomes (list[Outcome]): The distinct outcomes reached
        complete (bool): Whether every linearization within the budget was run
        runs (int): How many runs were made
        budgets_hit (list[str]): The budgets the exploration ran into — ``"runs"``,
            ``"depth"`` — empty when it is complete
        runs_budget (int): The most runs the exploration would make
        depth_budget (int): The most choice points one run would resolve
    """

    def __init__(self, outcomes, complete, runs, budgets_hit, runs_budget, depth_budget):
        self.outcomes = list(outcomes or [])
        self.complete = complete
        self.runs = runs
        self.budgets_hit = list(budgets_hit or [])
        self.runs_budget = runs_budget
        self.depth_budget = depth_budget

    def __iter__(self):
        return iter(self.outcomes)

    def __len__(self):
        return len(self.outcomes)

    def __bool__(self):
        """An exploration is truthy when it is complete and no run failed."""
        return self.complete and not any(o.failed for o in self.outcomes)

    @property
    def status(self):
        """How the exploration ended, as ``sysml -schedule explore`` prints it."""
        if self.complete:
            return f"complete ({self.runs} runs)"
        named = " and ".join(
            f"{budget} budget {self.depth_budget if budget == 'depth' else self.runs_budget}"
            for budget in self.budgets_hit
        )
        return f"incomplete: {named} hit after {self.runs} runs"

    def raise_for_incomplete(self):
        """Raise an :class:`~opensysml.errors.ExecutionError` unless every linearization was run."""
        if not self.complete:
            raise ExecutionError(self.status)

    def __str__(self):
        lines = [
            f"{o!s} ({o.linearizations} linearizations; "
            f"{'; '.join(o.witness) or 'no choice points'})"
            for o in self.outcomes
        ]
        lines.append(self.status)
        return "\n".join(lines)

    def __repr__(self):
        return f"Exploration(outcomes={self.outcomes!r}, status={self.status!r})"
