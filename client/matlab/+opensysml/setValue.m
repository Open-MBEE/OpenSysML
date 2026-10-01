function value = setValue(elements)
%SETVALUE Construct an unordered set value.

    if nargin < 1, elements = {}; end
    if ~iscell(elements)
        opensysml.internal.raise('opensysml:argument', 'setValue expects a cell array');
    end
    value = struct('set', {elements(:)'});
end
