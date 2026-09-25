"""Tests for reading a host:port address written as the host.

``connect("localhost:50123")`` names an address, and used to build the target
``localhost:50123:50051`` and report the service unreachable — which sends a
reader to the service rather than to the call.
"""

import pytest
from unittest.mock import Mock, patch

import opensysml
from opensysml.connection import DEFAULT_PORT, Connection, split_target
from opensysml.generate import main as generate_main


class TestSplitTarget:
    def test_a_host_alone_keeps_the_port_given(self):
        assert split_target("localhost") == ("localhost", DEFAULT_PORT)
        assert split_target("example.internal", 50123) == (
            "example.internal", 50123
        )

    def test_an_address_names_its_own_port(self):
        assert split_target("localhost:50123") == ("localhost", 50123)

    def test_an_address_agreeing_with_the_port_given_is_accepted(self):
        assert split_target("localhost:50123", 50123) == ("localhost", 50123)

    def test_an_address_disagreeing_with_the_port_given_is_rejected(self):
        with pytest.raises(ValueError) as exc_info:
            split_target("localhost:50123", 50999)
        assert "different ports" in str(exc_info.value)

    def test_the_standard_port_written_out_disagrees_like_any_other(self):
        # A port given as 50051 was given, not omitted, so it disagrees too.
        with pytest.raises(ValueError) as exc_info:
            split_target("localhost:50123", DEFAULT_PORT)
        assert "different ports" in str(exc_info.value)

    def test_an_unreadable_port_names_the_mistake(self):
        with pytest.raises(ValueError) as exc_info:
            split_target("localhost:grpc")
        assert "localhost:grpc" in str(exc_info.value)
        assert "separately" in str(exc_info.value)

    def test_a_bare_ipv6_address_is_a_host(self):
        # Its colons are the address's own, so none of them names a port.
        assert split_target("::1") == ("::1", DEFAULT_PORT)
        assert split_target("fe80::1", 50123) == ("fe80::1", 50123)

    def test_a_bracketed_ipv6_address_names_its_port(self):
        assert split_target("[::1]:50123") == ("[::1]", 50123)
        assert split_target("[::1]") == ("[::1]", DEFAULT_PORT)


class TestConnectionTarget:
    def _connection(self, *args, **kwargs):
        stub = Mock()
        with patch('grpc.insecure_channel'):
            with patch(
                'opensysml.proto.sysml_pb2_grpc.SysMLServiceStub',
                return_value=stub,
            ):
                return Connection(*args, auto_start=False, **kwargs)

    def test_an_address_connects_to_the_port_it_names(self):
        conn = self._connection("localhost:50123")
        assert (conn.host, conn.port) == ("localhost", 50123)
        assert conn._address == "localhost:50123"

    def test_connect_reads_an_address_too(self):
        with patch('grpc.insecure_channel'):
            with patch('opensysml.proto.sysml_pb2_grpc.SysMLServiceStub'):
                conn = opensysml.connect("localhost:50123", auto_start=False)
        assert (conn.host, conn.port) == ("localhost", 50123)

    def test_a_disagreeing_port_is_rejected_before_any_call(self):
        with pytest.raises(ValueError):
            self._connection("localhost:50123", 50999)
        with pytest.raises(ValueError):
            self._connection("localhost:50123", DEFAULT_PORT)


class TestModuleHelpersReadAnAddress:
    """The module-level helpers taking host/port read an address the same way."""

    def setup_method(self):
        opensysml._default_connection = None
        opensysml._default_connection_params = None

    def teardown_method(self):
        opensysml._default_connection = None
        opensysml._default_connection_params = None

    def test_helpers_reject_a_disagreeing_port(self):
        for call in (
            lambda: opensysml.load("demo.sysml", "localhost:50123", 50999),
            lambda: opensysml.evaluate("1+1", model_hash="h", host="localhost:50123",
                                     port=50999),
            lambda: opensysml.instantiate("Demo::v", model_hash="h",
                                        host="localhost:50123", port=50999),
            lambda: opensysml.convert("sysml", model_hash="h",
                                    host="localhost:50123", port=50999),
            lambda: opensysml.load("demo.sysml", "localhost:50123", DEFAULT_PORT),
        ):
            with pytest.raises(ValueError):
                call()

    def test_the_generator_cli_lets_an_address_carry_its_port(self, tmp_path):
        # --port unwritten must stay unwritten, or --host's port disagrees with it.
        source = tmp_path / "demo.sysml"
        source.write_text("package Demo;\n")
        recorded = {}

        def record(host='localhost', port=None):
            recorded['target'] = (host, port)
            raise KeyboardInterrupt

        with patch('opensysml.connect', record):
            with pytest.raises(KeyboardInterrupt):
                generate_main([str(source), "--host", "localhost:50123"])
        assert recorded['target'] == ("localhost:50123", None)

    def test_the_generator_cli_reports_a_disagreement_as_an_error(self, tmp_path,
                                                                  capsys):
        # The library raises; the CLI names the mistake rather than tracing back.
        source = tmp_path / "demo.sysml"
        source.write_text("package Demo;\n")
        code = generate_main(
            [str(source), "--host", "localhost:50123", "--port", "50051"]
        )
        assert code == 2
        stderr = capsys.readouterr().err
        assert stderr.startswith("error: ")
        assert "different ports" in stderr

    def test_an_address_reaches_the_port_it_names(self):
        # The address is read where the target is: by the connection itself.
        with patch('opensysml.Connection') as connection:
            opensysml.load("demo.sysml", "localhost:50123")
        assert connection.call_args[0] == ("localhost:50123", None)
