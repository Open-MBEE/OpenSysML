function text = conf_base64(bytes)
%CONF_BASE64 Standard base64 of a uint8 vector, on MATLAB and Octave alike.

    bytes = uint8(bytes(:)');
    if exist('OCTAVE_VERSION', 'builtin')
        text = base64_encode(bytes);
    else
        text = matlab.net.base64encode(bytes);
    end
end
