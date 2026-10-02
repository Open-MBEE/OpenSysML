function bytes = base64Decode(text)
%BASE64DECODE The bytes of standard base64 text, on MATLAB and Octave alike.

    text = char(text);
    text = text(~isspace(text));
    if isempty(text)
        bytes = zeros(1, 0, 'uint8');
        return;
    end
    if exist('OCTAVE_VERSION', 'builtin')
        bytes = decode_text(text);
    else
        bytes = uint8(matlab.net.base64decode(text));
    end
    bytes = uint8(bytes(:)');
end

function bytes = decode_text(text)
    alphabet = ['A':'Z' 'a':'z' '0':'9' '+' '/'];
    lookup = -ones(1, 256);
    lookup(double(alphabet) + 1) = 0:63;
    text = text(text ~= '=');
    values = lookup(double(text) + 1);
    if any(values < 0)
        opensysml.internal.raise('opensysml:decode', 'malformed base64 content');
    end
    bits = dec2bin(values, 6)';
    bits = bits(:)';
    count = floor(numel(bits) / 8);
    bytes = uint8(bin2dec(reshape(bits(1:count * 8), 8, count)'))';
end
