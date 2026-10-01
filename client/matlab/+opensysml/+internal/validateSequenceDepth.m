function validateSequenceDepth(items, depth)
%VALIDATESEQUENCEDEPTH Enforce the service's nested action-body limit.

    if nargin < 2, depth = 0; end
    for i = 1:numel(items)
        if depth > 128
            opensysml.internal.raise('opensysml:argument', ...
                'nested action-body items exceed the maximum depth');
        end
        item = items{i};
        for name = {'body', 'elseBody'}
            field = name{1};
            if isfield(item, field) && ~isempty(item.(field))
                opensysml.internal.validateSequenceDepth(item.(field), depth + 1);
            end
        end
    end
end
