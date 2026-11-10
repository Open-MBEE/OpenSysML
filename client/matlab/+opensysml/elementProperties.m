function answer = elementProperties(model, element, varargin)
%ELEMENTPROPERTIES Read an element's metamodel properties by specification name.
%   answer = opensysml.elementProperties(model, 'Demo::Pair', 'inheritedFeature', 'isAbstract')
%   answers the KerML/SysML properties named, as the model's api-json export spells
%   them; with no names every property the element form carries. The element is a
%   qualified name, the positional id of an unnamed member ('Owner::@2') or the id an
%   earlier answer carried. The answer has 'element' (id, qualifiedName, metaclass),
%   'properties' (a cell, in the order asked for) and 'byName' (a containers.Map from
%   property name to its record: name, definingClass, many, derived, values, unsupported).
%   Requires the element_properties capability.

    if nargin < 2 || isempty(element)
        opensysml.internal.raise('opensysml:argument', 'an element id or qualified name is required');
    end
    properties = cell(1, numel(varargin));
    for i = 1:numel(varargin)
        properties{i} = char(varargin{i});
    end
    model.connection.require('element_properties');
    request = struct('modelHash', model.hash, 'elementId', char(element));
    if ~isempty(properties)
        request.properties = properties;
    end
    raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
        'GetElementProperties', request, {'element_properties'}), 'GetElementProperties');
    answer = opensysml.internal.decodeElementProperties(raw);
end
