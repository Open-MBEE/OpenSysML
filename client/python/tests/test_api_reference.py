import re
from collections import Counter
from pathlib import Path

import opensysml
import pytest


API_REFERENCE = (
    Path(__file__).resolve().parents[2].parent
    / "docs"
    / "reference"
    / "python-api.md"
)
DIRECTIVE = re.compile(r"^::: opensysml\.(\w+)\s*$", re.MULTILINE)


def test_api_reference_matches_public_exports():
    if not API_REFERENCE.exists():
        pytest.skip("documentation is not included in this sdist")

    names = DIRECTIVE.findall(API_REFERENCE.read_text(encoding="utf-8"))
    counts = Counter(names)
    duplicates = sorted(name for name, count in counts.items() if count > 1)
    public = set(opensysml.__all__)
    documented = set(names)
    unexpected = sorted(documented - public)
    missing = sorted(public - documented)

    assert not duplicates, f"duplicate API directives: {', '.join(duplicates)}"
    assert not unexpected, f"names outside opensysml.__all__: {', '.join(unexpected)}"
    assert not missing, f"missing public exports: {', '.join(missing)}"
