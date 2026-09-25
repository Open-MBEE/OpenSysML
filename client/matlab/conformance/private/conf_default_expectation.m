function tf = conf_default_expectation(expected)
%CONF_DEFAULT_EXPECTATION An expectation subtree the absence of the field
%satisfies: null, "", false, 0, an empty list, or a map of only defaults.

    tf = false;
    if isempty(expected)
        tf = true; return;
    end
    if islogical(expected) && ~expected
        tf = true; return;
    end
    if ischar(expected) && isempty(expected)
        tf = true; return;
    end
    if isnumeric(expected) && isscalar(expected) && ~islogical(expected) && expected == 0
        tf = true; return;
    end
    if iscell(expected) && isempty(expected)
        tf = true; return;
    end
    if isa(expected, 'containers.Map')
        vals = expected.values;
        tf = all(cellfun(@conf_default_expectation, vals));
        return;
    end
end
