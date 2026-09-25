"""Diagnostic class wrapping protobuf diagnostic message."""


class Diagnostic:
    """Represents a parse diagnostic (error or warning).
    
    Attributes:
        severity (str): Diagnostic severity (e.g., "error", "warning")
        message (str): Diagnostic message
        code (str): Stable identifier of what was found (a validation code,
            "syntax", "choice-point", "guard-unevaluable"); "" when none
        file (str): Source file name
        start_line (int): Starting line number (1-based)
        start_column (int): Starting column number (1-based)
        end_line (int): Ending line number (1-based)
        end_column (int): Ending column number (1-based)
        span: Protobuf Span object
    """
    
    def __init__(self, pb_diagnostic):
        """Initialize Diagnostic from protobuf Diagnostic message.
        
        Args:
            pb_diagnostic: sysml_pb2.Diagnostic protobuf message
        """
        self._pb = pb_diagnostic
    
    @property
    def severity(self):
        """Get diagnostic severity."""
        return self._pb.severity
    
    @property
    def message(self):
        """Get diagnostic message."""
        return self._pb.message
    
    @property
    def code(self):
        """Get the diagnostic code, independent of the message wording."""
        return self._pb.code
    
    @property
    def span(self):
        """Get protobuf Span object."""
        return self._pb.span
    
    @property
    def file(self):
        """Get source file name."""
        return self._pb.span.file
    
    @property
    def start_line(self):
        """Get starting line number (1-based)."""
        return self._pb.span.start_line
    
    @property
    def start_column(self):
        """Get starting column number (1-based)."""
        return self._pb.span.start_col
    
    @property
    def end_line(self):
        """Get ending line number (1-based)."""
        return self._pb.span.end_line
    
    @property
    def end_column(self):
        """Get ending column number (1-based)."""
        return self._pb.span.end_col
    
    def __str__(self):
        """String representation: 'file:line:col: severity: message'."""
        return (
            f"{self.file}:{self.start_line}:{self.start_column}: "
            f"{self.severity}: {self.message}"
        )
    
    def __repr__(self):
        """Detailed representation."""
        return (
            f"Diagnostic(severity={self.severity!r}, "
            f"message={self.message!r}, "
            f"code={self.code!r}, "
            f"file={self.file!r}, "
            f"line={self.start_line})"
        )
