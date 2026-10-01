function item = sequenceEdit(owner, keyword, ref, memberKind, memberName, typeName, after, extra)
%SEQUENCEEDIT Build an AddSequenceEdit payload.

    if nargin < 8, extra = struct(); end
    item = struct('owner', owner, 'keyword', keyword, 'ref', ref, ...
        'memberKind', memberKind, 'memberName', memberName, ...
        'type', typeName, 'after', after);
    names = fieldnames(extra);
    for i = 1:numel(names)
        item.(names{i}) = extra.(names{i});
    end
end
