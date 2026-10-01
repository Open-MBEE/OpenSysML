function value = elementRef(id, elementType)
%ELEMENTREF Reference a model element by qualified name.

    if nargin < 2, elementType = ''; end
    value = struct('type', 'element', 'id', char(id), ...
        'elementType', char(elementType));
end
