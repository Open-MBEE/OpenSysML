"""A variation point and its variants are told apart through symbol metadata.

The service reports ``variation`` (and ``abstract``, since a variation is
always abstract) on a variation definition or usage, ``variant`` on a variant,
and an enumeration definition and its values likewise; an ordinary part carries
none of the three. The keys travel the metadata map unchanged.
"""

import pytest

from opensysml import Connection

MODEL_SOURCE = """
package PLE {
    part def Engine;
    part def Transmission;
    part fourCyl : Engine;
    part manual : Transmission;

    variation part def EngineChoices :> Engine {
        variant fourCyl;
    }

    part family {
        part engine : EngineChoices;
        variation part transmission : Transmission {
            variant manual;
        }
    }

    enum def Color { red; blue; }
}
"""


@pytest.mark.integration
class TestVariationMetadata:
    def setup_method(self):
        self.conn = Connection()
        self.model = self.conn.load_from_content(MODEL_SOURCE)

    def teardown_method(self):
        self.conn.close()

    def metadata(self, name):
        symbol = self.model.find(name)
        assert symbol is not None, name
        return symbol.metadata

    def test_a_variation_definition_is_a_variation_and_abstract(self):
        meta = self.metadata("PLE::EngineChoices")
        assert meta["variation"] == "true"
        assert meta["abstract"] == "true"
        assert "variant" not in meta

    def test_a_variation_usage_is_a_variation_and_abstract(self):
        meta = self.metadata("PLE::family::transmission")
        assert meta["variation"] == "true"
        assert meta["abstract"] == "true"
        assert "variant" not in meta

    def test_a_variant_is_a_variant(self):
        for name in ("PLE::EngineChoices::fourCyl", "PLE::family::transmission::manual"):
            meta = self.metadata(name)
            assert meta["variant"] == "true", name
            assert "variation" not in meta, name
            assert "abstract" not in meta, name

    def test_an_enumeration_is_a_variation_of_its_values(self):
        assert self.metadata("PLE::Color")["variation"] == "true"
        assert self.metadata("PLE::Color")["abstract"] == "true"
        assert self.metadata("PLE::Color::red")["variant"] == "true"

    def test_an_ordinary_part_is_neither(self):
        meta = self.metadata("PLE::fourCyl")
        assert "variation" not in meta
        assert "variant" not in meta
        assert "abstract" not in meta
