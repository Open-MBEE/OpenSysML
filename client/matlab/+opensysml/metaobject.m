function value = metaobject(elementId, metaclassId)
%METAOBJECT Construct a metaobject value.

    if nargin < 2, metaclassId = ''; end
    value = struct('elementId', char(elementId), 'metaclassId', char(metaclassId));
end
