function assert_equal(actual, expected, what)
%ASSERT_EQUAL isequaln check with the comparison shown on failure.

    if nargin < 3, what = 'value'; end
    if ~isequaln(actual, expected)
        error('assert:equal', '%s: got %s, want %s', what, show_val(actual), show_val(expected));
    end
end

function s = show_val(v)
    if ischar(v), s = ['"' v '"'];
    elseif isnumeric(v) || islogical(v), s = mat2str(v);
    else, s = sprintf('<%s>', class(v)); end
end
