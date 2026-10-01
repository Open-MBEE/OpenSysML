function raiseFailure(message, failureReason, diagnostics)
%RAISEFAILURE Raise the public client error for a service-classified failure.

    if nargin < 3, diagnostics = {}; end
    if strcmp(failureReason, 'FAILURE_REASON_WRONG_KIND')
        identifier = 'opensysml:diagnostics:wrongKind';
    else
        identifier = 'opensysml:diagnostics:execution';
    end
    opensysml.internal.raise(identifier, char(message), diagnostics, ...
        struct('failure', char(failureReason)));
end
