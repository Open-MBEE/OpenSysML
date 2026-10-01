//! SysML v2 API & Services queries: `scope`, `select` and `where` over a loaded model.

use std::collections::BTreeMap;
use std::fmt;

use serde_json::{Map, Value as Json};

use crate::error::Error;
use crate::wire;

/// The standard's `@type` of a query.
pub const TYPE_QUERY: &str = "Query";
/// The standard's `@type` of a constraint comparing one property.
pub const TYPE_PRIMITIVE_CONSTRAINT: &str = "PrimitiveConstraint";
/// The standard's `@type` of a constraint combining others.
pub const TYPE_COMPOSITE_CONSTRAINT: &str = "CompositeConstraint";

const TYPE_KEY: &str = "@type";

/// How a primitive constraint compares a property with its value.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum PrimitiveOperator {
    /// `=`: the property equals one of the values.
    Equal,
    /// `>`: the property is greater than the value.
    Greater,
    /// `<`: the property is less than the value.
    Less,
}

impl PrimitiveOperator {
    fn parse(text: &str) -> Option<Self> {
        match text {
            "=" => Some(Self::Equal),
            ">" => Some(Self::Greater),
            "<" => Some(Self::Less),
            _ => None,
        }
    }
    fn from_wire(code: i32) -> Result<Self, Error> {
        match wire::PrimitiveOperator::try_from(code) {
            Ok(wire::PrimitiveOperator::Equal) => Ok(Self::Equal),
            Ok(wire::PrimitiveOperator::Greater) => Ok(Self::Greater),
            Ok(wire::PrimitiveOperator::Less) => Ok(Self::Less),
            _ => Err(Error::Query(format!(
                "a primitive constraint compares by =, > or <, not operator {code}"
            ))),
        }
    }
    fn to_wire(self) -> wire::PrimitiveOperator {
        match self {
            Self::Equal => wire::PrimitiveOperator::Equal,
            Self::Greater => wire::PrimitiveOperator::Greater,
            Self::Less => wire::PrimitiveOperator::Less,
        }
    }
}

/// How a composite constraint combines its constraints.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum CompositeOperator {
    /// Every constraint holds.
    And,
    /// At least one constraint holds.
    Or,
}

impl CompositeOperator {
    fn parse(text: &str) -> Option<Self> {
        match text {
            "and" => Some(Self::And),
            "or" => Some(Self::Or),
            _ => None,
        }
    }
    fn from_wire(code: i32) -> Result<Self, Error> {
        match wire::CompositeOperator::try_from(code) {
            Ok(wire::CompositeOperator::And) => Ok(Self::And),
            Ok(wire::CompositeOperator::Or) => Ok(Self::Or),
            _ => Err(Error::Query(format!(
                "a composite constraint combines by and or or, not operator {code}"
            ))),
        }
    }
    fn to_wire(self) -> wire::CompositeOperator {
        match self {
            Self::And => wire::CompositeOperator::And,
            Self::Or => wire::CompositeOperator::Or,
        }
    }
}

/// A query's `where`: one property compared, or several constraints combined.
#[derive(Clone, Debug, PartialEq)]
pub enum Constraint {
    /// Compare one property with values, written as the strings the standard compares.
    Primitive {
        /// Hold where the comparison does not.
        inverse: bool,
        /// The property compared.
        property: String,
        /// The comparison.
        operator: PrimitiveOperator,
        /// The values compared against.
        values: Vec<String>,
    },
    /// Combine constraints.
    Composite {
        /// How they combine.
        operator: CompositeOperator,
        /// The constraints combined; never empty.
        constraints: Vec<Constraint>,
    },
}

impl Constraint {
    /// `property = value`.
    pub fn equals(property: impl Into<String>, value: impl Into<String>) -> Self {
        Self::compare(property, PrimitiveOperator::Equal, vec![value.into()])
    }
    /// `property` equals any of `values`.
    pub fn one_of<I, S>(property: impl Into<String>, values: I) -> Self
    where
        I: IntoIterator<Item = S>,
        S: Into<String>,
    {
        Self::compare(
            property,
            PrimitiveOperator::Equal,
            values.into_iter().map(Into::into).collect(),
        )
    }
    /// `property > value`.
    pub fn greater(property: impl Into<String>, value: impl Into<String>) -> Self {
        Self::compare(property, PrimitiveOperator::Greater, vec![value.into()])
    }
    /// `property < value`.
    pub fn less(property: impl Into<String>, value: impl Into<String>) -> Self {
        Self::compare(property, PrimitiveOperator::Less, vec![value.into()])
    }
    fn compare(
        property: impl Into<String>,
        operator: PrimitiveOperator,
        values: Vec<String>,
    ) -> Self {
        Self::Primitive {
            inverse: false,
            property: property.into(),
            operator,
            values,
        }
    }
    /// Every one of `constraints`.
    pub fn and(constraints: impl IntoIterator<Item = Constraint>) -> Self {
        Self::Composite {
            operator: CompositeOperator::And,
            constraints: constraints.into_iter().collect(),
        }
    }
    /// Any of `constraints`.
    pub fn or(constraints: impl IntoIterator<Item = Constraint>) -> Self {
        Self::Composite {
            operator: CompositeOperator::Or,
            constraints: constraints.into_iter().collect(),
        }
    }
    /// The same primitive comparison, holding where it does not; a composite is unchanged.
    pub fn inverted(self) -> Self {
        match self {
            Self::Primitive {
                inverse,
                property,
                operator,
                values,
            } => Self::Primitive {
                inverse: !inverse,
                property,
                operator,
                values,
            },
            composite => composite,
        }
    }

    fn to_wire(&self) -> Result<wire::Constraint, Error> {
        use wire::constraint::Constraint as Kind;
        Ok(wire::Constraint {
            constraint: Some(match self {
                Self::Primitive {
                    inverse,
                    property,
                    operator,
                    values,
                } => {
                    if property.is_empty() {
                        return Err(Error::Query(
                            "a primitive constraint names one property, not an empty one"
                                .to_owned(),
                        ));
                    }
                    Kind::Primitive(wire::PrimitiveConstraint {
                        inverse: *inverse,
                        property: property.clone(),
                        operator: operator.to_wire() as i32,
                        value: values.clone(),
                    })
                }
                Self::Composite {
                    operator,
                    constraints,
                } => {
                    if constraints.is_empty() {
                        return Err(Error::Query(
                            "a composite constraint combines a non-empty list of constraints"
                                .to_owned(),
                        ));
                    }
                    Kind::Composite(wire::CompositeConstraint {
                        operator: operator.to_wire() as i32,
                        constraint: constraints
                            .iter()
                            .map(Constraint::to_wire)
                            .collect::<Result<_, _>>()?,
                    })
                }
            }),
        })
    }

    fn from_json(payload: &Json) -> Result<Self, Error> {
        let Json::Object(object) = payload else {
            return Err(Error::Query(format!(
                "a constraint is an object, not {}",
                json_kind(payload)
            )));
        };
        let declared = match object.get(TYPE_KEY) {
            Some(Json::String(declared)) => declared.as_str(),
            Some(other) => {
                return Err(Error::Query(format!(
                    "unknown constraint type {other}; the standard's constraints are \
                     {TYPE_PRIMITIVE_CONSTRAINT} and {TYPE_COMPOSITE_CONSTRAINT}"
                )))
            }
            None if object.contains_key("constraint") => TYPE_COMPOSITE_CONSTRAINT,
            None => TYPE_PRIMITIVE_CONSTRAINT,
        };
        match declared {
            TYPE_PRIMITIVE_CONSTRAINT => Self::primitive_from_json(object),
            TYPE_COMPOSITE_CONSTRAINT => Self::composite_from_json(object),
            other => Err(Error::Query(format!(
                "unknown constraint type {other:?}; the standard's constraints are \
                 {TYPE_PRIMITIVE_CONSTRAINT} and {TYPE_COMPOSITE_CONSTRAINT}"
            ))),
        }
    }

    fn primitive_from_json(object: &Map<String, Json>) -> Result<Self, Error> {
        reject_unknown(
            object,
            &[TYPE_KEY, "@id", "inverse", "property", "operator", "value"],
            TYPE_PRIMITIVE_CONSTRAINT,
        )?;
        let operator = object
            .get("operator")
            .and_then(Json::as_str)
            .and_then(PrimitiveOperator::parse)
            .ok_or_else(|| {
                Error::Query(format!(
                    "unknown primitive operator {}; expected one of <, =, >",
                    shown(object.get("operator"))
                ))
            })?;
        let property = match object.get("property") {
            Some(Json::String(name)) if !name.is_empty() => name.clone(),
            other => {
                return Err(Error::Query(format!(
                    "a primitive constraint names one property, not {}",
                    shown(other)
                )))
            }
        };
        let inverse = match object.get("inverse") {
            None | Some(Json::Null) => false,
            Some(Json::Bool(inverse)) => *inverse,
            Some(other) => {
                return Err(Error::Query(format!(
                    "a constraint's inverse is true or false, not {other}"
                )))
            }
        };
        let values = match object.get("value") {
            None | Some(Json::Null) => Vec::new(),
            Some(Json::Array(values)) => values.iter().map(compared).collect::<Result<_, _>>()?,
            Some(value) => vec![compared(value)?],
        };
        Ok(Self::Primitive {
            inverse,
            property,
            operator,
            values,
        })
    }

    fn composite_from_json(object: &Map<String, Json>) -> Result<Self, Error> {
        reject_unknown(
            object,
            &[TYPE_KEY, "@id", "constraint", "operator"],
            TYPE_COMPOSITE_CONSTRAINT,
        )?;
        let operator = object
            .get("operator")
            .and_then(Json::as_str)
            .and_then(CompositeOperator::parse)
            .ok_or_else(|| {
                Error::Query(format!(
                    "unknown composite operator {}; expected one of and, or",
                    shown(object.get("operator"))
                ))
            })?;
        let constraints = match object.get("constraint") {
            Some(Json::Array(nested)) if !nested.is_empty() => nested
                .iter()
                .map(Constraint::from_json)
                .collect::<Result<_, _>>()?,
            other => {
                return Err(Error::Query(format!(
                    "a composite constraint combines a non-empty list of constraints, not {}",
                    shown(other)
                )))
            }
        };
        Ok(Self::Composite {
            operator,
            constraints,
        })
    }
}

/// A query over a loaded model: which elements, which properties, and which constraint they meet.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct Query {
    /// Qualified names of the elements to consider; empty is the whole model.
    pub scope: Vec<String>,
    /// Properties to report; empty reports every one.
    pub select: Vec<String>,
    /// Constraint the selected elements meet.
    pub filter: Option<Constraint>,
}

impl Query {
    /// The query over the whole model reporting every property.
    pub fn new() -> Self {
        Self::default()
    }
    /// The same query considering `scope` too.
    pub fn scope<I, S>(mut self, scope: I) -> Self
    where
        I: IntoIterator<Item = S>,
        S: Into<String>,
    {
        self.scope.extend(scope.into_iter().map(Into::into));
        self
    }
    /// The same query reporting `properties` too.
    pub fn select<I, S>(mut self, properties: I) -> Self
    where
        I: IntoIterator<Item = S>,
        S: Into<String>,
    {
        self.select.extend(properties.into_iter().map(Into::into));
        self
    }
    /// The same query with `constraint` as its `where`.
    pub fn filter(mut self, constraint: Constraint) -> Self {
        self.filter = Some(constraint);
        self
    }

    /// Read the standard's JSON `Query` object, so a cookbook payload works verbatim.
    pub fn from_json(payload: &Json) -> Result<Self, Error> {
        let Json::Object(object) = payload else {
            return Err(Error::Query(format!(
                "a query is an object, not {}",
                json_kind(payload)
            )));
        };
        match object.get(TYPE_KEY) {
            None => {}
            Some(Json::String(declared)) if declared == TYPE_QUERY => {}
            Some(other) => {
                return Err(Error::Query(format!(
                    "expected a {TYPE_QUERY:?} payload, got {other}"
                )))
            }
        }
        let mut unknown: Vec<&str> = object
            .keys()
            .map(String::as_str)
            .filter(|key| {
                !matches!(
                    *key,
                    TYPE_KEY | "@id" | "owningProject" | "scope" | "select" | "where"
                )
            })
            .collect();
        if !unknown.is_empty() {
            unknown.sort_unstable();
            return Err(Error::Query(format!(
                "a query has no {}; the standard's query is scope, select and where",
                unknown.join(", ")
            )));
        }
        let scope = entries("scope", object.get("scope"))?
            .into_iter()
            .map(scope_id)
            .collect::<Result<_, _>>()?;
        let select = entries("select", object.get("select"))?
            .into_iter()
            .map(|entry| match entry {
                Json::String(name) => Ok(name.clone()),
                other => Err(Error::Query(format!(
                    "a selected property is a name, not {other}"
                ))),
            })
            .collect::<Result<_, _>>()?;
        let filter = match object.get("where") {
            None | Some(Json::Null) => None,
            Some(constraint) => Some(Constraint::from_json(constraint)?),
        };
        Ok(Self {
            scope,
            select,
            filter,
        })
    }

    pub(crate) fn to_wire(&self) -> Result<wire::Query, Error> {
        Ok(wire::Query {
            scope: self.scope.clone(),
            select: self.select.clone(),
            r#where: self.filter.as_ref().map(Constraint::to_wire).transpose()?,
        })
    }
}

impl TryFrom<wire::Query> for Query {
    type Error = Error;

    fn try_from(query: wire::Query) -> Result<Self, Error> {
        Ok(Self {
            scope: query.scope,
            select: query.select,
            filter: query.r#where.map(Constraint::try_from).transpose()?,
        })
    }
}

impl TryFrom<wire::Constraint> for Constraint {
    type Error = Error;

    fn try_from(constraint: wire::Constraint) -> Result<Self, Error> {
        use wire::constraint::Constraint as Kind;
        match constraint.constraint {
            Some(Kind::Primitive(primitive)) => Ok(Self::Primitive {
                inverse: primitive.inverse,
                property: primitive.property,
                operator: PrimitiveOperator::from_wire(primitive.operator)?,
                values: primitive.value,
            }),
            Some(Kind::Composite(composite)) => Ok(Self::Composite {
                operator: CompositeOperator::from_wire(composite.operator)?,
                constraints: composite
                    .constraint
                    .into_iter()
                    .map(Self::try_from)
                    .collect::<Result<_, _>>()?,
            }),
            None => Err(Error::Query(
                "a constraint is primitive or composite; this one is neither".to_owned(),
            )),
        }
    }
}

/// One element a query selected, with the properties it reported as their string forms.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct QueryElement {
    /// Qualified name.
    pub id: String,
    /// The standard's element type, such as `PartUsage`.
    pub element_type: String,
    /// Selected properties by name.
    pub properties: BTreeMap<String, String>,
}

impl QueryElement {
    /// A reported property.
    pub fn get(&self, property: &str) -> Option<&str> {
        self.properties.get(property).map(String::as_str)
    }
}

impl From<QueryElement> for wire::QueryResultElement {
    fn from(element: QueryElement) -> Self {
        Self {
            id: element.id,
            r#type: element.element_type,
            properties: element.properties.into_iter().collect(),
        }
    }
}

impl fmt::Display for QueryElement {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{} ({})", self.id, self.element_type)
    }
}

pub(crate) fn elements_of(response: wire::QueryResponse) -> Vec<QueryElement> {
    response
        .elements
        .into_iter()
        .map(|element| QueryElement {
            id: element.id,
            element_type: element.r#type,
            properties: element.properties.into_iter().collect(),
        })
        .collect()
}

fn reject_unknown(object: &Map<String, Json>, known: &[&str], what: &str) -> Result<(), Error> {
    let mut unknown: Vec<&str> = object
        .keys()
        .map(String::as_str)
        .filter(|key| !known.contains(key))
        .collect();
    if unknown.is_empty() {
        return Ok(());
    }
    unknown.sort_unstable();
    Err(Error::Query(format!(
        "a {what} has no {}",
        unknown.join(", ")
    )))
}

fn entries<'a>(field: &str, value: Option<&'a Json>) -> Result<Vec<&'a Json>, Error> {
    match value {
        None | Some(Json::Null) => Ok(Vec::new()),
        Some(Json::Array(items)) => Ok(items.iter().collect()),
        Some(item @ (Json::String(_) | Json::Object(_))) => Ok(vec![item]),
        Some(other) => Err(Error::Query(format!(
            "{field} is a list, not {}",
            json_kind(other)
        ))),
    }
}

fn scope_id(entry: &Json) -> Result<String, Error> {
    match entry {
        Json::String(id) => Ok(id.clone()),
        Json::Object(reference) => match reference.get("@id") {
            Some(Json::String(id)) => Ok(id.clone()),
            _ => Err(scope_error(entry)),
        },
        _ => Err(scope_error(entry)),
    }
}

fn scope_error(entry: &Json) -> Error {
    Error::Query(format!(
        "a scope entry is an element's qualified name or a {{\"@id\": ...}} reference, not {entry}"
    ))
}

fn compared(value: &Json) -> Result<String, Error> {
    match value {
        Json::Bool(flag) => Ok(flag.to_string()),
        Json::String(text) => Ok(text.clone()),
        Json::Number(number) => Ok(number.to_string()),
        other => Err(Error::Query(format!("cannot compare against {other}"))),
    }
}

fn shown(value: Option<&Json>) -> String {
    value.map_or_else(|| "none".to_owned(), Json::to_string)
}

fn json_kind(value: &Json) -> &'static str {
    match value {
        Json::Null => "null",
        Json::Bool(_) => "a boolean",
        Json::Number(_) => "a number",
        Json::String(_) => "a string",
        Json::Array(_) => "a list",
        Json::Object(_) => "an object",
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn cookbook_payload_reads_as_the_typed_query() {
        let payload = json!({
            "@type": "Query",
            "select": ["name", "owner"],
            "scope": [{"@id": "P::a"}, "P::b"],
            "where": {"@type": "CompositeConstraint", "operator": "or", "constraint": [
                {"property": "name", "operator": "=", "value": ["a", 1, true]},
                {"@type": "PrimitiveConstraint", "property": "x", "operator": ">", "value": 2.5, "inverse": true}
            ]}
        });
        let typed = Query::new()
            .scope(["P::a", "P::b"])
            .select(["name", "owner"])
            .filter(Constraint::or([
                Constraint::one_of("name", ["a", "1", "true"]),
                Constraint::greater("x", "2.5").inverted(),
            ]));
        assert_eq!(Query::from_json(&payload).unwrap(), typed);
        let wire = typed.to_wire().unwrap();
        assert_eq!(wire.scope, ["P::a", "P::b"]);
        let Some(wire::constraint::Constraint::Composite(composite)) =
            wire.r#where.unwrap().constraint
        else {
            panic!("composite expected");
        };
        assert_eq!(composite.operator, wire::CompositeOperator::Or as i32);
        assert_eq!(composite.constraint.len(), 2);
    }

    #[test]
    fn malformed_payloads_are_refused() {
        for (payload, needle) in [
            (json!([]), "a query is an object"),
            (json!({"@type": "Element"}), "expected a \"Query\""),
            (json!({"from": 1}), "a query has no from"),
            (json!({"select": [1]}), "a selected property is a name"),
            (json!({"scope": [3]}), "a scope entry"),
            (json!({"select": 3}), "select is a list"),
            (
                json!({"where": {"property": "x", "operator": "!="}}),
                "unknown primitive operator",
            ),
            (
                json!({"where": {"property": "", "operator": "="}}),
                "names one property",
            ),
            (
                json!({"where": {"operator": "and", "constraint": []}}),
                "non-empty list",
            ),
            (
                json!({"where": {"operator": "xor", "constraint": [{}]}}),
                "unknown composite operator",
            ),
            (
                json!({"where": {"@type": "Other"}}),
                "unknown constraint type",
            ),
            (
                json!({"where": {"property": "x", "operator": "=", "value": [null]}}),
                "cannot compare",
            ),
            (
                json!({"where": {"property": "x", "operator": "=", "extra": 1}}),
                "has no extra",
            ),
        ] {
            let error = Query::from_json(&payload).unwrap_err();
            assert!(matches!(error, Error::Query(_)), "{payload}: {error}");
            assert!(error.to_string().contains(needle), "{payload}: {error}");
        }
    }

    #[test]
    fn empty_typed_constraints_are_refused_before_sending() {
        assert!(Query::new().filter(Constraint::and([])).to_wire().is_err());
        assert!(Query::new()
            .filter(Constraint::equals("", "a"))
            .to_wire()
            .is_err());
    }
}
