classdef Validation
%VALIDATION Every assertion about one object and its held objects.

    properties
        verdicts = {}
        summary = []
        instances = {}
        diagnostics = {}
        verifications = {}
        bounded = false
        standing
    end

    methods
        function obj = Validation(verdicts, summary, instances, diagnostics, verifications, bounded)
            if nargin < 1, verdicts = {}; end
            if nargin < 2, summary = []; end
            if nargin < 3, instances = []; end
            if nargin < 4, diagnostics = {}; end
            if nargin < 5, verifications = {}; end
            if nargin < 6, bounded = false; end
            obj.verdicts = toCells(verdicts);
            obj.summary = summary;
            obj.instances = toCells(instances);
            obj.diagnostics = toCells(diagnostics);
            obj.verifications = toCells(verifications);
            obj.bounded = logical(bounded);
            if isempty(summary), obj.standing = opensysml.Standing();
            else, obj.standing = summary.standing;
            end
        end

        function tf = valid(obj)
            tf = ~isempty(obj.summary) && obj.summary.holds && isempty(obj.summary.error);
        end

        function values = violated(obj)
            values = {};
            for i = 1:numel(obj.verdicts)
                verdict = obj.verdicts{i};
                if ~verdict.holds && isempty(verdict.error), values{end+1} = verdict; end
            end
        end

        function values = undecided(obj)
            values = {};
            for i = 1:numel(obj.verdicts)
                verdict = obj.verdicts{i};
                if ~isempty(verdict.error), values{end+1} = verdict; end
            end
        end

        function result = raiseForError(obj)
            for i = 1:numel(obj.verdicts)
                obj.verdicts{i}.raiseForError();
            end
            result = obj;
        end

        function text = explain(obj)
            lines = cellfun(@(v) v.explain(), obj.verdicts, 'UniformOutput', false);
            if ~isempty(obj.summary), lines{end+1} = obj.summary.explain(); end
            text = strjoin(lines, sprintf('\n'));
        end

        function tf = logical(obj)
            tf = obj.valid();
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
