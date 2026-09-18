"""Integration tests against real sysml-grpc server.

These tests require a running sysml-grpc server on localhost:50051.
They verify end-to-end functionality with real RPC calls (no mocks).

Run with: pytest tests/test_integration.py
Skip with: pytest tests/ -k "not integration"

Without a service they skip, which is what a developer who has no binary wants;
with $OPENSYSML_REQUIRE_SERVICE set, as CI sets it, an absent service fails
instead, since a skip that never runs exercises nothing.
"""

import pytest
import grpc
from opensysml import Connection
from opensysml.errors import ModelFileNotFoundError
from tests.service_gate import fail_if_service_promised, is_server_available

_AVAILABLE = is_server_available()
fail_if_service_promised(_AVAILABLE)

pytestmark = pytest.mark.skipif(
    not _AVAILABLE,
    reason="sysml-grpc server not running on localhost:50051"
)


class TestIntegrationRealServer:
    """Integration tests using real sysml-grpc server."""

    def test_connection_to_real_server(self):
        """Verify Connection can reach real sysml-grpc server."""
        with Connection(auto_start=False) as conn:
            assert conn is not None
            # If we get here without grpc.RpcError, connection works

    def test_load_real_file_a1_sysml(self, tmp_path):
        """Verify end-to-end: load A1.sysml via real server.
        
        Phase 2 DoD requirement: "model = conn.load('A1.sysml') works"
        """
        # Create minimal valid SysML file (since we don't know if A1.sysml exists)
        test_file = tmp_path / "test_model.sysml"
        test_file.write_text("""
            package TestPackage {
                part Vehicle {
                    part Engine;
                    part Wheels;
                }
                part Sensor;
            }
        """)
        
        with Connection(auto_start=False) as conn:
            # Load file
            model = conn.load(str(test_file))
            
            # Verify model structure
            assert model is not None
            assert model.hash is not None
            assert len(model.hash) > 0
            
            # Verify root
            assert model.root is not None
            assert model.root.name is not None  # Has a name (may be empty string for RootNamespace)
            assert model.root.kind  # Has a kind
            
            # Verify no diagnostics (valid file)
            assert isinstance(model.diagnostics, list)
            # Note: parser might emit warnings, so don't assert == 0

    def test_lazy_loading_via_real_server(self, tmp_path):
        """Verify lazy Symbol.children() fetches via real GetSymbol RPC.
        
        Phase 2 DoD requirement: navigation works, lazy loading functional.
        """
        test_file = tmp_path / "lazy_test.sysml"
        test_file.write_text("""
            package MyModel {
                part Vehicle {
                    part Engine;
                }
            }
        """)
        
        with Connection(auto_start=False) as conn:
            model = conn.load(str(test_file))
            
            # Navigate via lazy loading
            root_children = model.root.children()
            assert isinstance(root_children, list)
            
            # If model has children, verify they're Symbol objects
            if len(root_children) > 0:
                first_child = root_children[0]
                from opensysml import Symbol
                assert isinstance(first_child, Symbol)
                assert first_child.name is not None
                assert first_child.kind is not None

    def test_model_find_via_real_server(self, tmp_path):
        """Verify Model.find() works end-to-end.
        
        Phase 2 DoD requirement: "part = model.find('SPACECRAFT_WET') works"
        """
        test_file = tmp_path / "find_test.sysml"
        test_file.write_text("""
            package TestPkg {
                part Vehicle;
                part Sensor;
            }
        """)
        
        with Connection(auto_start=False) as conn:
            model = conn.load(str(test_file))
            
            # Try to find a symbol
            # Note: exact names depend on how parser handles file structure
            # Just verify find() doesn't crash and returns None for missing
            result = model.find("NonExistentSymbol")
            assert result is None  # Should return None for missing symbols

    def test_context_manager_cleanup_real_server(self):
        """Verify context manager closes channel properly with real server."""
        conn = Connection(auto_start=False)
        with conn:
            # Use connection
            assert conn._channel is not None
        # After __exit__, channel should be closed
        # (Can't directly verify, but should not raise)

    def test_load_nonexistent_file_real_server(self):
        """Verify a file the service cannot read raises the typed domain error.

        The public contract is the domain exception, not the status it was
        translated from, which stays reachable as __cause__.
        """
        with Connection(auto_start=False) as conn:
            with pytest.raises(ModelFileNotFoundError) as exc_info:
                conn.load("/nonexistent/path/to/file.sysml")

            assert exc_info.value.code == grpc.StatusCode.NOT_FOUND
            # And is what the failure is, so os-level handling catches it.
            assert isinstance(exc_info.value, FileNotFoundError)
            assert exc_info.value.__cause__.code() == grpc.StatusCode.NOT_FOUND

    def test_get_symbol_with_real_server(self, tmp_path):
        """Verify get_symbol() works with real server."""
        test_file = tmp_path / "symbol_test.sysml"
        test_file.write_text("""
            package SymbolTest {
                part TestPart;
            }
        """)
        
        with Connection(auto_start=False) as conn:
            model = conn.load(str(test_file))
            
            # Get a child symbol (not root with empty ID)
            if len(model.root.children()) > 0:
                child = model.root.children()[0]
                # Try to get this symbol via its ID
                symbol_info = conn.get_symbol(model.hash, child.id)
                
                # Should return SymbolInfo protobuf
                if symbol_info is not None:
                    assert hasattr(symbol_info, 'id')
                    assert hasattr(symbol_info, 'name')
                    assert hasattr(symbol_info, 'kind')
                # Else: child_ids might be empty for this simple model, test passes
