function options = editOptions(defaults, positionalNames, varargin)
%EDITOPTIONS Read optional positional values and lowerCamel name-value pairs.

    names = fieldnames(defaults);
    options = defaults;
    args = varargin;
    count = [];
    if ~isempty(args) && mod(numel(args), 2) == 0 && ...
            isOptionName(args{1}, names) && validPairs(args, names)
        count = 0;
    else
        for candidate = 0:min(numel(args), numel(positionalNames))
            remaining = args(candidate+1:end);
            if mod(numel(remaining), 2) ~= 0 || ...
                    ~validPairs(remaining, names) || ...
                    ~validPositionals(args(1:candidate), positionalNames, defaults)
                continue;
            end
            if candidate == 0 && ~isempty(args) && ~isOptionName(args{1}, names)
                continue;
            end
            count = candidate;
            break;
        end
    end
    if isempty(count)
        if mod(numel(args), 2) == 0 && ~isempty(args) && ...
                (ischar(args{1}) || (isstring(args{1}) && isscalar(args{1}))) && ...
                ~isOptionName(args{1}, names)
            opensysml.internal.raise('opensysml:argument', sprintf( ...
                'editor has no option ''%s''', char(args{1})));
        end
        opensysml.internal.raise('opensysml:argument', ...
            'editor options must be name-value pairs');
    end
    for i = 1:count
        options.(positionalNames{i}) = args{i};
    end
    args = args(count+1:end);
    if mod(numel(args), 2) ~= 0
        opensysml.internal.raise('opensysml:argument', ...
            'editor options must be name-value pairs');
    end
    for i = 1:2:numel(args)
        if ~(ischar(args{i}) || (isstring(args{i}) && isscalar(args{i})))
            opensysml.internal.raise('opensysml:argument', ...
                'editor option names must be text');
        end
        name = char(args{i});
        if ~any(strcmp(names, name))
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('editor has no option ''%s''', name));
        end
        options.(name) = args{i+1};
    end
end

function tf = validPairs(args, names)
    tf = mod(numel(args), 2) == 0;
    if ~tf, return; end
    for i = 1:2:numel(args)
        if ~isOptionName(args{i}, names)
            tf = false;
            return;
        end
    end
end

function tf = validPositionals(args, positionalNames, defaults)
    tf = true;
    for i = 1:numel(args)
        if i > numel(positionalNames) || ~isfield(defaults, positionalNames{i})
            tf = false;
            return;
        end
        default = defaults.(positionalNames{i});
        value = args{i};
        if islogical(default) && (~islogical(value) || ~isscalar(value))
            tf = false;
            return;
        end
        if ischar(default) && ~(ischar(value) || ...
                (isstring(value) && isscalar(value)) || isempty(value))
            tf = false;
            return;
        end
        if isTextField(positionalNames{i}) && ~(ischar(value) || ...
                (isstring(value) && isscalar(value)) || isempty(value))
            tf = false;
            return;
        end
    end
end

function tf = isTextField(name)
    names = {'type', 'multiplicity', 'value', 'direction', 'expression', ...
        'doc', 'name', 'kind', 'after', 'ref', 'action', 'via', 'to', ...
        'visibility', 'filter', 'trigger', 'guard', 'effect', 'source', ...
        'target', 'collection', 'variable', 'condition', 'until', ...
        'occurrence', 'requirement', 'metadataType', 'locale', 'body'};
    tf = any(strcmp(names, name));
end

function tf = isOptionName(value, names)
    tf = (ischar(value) || (isstring(value) && isscalar(value))) && ...
        any(strcmp(names, char(value)));
end
