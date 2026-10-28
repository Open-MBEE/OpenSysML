function test_token_offline()
%TEST_TOKEN_OFFLINE The bearer token an external connection carries: the
%'token' option, else $OPENSYSML_GRPC_TOKEN; a private child never gets one.

    previous = getenv('OPENSYSML_GRPC_TOKEN');
    cleanup = onCleanup(@() setenv('OPENSYSML_GRPC_TOKEN', previous));

    setenv('OPENSYSML_GRPC_TOKEN', '');
    conn = opensysml.external('127.0.0.1:1');
    assert_equal(conn.token, '', 'no token by default');
    conn = opensysml.external('127.0.0.1:1', 'token', 'matlab-token_1');
    assert_equal(conn.token, 'matlab-token_1', 'the token option');

    setenv('OPENSYSML_GRPC_TOKEN', 'env-token_1');
    conn = opensysml.external('127.0.0.1:1', 'version', 'v9');
    assert_equal(conn.token, 'env-token_1', 'the environment token');
    conn = opensysml.external('127.0.0.1:1', 'token', 'matlab-token_1');
    assert_equal(conn.token, 'matlab-token_1', 'the option wins over the environment');

    % A token that is not RFC 6750 token syntax never reaches curl's command line.
    assert_error(@() opensysml.internal.httpPost('http://127.0.0.1:1/x', '{}', 1, 'bad token'), ...
        'opensysml:transport', 'malformed token refused');

    % private has no token option; connect drops it on the way to a private child.
    assert_error(@() opensysml.private('token', 'x', 'binary', '/nonexistent/sysml-grpc'), ...
        'opensysml:argument', 'private has no token option');
end
