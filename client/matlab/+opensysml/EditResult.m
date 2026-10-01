classdef EditResult < opensysml.Conversion
%EDITRESULT Edited notation and source ranges changed by an editor.

    properties
        applied = {}
        documents = {}
    end

    methods
        function obj = EditResult(content, applied, documents)
            if nargin < 1, content = ''; end
            if nargin < 2, applied = {}; end
            if nargin < 3, documents = {}; end
            obj@opensysml.Conversion(content, 'sysml', 'sysml');
            obj.applied = asCells(applied);
            obj.documents = asCells(documents);
        end

        function path = save(obj, path)
            path = obj.write(path);
        end
    end
end

function values = asCells(values)
    if isempty(values), values = {};
    elseif iscell(values), values = values(:)';
    elseif isstruct(values), values = num2cell(values(:)');
    else, values = {values};
    end
end
