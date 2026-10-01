function result = decodeDocumentResult(raw)
%DECODEDOCUMENTRESULT Decode a RunDocumentQuery response.

    columns = toCells(fieldOr(raw, 'columns', {}));
    names = cell(1, numel(columns));
    for i = 1:numel(columns), names{i} = columns{i}.name; end
    rawRows = toCells(fieldOr(raw, 'rows', {}));
    rows = cell(1, numel(rawRows));
    for i = 1:numel(rawRows)
        rows{i} = decodeRow(rawRows{i});
    end
    result = opensysml.DocumentQueryResult(names, rows);
end

function row = decodeRow(raw)
    if isstruct(raw.element) && hasTypedRowValue(raw.element)
        element = opensysml.internal.decodeDocumentValue(raw.element);
    else
        element = decodeElement(raw.element);
    end
    cells = toCells(fieldOr(raw, 'cells', {}));
    decodedCells = cell(1, numel(cells));
    for i = 1:numel(cells)
        values = toCells(fieldOr(cells{i}, 'values', {}));
        decodedCells{i} = cellfun(@opensysml.internal.decodeDocumentValue, ...
            values, 'UniformOutput', false);
    end
    verdict = [];
    object = [];
    state = [];
    event = [];
    switch fieldOr(element, 'type', '')
        case 'verdict'
            verdict = element;
            element = verdict.assertion;
        case 'object'
            object = element;
            element = object.element;
        case 'state'
            state = element;
            object = state.object;
            element = object.element;
        case 'event'
            event = element;
            object = event.object;
            if isempty(object), element = opensysml.elementRef(''); else, element = object.element; end
    end
    row = struct('element', element, 'cells', {decodedCells}, ...
        'verdict', verdict, 'object', object, 'state', state, 'event', event);
end

function tf = hasTypedRowValue(raw)
    arms = {'object', 'verdict', 'state', 'event'};
    tf = false;
    for i = 1:numel(arms)
        if isfield(raw, arms{i}), tf = true; return; end
    end
end

function value = decodeElement(raw)
    if isstruct(raw) && isfield(raw, 'elementId')
        value = opensysml.internal.decodeDocumentValue(raw);
    else
        elementType = '';
        if isstruct(raw) && isfield(raw, 'elementType'), elementType = raw.elementType; end
        value = opensysml.elementRef('', elementType);
    end
end

function value = fieldOr(record, name, fallback)
    if isstruct(record) && isfield(record, name), value = record.(name);
    else, value = fallback;
    end
end

function cells = toCells(raw)
    if isempty(raw), cells = {};
    elseif iscell(raw), cells = raw(:)';
    elseif isstruct(raw), cells = num2cell(raw(:)');
    else, cells = {raw};
    end
end
