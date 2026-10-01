function err = lastError()
%LASTERROR Return details for the most recently raised client error.

    err = opensysml.internal.errorStore('get');
end
