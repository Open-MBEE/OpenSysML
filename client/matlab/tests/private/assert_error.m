function assert_error(fn, idPrefix, what)
%ASSERT_ERROR fn() must throw an MException whose id starts with idPrefix.

    if nargin < 3, what = 'call'; end
    try
        fn();
    catch e
        if strncmp(e.identifier, idPrefix, numel(idPrefix))
            return;
        end
        error('assert:error', '%s threw %s, want id %s*', what, e.identifier, idPrefix);
    end
    error('assert:error', '%s returned normally, want error %s*', what, idPrefix);
end
