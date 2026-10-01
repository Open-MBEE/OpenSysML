function value = unicodeChar(code)
%UNICODECHAR Return one Unicode character on MATLAB and Octave.

    if exist('OCTAVE_VERSION', 'builtin')
        if code <= 127
            bytes = code;
        elseif code <= 2047
            bytes = [192 + floor(code / 64), 128 + mod(code, 64)];
        elseif code <= 65535
            bytes = [224 + floor(code / 4096), ...
                128 + mod(floor(code / 64), 64), 128 + mod(code, 64)];
        else
            bytes = [240 + floor(code / 262144), ...
                128 + mod(floor(code / 4096), 64), ...
                128 + mod(floor(code / 64), 64), 128 + mod(code, 64)];
        end
        value = native2unicode(uint8(bytes), 'UTF-8');
    else
        value = char(code);
    end
end
