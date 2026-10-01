//! What the service resolves about a symbol's type, multiplicity and specializations.

use std::collections::{BTreeMap, HashMap, HashSet};

use crate::capabilities::{upgrade_remedy, CAPABILITY_SYMBOL_ATTRIBUTES};
use crate::domain::{value_from_wire, Symbol, Value};
use crate::error::Error;
use crate::wire;

/// A feature's declared and resolved type.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct TypeFacts {
    /// Type name as written; empty when none is declared.
    pub declared: String,
    /// Qualified name of the resolved type; empty when unresolved or undeclared.
    pub resolved_id: String,
    /// Symbol kind of the resolved type, such as `partDef`.
    pub resolved_kind: String,
    /// Library scalar the type reduces to, such as `Real`; empty when it is not one.
    pub primitive: String,
    /// Where [`TypeFacts::primitive`] came from: `declared`, `value` or empty.
    pub primitive_source: String,
    /// Whether values carry a measurement unit.
    pub quantity: bool,
    /// Unit as written, when the default value names one.
    pub unit: String,
}

impl From<wire::TypeInfo> for TypeFacts {
    fn from(info: wire::TypeInfo) -> Self {
        Self {
            declared: info.declared,
            resolved_id: info.resolved_id,
            resolved_kind: info.resolved_kind,
            primitive: info.primitive,
            primitive_source: info.primitive_source,
            quantity: info.quantity,
            unit: info.unit,
        }
    }
}

/// A feature's multiplicity bounds as written; `upper` is `*` when unbounded.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Multiplicity {
    /// Lower bound; empty when unstated.
    pub lower: String,
    /// Upper bound; empty when unstated.
    pub upper: String,
}

impl Multiplicity {
    /// Whether the feature holds more than one value; `None` when the bound is not a number.
    pub fn is_collection(&self) -> Option<bool> {
        match self.upper.as_str() {
            "" => None,
            "*" => Some(true),
            upper => upper.parse::<i64>().ok().map(|n| n > 1),
        }
    }
    /// Whether the feature may hold no value; `None` when the bound is not a number.
    pub fn is_optional(&self) -> Option<bool> {
        self.lower.parse::<i64>().ok().map(|n| n == 0)
    }
}

impl From<wire::MultiplicityInfo> for Multiplicity {
    fn from(info: wire::MultiplicityInfo) -> Self {
        Self {
            lower: info.lower,
            upper: info.upper,
        }
    }
}

/// One specialization a symbol declares: subclassification, subsetting, redefinition and so on.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Specialization {
    /// The relationship, such as `subclassification`.
    pub kind: String,
    /// The target as written.
    pub declared: String,
    /// Qualified name of the resolved target; empty when unresolved.
    pub target_id: String,
    /// Symbol kind of the resolved target.
    pub target_kind: String,
}

impl From<wire::Specialization> for Specialization {
    fn from(info: wire::Specialization) -> Self {
        Self {
            kind: info.kind,
            declared: info.declared,
            target_id: info.target_id,
            target_kind: info.target_kind,
        }
    }
}

/// One attribute the service resolved for a symbol, inherited ones included.
#[derive(Clone, Debug, PartialEq)]
pub struct AttributeFacts {
    /// Short name.
    pub name: String,
    /// Qualified name of the resolved type, else the type as written, else the library scalar.
    pub attribute_type: String,
    /// Default value, when it is a model-level constant.
    pub value: Option<Value>,
    /// Unit the default value is written in; empty when it carries none.
    pub unit: String,
}

impl AttributeFacts {
    pub(crate) fn from_wire(info: wire::AttributeInfo) -> Result<Self, Error> {
        Ok(Self {
            name: info.name,
            attribute_type: info.r#type,
            value: info.value.map(value_from_wire).transpose()?,
            unit: info.unit,
        })
    }
}

/// Every static fact the service reported about one symbol.
#[derive(Clone, Debug, PartialEq)]
pub struct SymbolFacts {
    /// Qualified name.
    pub id: String,
    /// Short name.
    pub name: String,
    /// Symbol kind.
    pub kind: String,
    /// Its type, when it is a feature with one.
    pub type_facts: Option<TypeFacts>,
    /// Its multiplicity, when it states one.
    pub multiplicity: Option<Multiplicity>,
    /// Its specializations, in declaration order.
    pub specializations: Vec<Specialization>,
    /// Its attributes, inherited ones included.
    pub attributes: Vec<AttributeFacts>,
    /// How many standard-library attributes were left out of [`SymbolFacts::attributes`].
    pub withheld_library_attributes: i32,
}

const ATTRIBUTE_KINDS: &[&str] = &["attributedef", "attributeusage", "referenceusage"];
const PART_KINDS: &[&str] = &["partdef", "partusage"];

fn kind_in(symbol: &Symbol, kinds: &[&str]) -> bool {
    kinds.contains(&symbol.kind().to_ascii_lowercase().as_str())
}

impl Symbol {
    /// Metadata the service attached, by key.
    pub fn metadata(&self) -> BTreeMap<String, String> {
        self.wire()
            .metadata
            .iter()
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect()
    }
    /// Its declared and resolved type, when it is a feature with one.
    pub fn type_facts(&self) -> Option<TypeFacts> {
        self.wire().type_info.clone().map(TypeFacts::from)
    }
    /// Its multiplicity, when it states one.
    pub fn multiplicity(&self) -> Option<Multiplicity> {
        self.wire().multiplicity.clone().map(Multiplicity::from)
    }
    /// Its specializations, in declaration order.
    pub fn specializations(&self) -> Vec<Specialization> {
        self.wire()
            .specializations
            .iter()
            .cloned()
            .map(Specialization::from)
            .collect()
    }
    /// How many standard-library attributes the service left out of its attributes.
    pub fn withheld_library_attributes(&self) -> i32 {
        self.wire().withheld_library_attributes
    }
    /// The attributes the service resolved, inherited ones included; needs `symbol_attributes`.
    pub fn attribute_facts(&self) -> Result<Vec<AttributeFacts>, Error> {
        self.connection().capabilities().require(
            CAPABILITY_SYMBOL_ATTRIBUTES,
            upgrade_remedy(CAPABILITY_SYMBOL_ATTRIBUTES),
        )?;
        self.decoded_attribute_facts()
    }
    fn decoded_attribute_facts(&self) -> Result<Vec<AttributeFacts>, Error> {
        self.wire()
            .attributes
            .iter()
            .cloned()
            .map(AttributeFacts::from_wire)
            .collect()
    }
    /// Every static fact the service reported about the symbol.
    pub fn facts(&self) -> Result<SymbolFacts, Error> {
        Ok(SymbolFacts {
            id: self.id().to_owned(),
            name: self.name().to_owned(),
            kind: self.kind().to_owned(),
            type_facts: self.type_facts(),
            multiplicity: self.multiplicity(),
            specializations: self.specializations(),
            attributes: self.decoded_attribute_facts()?,
            withheld_library_attributes: self.withheld_library_attributes(),
        })
    }
    /// Attribute and reference members, declared and inherited, in the order the service
    /// resolved them; a member redeclared here hides the inherited one.
    pub fn attributes(&self) -> Result<Vec<Symbol>, Error> {
        let declared: Vec<Symbol> = self
            .children()?
            .into_iter()
            .filter(|c| kind_in(c, ATTRIBUTE_KINDS))
            .collect();
        let mut by_name: HashMap<String, Symbol> = declared
            .iter()
            .map(|c| (c.name().to_owned(), c.clone()))
            .collect();
        for inherited in self.inherited_attributes(&mut HashSet::new())? {
            by_name
                .entry(inherited.name().to_owned())
                .or_insert(inherited);
        }
        let reported: Vec<&str> = self
            .wire()
            .attributes
            .iter()
            .map(|a| a.name.as_str())
            .collect();
        let mut ordered: Vec<Symbol> = reported
            .iter()
            .filter_map(|name| by_name.get(*name).cloned())
            .collect();
        ordered.extend(
            declared
                .into_iter()
                .filter(|c| !reported.contains(&c.name())),
        );
        Ok(ordered)
    }
    fn inherited_attributes(&self, visited: &mut HashSet<String>) -> Result<Vec<Symbol>, Error> {
        if !visited.insert(self.id().to_owned()) {
            return Ok(Vec::new());
        }
        let mut found = Vec::new();
        for specialization in self.specializations() {
            if specialization.target_id.is_empty() || visited.contains(&specialization.target_id) {
                continue;
            }
            let supertype = match self
                .connection()
                .get_symbol(self.model_hash(), &specialization.target_id)
            {
                Ok(symbol) => symbol,
                Err(Error::Model(_)) => continue,
                Err(error) => return Err(error),
            };
            found.extend(
                supertype
                    .children()?
                    .into_iter()
                    .filter(|c| kind_in(c, ATTRIBUTE_KINDS)),
            );
            found.extend(supertype.inherited_attributes(visited)?);
        }
        Ok(found)
    }
    /// Part members declared directly in the symbol.
    pub fn parts(&self) -> Result<Vec<Symbol>, Error> {
        Ok(self
            .children()?
            .into_iter()
            .filter(|c| kind_in(c, PART_KINDS))
            .collect())
    }
    /// The attribute named `name`, declared or inherited.
    pub fn attribute(&self, name: &str) -> Result<Option<Symbol>, Error> {
        Ok(self.attributes()?.into_iter().find(|a| a.name() == name))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn multiplicity_reads_its_bounds() {
        let m = |lower: &str, upper: &str| Multiplicity {
            lower: lower.to_owned(),
            upper: upper.to_owned(),
        };
        assert_eq!(m("0", "*").is_collection(), Some(true));
        assert_eq!(m("0", "*").is_optional(), Some(true));
        assert_eq!(m("1", "1").is_collection(), Some(false));
        assert_eq!(m("1", "1").is_optional(), Some(false));
        assert_eq!(m("", "").is_collection(), None);
        assert_eq!(m("n", "n").is_optional(), None);
        assert_eq!(m("n", "n").is_collection(), None);
    }
}
