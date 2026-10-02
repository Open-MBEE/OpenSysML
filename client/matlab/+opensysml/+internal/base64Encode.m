function text = base64Encode(bytes)
%BASE64ENCODE Standard base64 of a uint8 vector, on MATLAB and Octave alike.

    bytes = uint8(bytes(:)');
    if exist('OCTAVE_VERSION', 'builtin')
        text = base64_encode(bytes);
    else
        text = matlab.net.base64encode(bytes);
    end
end
