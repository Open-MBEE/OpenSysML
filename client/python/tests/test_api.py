"""Tests for opensysml module-level convenience API."""

import warnings

import pytest
from unittest.mock import patch, MagicMock, Mock
import opensysml


@pytest.fixture
def no_default_connection(monkeypatch):
    """No connection is cached when the test starts, nor left behind when it ends."""
    monkeypatch.setattr(opensysml, "_default_connection", None)
    monkeypatch.setattr(opensysml, "_default_connection_params", None)


class TestModuleLevelAPI:
    """Test module-level convenience functions."""
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_load(self):
        """Test opensysml.load() uses default connection."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            mock_model = MagicMock()
            mock_conn.load.return_value = mock_model
            MockConnection.return_value = mock_conn
            
            # Call load
            result = opensysml.load("test.sysml")
            
            # No port named, so the connection starts a private child of its own.
            MockConnection.assert_called_once_with('localhost', None, auto_start=True)
            
            # Should delegate to Connection.load(), not asking for strict loading
            mock_conn.load.assert_called_once_with(
                "test.sysml", strict=False, strict_conformance=False)
            
            assert result == mock_model
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_load_with_custom_host_port(self):
        """Test opensysml.load() with custom host and port."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            mock_model = MagicMock()
            mock_conn.load.return_value = mock_model
            MockConnection.return_value = mock_conn
            
            # Call load with custom params
            result = opensysml.load("test.sysml", host="example.com", port=9999)
            
            # Should create connection with custom params
            MockConnection.assert_called_once_with('example.com', 9999, auto_start=True)
            
            mock_conn.load.assert_called_once_with(
                "test.sysml", strict=False, strict_conformance=False)
            assert result == mock_model

    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_loads_with_custom_host_port(self):
        """Test opensysml.loads() with custom host and port."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            mock_model = MagicMock()
            mock_conn.load_from_content.return_value = mock_model
            MockConnection.return_value = mock_conn

            result = opensysml.loads(
                "package Demo;", host="example.com", port=9999,
                language="sysml", strict=True,
            )

            MockConnection.assert_called_once_with(
                'example.com', 9999, auto_start=True
            )
            mock_conn.load_from_content.assert_called_once_with(
                "package Demo;", strict=True, language="sysml",
                strict_conformance=False
            )
            assert result == mock_model
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_load_reuses_default_connection(self):
        """Test opensysml.load() reuses default connection for same host/port."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            mock_model1 = MagicMock()
            mock_model2 = MagicMock()
            mock_conn.load.side_effect = [mock_model1, mock_model2]
            MockConnection.return_value = mock_conn
            
            # Call load twice with same host/port
            result1 = opensysml.load("test1.sysml")
            result2 = opensysml.load("test2.sysml")
            
            # Should create connection only once
            MockConnection.assert_called_once()
            
            # Both loads should use same connection
            assert mock_conn.load.call_count == 2
            assert result1 == mock_model1
            assert result2 == mock_model2
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_load_recreates_connection_on_param_change(self):
        """Test opensysml.load() creates new connection when host/port changes."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn1 = MagicMock()
            mock_conn2 = MagicMock()
            mock_model1 = MagicMock()
            mock_model2 = MagicMock()
            mock_conn1.load.return_value = mock_model1
            mock_conn2.load.return_value = mock_model2
            MockConnection.side_effect = [mock_conn1, mock_conn2]
            
            # Call load with different host/port
            result1 = opensysml.load("test1.sysml", host="h1", port=1111)
            result2 = opensysml.load("test2.sysml", host="h2", port=2222)
            
            # Should create two connections
            assert MockConnection.call_count == 2
            MockConnection.assert_any_call('h1', 1111, auto_start=True)
            MockConnection.assert_any_call('h2', 2222, auto_start=True)
            
            # Each load uses its own connection
            mock_conn1.load.assert_called_once_with(
                "test1.sysml", strict=False, strict_conformance=False)
            mock_conn2.load.assert_called_once_with(
                "test2.sysml", strict=False, strict_conformance=False)
            assert result1 == mock_model1
            assert result2 == mock_model2
    
    def test_opensysml_connect(self):
        """Test opensysml.connect() creates new Connection."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            MockConnection.return_value = mock_conn
            
            # Call connect
            result = opensysml.connect()
            
            # No port named, so the connection starts a private child of its own.
            MockConnection.assert_called_once_with(
                'localhost', None, auto_start=True, version=None,
                require_capabilities=None,
            )
            
            assert result == mock_conn
    
    def test_opensysml_connect_with_custom_params(self):
        """Test opensysml.connect() with custom host, port, auto_start."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = MagicMock()
            MockConnection.return_value = mock_conn
            
            # Call connect with custom params
            result = opensysml.connect(host="example.com", port=9999, auto_start=False)
            
            # Should create connection with custom params
            MockConnection.assert_called_once_with(
                'example.com', 9999, auto_start=False, version=None,
                require_capabilities=None,
            )
            
            assert result == mock_conn
    
    def test_opensysml_connect_creates_new_instance(self):
        """Test opensysml.connect() creates new instance each time."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn1 = MagicMock()
            mock_conn2 = MagicMock()
            MockConnection.side_effect = [mock_conn1, mock_conn2]
            
            # Call connect twice
            result1 = opensysml.connect()
            result2 = opensysml.connect()
            
            # Should create two instances
            assert MockConnection.call_count == 2
            
            # Should return different instances
            assert result1 == mock_conn1
            assert result2 == mock_conn2
            assert result1 != result2
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_evaluate_with_file(self):
        """Test module-level evaluate() loads file."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            mock_model = Mock()
            mock_model.hash = "model-abc"
            mock_conn.load.return_value = mock_model
            mock_conn.eval.return_value = 42
            
            result = opensysml.evaluate("6 * 7", file_path="test.sysml")
            
            assert result == 42
            mock_conn.load.assert_called_once_with("test.sysml")
            mock_conn.eval.assert_called_once_with(
                "6 * 7", "model-abc", context_symbol_id=None, subject_symbol_id=None
            )
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_eval_still_works_and_warns(self):
        """The deprecated name evaluates the same way, warning about itself."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            mock_conn.eval.return_value = 42

            with pytest.warns(DeprecationWarning, match="opensysml.evaluate"):
                result = opensysml.eval("6 * 7", model_hash="model-abc")

            assert result == 42
        # The name to write is exported; the deprecated one is not.
        assert "evaluate" in opensysml.__all__
        assert "eval" not in opensysml.__all__

    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_evaluate_takes_the_address_positionally(self):
        """subject comes last, so a positional call still binds host and port."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            mock_conn.eval.return_value = 4

            opensysml.evaluate("2 + 2", None, "model-abc", None, "localhost", 50123)

            assert MockConnection.call_args.args[:2] == ("localhost", 50123)

    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_instantiate_with_hash(self):
        """Test module-level instantiate() with model_hash."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            mock_instance = Mock()
            mock_instance.id = 999
            mock_conn.instantiate.return_value = mock_instance
            
            result = opensysml.instantiate("Part", model_hash="hash-xyz")
            
            assert result.id == 999
            mock_conn.instantiate.assert_called_once_with("Part", "hash-xyz")
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_evaluate_missing_both_params(self):
        """Test evaluate() raises ValueError if neither file_path nor model_hash."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            with pytest.raises(ValueError, match="Must provide either"):
                opensysml.evaluate("2 + 2")
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_evaluate_with_both_params(self):
        """Test evaluate() raises ValueError when both file_path and model_hash provided."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            with pytest.raises(ValueError, match="Provide either file_path or model_hash, not both"):
                opensysml.evaluate("2 + 2", file_path="test.sysml", model_hash="hash-abc")
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_evaluate_with_context(self):
        """Test evaluate() passes context_symbol_id through to conn.eval()."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            mock_model = Mock()
            mock_model.hash = "model-xyz"
            mock_conn.load.return_value = mock_model
            mock_conn.eval.return_value = 100
            
            result = opensysml.evaluate("x + y", file_path="test.sysml", context_symbol_id="ctx-123")
            
            assert result == 100
            mock_conn.load.assert_called_once_with("test.sysml")
            # Verify context_symbol_id passes through
            mock_conn.eval.assert_called_once_with(
                "x + y",
                "model-xyz",
                context_symbol_id="ctx-123",
                subject_symbol_id=None,
            )

    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_eval_with_subject(self):
        """Test eval() passes subject through to conn.eval()."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn

            mock_conn.eval.return_value = 1200.0

            result = opensysml.evaluate(
                "mass", model_hash="model-xyz", subject="Demo::sedan"
            )

            assert result == 1200.0
            mock_conn.eval.assert_called_once_with(
                "mass",
                "model-xyz",
                context_symbol_id=None,
                subject_symbol_id="Demo::sedan",
            )
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_instantiate_missing_both_params(self):
        """Test instantiate() raises ValueError if neither file_path nor model_hash."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            with pytest.raises(ValueError, match="Must provide either"):
                opensysml.instantiate("Part")
    
    @pytest.mark.usefixtures("no_default_connection")
    def test_opensysml_instantiate_with_both_params(self):
        """Test instantiate() raises ValueError when both file_path and model_hash provided."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            
            with pytest.raises(ValueError, match="Provide either file_path or model_hash, not both"):
                opensysml.instantiate("Part", file_path="test.sysml", model_hash="hash-abc")


class TestRenamedNames:
    """The two names that shadowed a built-in, and the aliases left behind."""

    def test_the_old_names_resolve_to_the_new_objects(self):
        """`opensysml.eval` is `evaluate` and `opensysml.RuntimeError` is `ExecutionError`."""
        for old, new in (("eval", opensysml.evaluate), ("RuntimeError", opensysml.ExecutionError)):
            with warnings.catch_warnings(record=True) as caught:
                warnings.simplefilter("always")
                alias = getattr(opensysml, old)
            assert alias is new
            assert len(caught) == 1
            assert issubclass(caught[0].category, DeprecationWarning)
            assert new.__name__ in str(caught[0].message)

    @pytest.mark.usefixtures("no_default_connection")
    def test_an_old_snippet_still_runs_through_the_alias(self):
        """A script written against `opensysml.eval` keeps working, warning included."""
        with patch('opensysml.Connection') as MockConnection:
            mock_conn = Mock()
            MockConnection.return_value = mock_conn
            mock_conn.eval.return_value = 4

            with warnings.catch_warnings():
                warnings.simplefilter("ignore", DeprecationWarning)
                assert opensysml.eval("2 + 2", model_hash="h") == 4

            mock_conn.eval.assert_called_once_with(
                "2 + 2", "h", context_symbol_id=None, subject_symbol_id=None
            )

    def test_the_old_names_no_longer_shadow_a_builtin_on_star_import(self):
        """Which is the point of the rename: `import *` binds neither name."""
        assert "eval" not in opensysml.__all__
        assert "evaluate" in opensysml.__all__
        assert "RuntimeError" not in opensysml.__all__

        namespace = {}
        exec("from opensysml import *", namespace)
        assert "eval" not in namespace
        assert "RuntimeError" not in namespace
        assert namespace["evaluate"] is opensysml.evaluate

    def test_a_name_the_package_never_had_is_still_an_attribute_error(self):
        with pytest.raises(AttributeError, match="no attribute 'nope'"):
            opensysml.nope
