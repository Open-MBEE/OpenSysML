classdef DocumentQueryResult
%DOCUMENTQUERYRESULT Projected columns and typed document-query rows.

    properties
        columns = {}
        rows = {}
    end

    methods
        function obj = DocumentQueryResult(columns, rows)
            if nargin >= 1, obj.columns = textCells(columns); end
            if nargin >= 2, obj.rows = valueCells(rows); end
        end

        function n = length(obj)
            n = numel(obj.rows);
        end

        function n = numel(obj)
            n = numel(obj.rows);
        end
    end
end

function values = textCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    else, values = cellstr(raw(:));
    end
end

function values = valueCells(raw)
    if isempty(raw), values = {};
    elseif iscell(raw), values = raw(:)';
    elseif isstruct(raw), values = num2cell(raw(:)');
    else, values = {raw};
    end
end
