"""Pytest fixtures for opensysml tests."""

import pytest
from unittest.mock import Mock, MagicMock
from opensysml.proto import sysml_pb2


@pytest.fixture
def mock_pb_diagnostic():
    """Create mock protobuf Diagnostic."""
    diag = sysml_pb2.Diagnostic()
    diag.severity = "ERROR"
    diag.message = "Test error"
    diag.span.source = "test.sysml"
    diag.span.start_line = 10
    diag.span.start_col = 5
    diag.span.end_line = 10
    diag.span.end_col = 15
    return diag


@pytest.fixture
def mock_pb_symbol():
    """Create mock protobuf SymbolInfo."""
    symbol = sysml_pb2.SymbolInfo()
    symbol.id = "Vehicle::engine"
    symbol.name = "engine"
    symbol.kind = "PartUsage"
    symbol.metadata["doc"] = "Engine subsystem"
    return symbol


@pytest.fixture
def mock_pb_parse_response():
    """Create mock protobuf ParseFileResponse."""
    response = sysml_pb2.ParseFileResponse()
    response.model_hash = "abc123"
    
    # Root symbol
    root = response.root
    root.id = "TestModel"
    root.name = "TestModel"
    root.kind = "package"
    
    # Add diagnostic
    diag = response.diagnostics.add()
    diag.severity = "WARNING"
    diag.message = "Test warning"
    diag.span.source = "test.sysml"
    diag.span.start_line = 1
    diag.span.start_col = 1
    diag.span.end_line = 1
    diag.span.end_col = 10
    
    return response


@pytest.fixture
def mock_client():
    """Create mock gRPC client stub."""
    client = MagicMock()
    
    # Mock GetSymbol response
    child_response = sysml_pb2.SymbolResponse()
    child = child_response.symbols.add()
    child.id = "TestModel::Child1"
    child.name = "Child1"
    child.kind = "PartDef"
    
    client.GetSymbol.return_value = child_response
    
    return client


@pytest.fixture
def mock_diagnostic(mock_pb_diagnostic):
    """Create Diagnostic instance from protobuf."""
    from opensysml.diagnostic import Diagnostic
    return Diagnostic(mock_pb_diagnostic)


@pytest.fixture
def mock_symbol(mock_pb_symbol, mock_client):
    """Create Symbol instance from protobuf."""
    from opensysml.symbol import Symbol
    return Symbol(mock_pb_symbol, mock_client, "test_hash")


@pytest.fixture
def mock_model(mock_pb_parse_response, mock_client):
    """Create Model instance from protobuf."""
    from opensysml.model import Model
    return Model(mock_pb_parse_response, mock_client)
