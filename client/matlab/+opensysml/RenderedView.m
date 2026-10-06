classdef RenderedView
%RENDEREDVIEW Machine-readable data from a named or targeted pseudo-view.

    properties (SetAccess = private)
        view
        kind
        stated
        nodes
        edges
        columns
        rows
        canvas
        notes
        notices
    end

    methods
        function result = RenderedView(answer)
            result.view = fieldOr(answer, 'view', '');
            result.kind = fieldOr(answer, 'kind', '');
            result.stated = fieldOr(answer, 'stated', '');
            rawNodes = sequence(fieldOr(answer, 'nodes', []));
            result.nodes = struct([]);
            for i = 1:numel(rawNodes)
                node = rawNodes{i};
                rawPorts = sequence(fieldOr(node, 'ports', []));
                ports = repmat(struct('id', '', 'name', '', 'type', '', 'direction', ''), ...
                    1, numel(rawPorts));
                for j = 1:numel(rawPorts)
                    ports(j) = struct('id', fieldOr(rawPorts{j}, 'id', ''), ...
                        'name', fieldOr(rawPorts{j}, 'name', ''), ...
                        'type', fieldOr(rawPorts{j}, 'type', ''), ...
                        'direction', fieldOr(rawPorts{j}, 'direction', ''));
                end
                origin = optionalSpan(fieldOr(node, 'origin', []));
                geometry = optionalGeometry(fieldOr(node, 'geometry', []));
                style = optionalStyle(fieldOr(node, 'style', []));
                value = struct('id', fieldOr(node, 'id', ''), ...
                    'kind', fieldOr(node, 'kind', ''), 'name', fieldOr(node, 'name', ''), ...
                    'nameSynthesized', fieldOr(node, 'nameSynthesized', false), ...
                    'type', fieldOr(node, 'type', ''), 'detail', fieldOr(node, 'detail', ''), ...
                    'text', fieldOr(node, 'text', ''), 'standIn', fieldOr(node, 'standIn', false), ...
                    'parent', fieldOr(node, 'parent', ''), 'ports', ports, ...
                    'origin', origin, 'geometry', geometry, 'style', style);
                result.nodes = appendStruct(result.nodes, value);
            end
            rawEdges = sequence(fieldOr(answer, 'edges', []));
            result.edges = struct([]);
            for i = 1:numel(rawEdges)
                edge = rawEdges{i};
                rawRoute = sequence(fieldOr(edge, 'route', []));
                route = repmat(struct('x', 0, 'y', 0), 1, numel(rawRoute));
                for j = 1:numel(rawRoute)
                    route(j) = struct('x', fieldOr(rawRoute{j}, 'x', 0), ...
                        'y', fieldOr(rawRoute{j}, 'y', 0));
                end
                value = struct('from', fieldOr(edge, 'from', ''), ...
                    'to', fieldOr(edge, 'to', ''), 'fromPort', fieldOr(edge, 'fromPort', ''), ...
                    'toPort', fieldOr(edge, 'toPort', ''), 'label', fieldOr(edge, 'label', ''), ...
                    'name', fieldOr(edge, 'name', ''), 'kind', fieldOr(edge, 'kind', ''), ...
                    'origin', optionalSpan(fieldOr(edge, 'origin', [])), 'route', route, ...
                    'style', optionalStyle(fieldOr(edge, 'style', [])));
                result.edges = appendStruct(result.edges, value);
            end
            result.columns = stringSequence(fieldOr(answer, 'columns', {}));
            rawRows = sequence(fieldOr(answer, 'rows', []));
            result.rows = struct([]);
            for i = 1:numel(rawRows)
                row = rawRows{i};
                result.rows = appendStruct(result.rows, struct( ...
                    'cells', {stringSequence(fieldOr(row, 'cells', {}))}, ...
                    'origin', optionalSpan(fieldOr(row, 'origin', []))));
            end
            canvas = fieldOr(answer, 'canvas', []);
            if isempty(canvas)
                result.canvas = [];
            else
                result.canvas = struct('unit', fieldOr(canvas, 'unit', ''), ...
                    'width', fieldOr(canvas, 'width', 0), 'height', fieldOr(canvas, 'height', 0), ...
                    'hasSize', fieldOr(canvas, 'hasSize', false));
            end
            rawNotes = sequence(fieldOr(answer, 'notes', []));
            result.notes = struct([]);
            for i = 1:numel(rawNotes)
                note = rawNotes{i};
                value = struct('text', fieldOr(note, 'text', ''), ...
                    'anchor', fieldOr(note, 'anchor', ''), ...
                    'edgeFrom', fieldOr(note, 'edgeFrom', ''), ...
                    'edgeTo', fieldOr(note, 'edgeTo', ''), ...
                    'x', fieldOr(note, 'x', 0), 'y', fieldOr(note, 'y', 0), ...
                    'width', fieldOr(note, 'width', 0), 'height', fieldOr(note, 'height', 0), ...
                    'hasSize', fieldOr(note, 'hasSize', false), ...
                    'origin', optionalSpan(fieldOr(note, 'origin', [])));
                result.notes = appendStruct(result.notes, value);
            end
            result.notices = stringSequence(fieldOr(answer, 'notices', {}));
        end
    end
end

function value = fieldOr(record, name, fallback)
    if isstruct(record) && isfield(record, name), value = record.(name);
    else, value = fallback;
    end
end

function values = sequence(value)
    if isempty(value), values = {};
    elseif iscell(value), values = value;
    else, values = num2cell(value);
    end
end

function values = stringSequence(value)
    if isempty(value), values = {};
    elseif iscell(value), values = cellfun(@char, value, 'UniformOutput', false);
    elseif ischar(value), values = cellstr(value);
    elseif isstring(value), values = cellstr(value);
    else, values = cellstr(string(value));
    end
end

function result = appendStruct(values, value)
    if isempty(values), result = value;
    else, result = [values, value];
    end
end

function result = optionalSpan(value)
    if isempty(value), result = [];
    else
        result = struct('file', fieldOr(value, 'file', ''), ...
            'startLine', fieldOr(value, 'startLine', 0), ...
            'startCol', fieldOr(value, 'startCol', 0), ...
            'endLine', fieldOr(value, 'endLine', 0), ...
            'endCol', fieldOr(value, 'endCol', 0));
    end
end

function result = optionalGeometry(value)
    if isempty(value), result = [];
    else
        result = struct('x', fieldOr(value, 'x', 0), 'y', fieldOr(value, 'y', 0), ...
            'width', fieldOr(value, 'width', 0), 'height', fieldOr(value, 'height', 0), ...
            'hasSize', fieldOr(value, 'hasSize', false), ...
            'collapsed', fieldOr(value, 'collapsed', false));
    end
end

function result = optionalStyle(value)
    if isempty(value), result = [];
    else
        result = struct('fill', fieldOr(value, 'fill', ''), ...
            'line', fieldOr(value, 'line', ''), 'text', fieldOr(value, 'text', ''), ...
            'font', fieldOr(value, 'font', ''), ...
            'fontSize', fieldOr(value, 'fontSize', 0), ...
            'bold', fieldOr(value, 'bold', false), ...
            'italic', fieldOr(value, 'italic', false));
    end
end
