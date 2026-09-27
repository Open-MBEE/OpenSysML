function name = status_canonical(code)
%STATUS_CANONICAL Connect error code -> canonical gRPC name; 200 -> OK.

    if strcmp(char(code), 'OK')
        name = 'OK'; return;
    end
    name = upper(strrep(char(code), '_', '_'));
end
