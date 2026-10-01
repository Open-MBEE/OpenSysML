classdef Exploration
%EXPLORATION Distinct outcomes and completeness of a behavior exploration.

    properties
        outcomes = {}
        complete = true
        runs = int64(0)
        budgetsHit = {}
        runsBudget = int64(0)
        depthBudget = int64(0)
        probabilitiesLowerBound = false
    end

    methods
        function obj = Exploration(outcomes, complete, runs, budgetsHit, ...
                runsBudget, depthBudget, probabilitiesLowerBound)
            if nargin >= 1, obj.outcomes = toCells(outcomes); end
            if nargin >= 2, obj.complete = logical(complete); end
            if nargin >= 3, obj.runs = int64(runs); end
            if nargin >= 4, obj.budgetsHit = toCells(budgetsHit); end
            if nargin >= 5, obj.runsBudget = int64(runsBudget); end
            if nargin >= 6, obj.depthBudget = int64(depthBudget); end
            if nargin >= 7, obj.probabilitiesLowerBound = logical(probabilitiesLowerBound); end
        end

        function text = status(obj)
            if obj.complete
                text = sprintf('complete (%d runs)', obj.runs);
                return;
            end
            labels = cell(1, numel(obj.budgetsHit));
            for i = 1:numel(obj.budgetsHit)
                budget = obj.budgetsHit{i};
                limit = obj.runsBudget;
                if strcmp(budget, 'depth'), limit = obj.depthBudget; end
                labels{i} = sprintf('%s budget %d', budget, limit);
            end
            text = sprintf('incomplete: %s hit after %d runs; probabilities are lower bounds', ...
                strjoin(labels, ' and '), obj.runs);
        end

        function result = raiseForIncomplete(obj)
            if ~obj.complete
                opensysml.internal.raise('opensysml:diagnostics:execution', obj.status());
            end
            result = obj;
        end

        function tf = ok(obj)
            tf = obj.complete;
            for i = 1:numel(obj.outcomes)
                tf = tf && ~obj.outcomes{i}.failed;
            end
        end

        function text = explain(obj)
            lines = cell(1, numel(obj.outcomes) + 1);
            for i = 1:numel(obj.outcomes)
                outcome = obj.outcomes{i};
                witness = outcome.witness;
                if isempty(witness), witnessText = 'no choice points';
                else, witnessText = strjoin(witness, '; ');
                end
                lines{i} = sprintf('%s (%d linearizations; %s)', ...
                    outcomeText(outcome), outcome.linearizations, witnessText);
            end
            lines{end} = obj.status();
            text = strjoin(lines, sprintf('\n'));
        end

        function n = length(obj)
            n = numel(obj.outcomes);
        end
    end
end

function cells = toCells(raw)
    if isempty(raw), cells = {};
    elseif iscell(raw), cells = raw(:)';
    elseif isstruct(raw), cells = num2cell(raw(:)');
    else, cells = {raw};
    end
end

function text = outcomeText(outcome)
    if outcome.failed
        text = ['error: ' outcome.error];
        return;
    end
    parts = {};
    if ~isempty(outcome.finalState), parts{end+1} = ['finalState ' outcome.finalState]; end
    if ~isempty(outcome.statesVisited)
        parts{end+1} = ['visits ' strjoin(outcome.statesVisited, ', ')];
    end
    names = fieldnames(outcome.outputs);
    names = sort(names);
    for i = 1:numel(names)
        parts{end+1} = sprintf('%s = %s', names{i}, valueText(outcome.outputs.(names{i})));
    end
    if isempty(parts), text = 'no outputs'; else, text = strjoin(parts, '; '); end
end

function text = valueText(value)
    if ischar(value), text = value;
    elseif isnumeric(value) || islogical(value), text = num2str(value);
    else, text = evalc('disp(value)'); text = strtrim(text);
    end
end
