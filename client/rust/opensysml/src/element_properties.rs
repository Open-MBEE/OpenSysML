//! Reading an element's metamodel properties by their specification names.
//!
//! The `GetElementProperties` RPC answers the KerML 1.1 / SysML 2.1 abstract
//! syntax of one element — `inheritedFeature`, `connectorEnd`,
//! `directedFeature`, `payloadType`, ... — as the model's API element form
//! (`Convert` to `api-json`) spells it, without converting the whole model. An
//! element is named by qualified name, by the positional id of an unnamed
//! member (`Owner::@2`) or by the element form's own id, which every element
//! reference in an answer carries, so a client walks the graph with one vocabulary.

use crate::wire;

/// An element the model's element form knows: one of the model, or a
/// standard-library element it references.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct ElementIdentity {
    /// The element form's `@id`, accepted back as the element to read.
    pub id: String,
    /// Empty for an unnamed element; an unnamed standard-library member
    /// carries its library path (`owner/Metaclass[i]`).
    pub qualified_name: String,
    /// `ConnectionUsage`, `PartDefinition`, ...; empty when the id is known to nothing.
    pub metaclass: String,
}

impl ElementIdentity {
    /// The qualified name, or the id for an unnamed element.
    pub fn display_name(&self) -> &str {
        if self.qualified_name.is_empty() {
            &self.id
        } else {
            &self.qualified_name
        }
    }
}

impl std::fmt::Display for ElementIdentity {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.display_name())
    }
}

/// One value of a metamodel property, as the element form spells it.
#[derive(Clone, Debug, PartialEq)]
pub enum PropertyValue {
    /// An element reference.
    Element(ElementIdentity),
    /// A string, or an enumeration literal (`in`, `public`).
    String(String),
    /// A Boolean.
    Bool(bool),
    /// An integer.
    Int(i64),
    /// A real.
    Real(f64),
    /// A number beyond `i64` and `f64`, in decimal.
    Number(String),
}

impl PropertyValue {
    /// The element reference, for an element value.
    pub fn as_element(&self) -> Option<&ElementIdentity> {
        match self {
            Self::Element(element) => Some(element),
            _ => None,
        }
    }
    /// The text of the value: an element's display name, a literal's spelling.
    pub fn text(&self) -> String {
        match self {
            Self::Element(element) => element.display_name().to_owned(),
            Self::String(s) | Self::Number(s) => s.clone(),
            Self::Bool(b) => b.to_string(),
            Self::Int(i) => i.to_string(),
            Self::Real(r) => r.to_string(),
        }
    }
}

/// One metamodel property of an element.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct ElementProperty {
    /// The specification name (`connectorEnd`).
    pub name: String,
    /// The metaclass declaring the property (`Connector`); empty when the
    /// element's metaclass declares no such property.
    pub defining_class: String,
    /// Whether the property holds a collection.
    pub many: bool,
    /// Whether the specification derives the property.
    pub derived: bool,
    /// The values in the collection's order; empty for an empty or null property.
    pub values: Vec<PropertyValue>,
    /// Why there are no values when the property cannot be answered: the
    /// metaclass declares no such property, or OpenSysML does not derive it
    /// yet. Empty for a property that is answered.
    pub unsupported: String,
}

impl ElementProperty {
    /// Whether the property was answered, with or without values.
    pub fn supported(&self) -> bool {
        self.unsupported.is_empty()
    }
    /// The one value of a single-valued property.
    pub fn value(&self) -> Option<&PropertyValue> {
        self.values.first()
    }
    /// The element references among the values, in order.
    pub fn elements(&self) -> Vec<&ElementIdentity> {
        self.values
            .iter()
            .filter_map(PropertyValue::as_element)
            .collect()
    }
}

/// What one read answered: the element and its properties, in the order asked
/// for, or the element form's order when none was named.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct ElementProperties {
    /// The element read.
    pub element: ElementIdentity,
    /// One entry per property.
    pub properties: Vec<ElementProperty>,
}

impl ElementProperties {
    /// The entry for the named property, when the read asked for it.
    pub fn get(&self, name: &str) -> Option<&ElementProperty> {
        self.properties.iter().find(|p| p.name == name)
    }
}

pub(crate) fn element_identity_of(reference: wire::ElementRef) -> ElementIdentity {
    ElementIdentity {
        id: reference.id,
        qualified_name: reference.qualified_name,
        metaclass: reference.metaclass,
    }
}

pub(crate) fn property_value_of(value: wire::ElementPropertyValue) -> Option<PropertyValue> {
    use wire::element_property_value::Value;
    Some(match value.value? {
        Value::Element(element) => PropertyValue::Element(element_identity_of(element)),
        Value::StringValue(s) => PropertyValue::String(s),
        Value::BoolValue(b) => PropertyValue::Bool(b),
        Value::IntValue(i) => PropertyValue::Int(i),
        Value::RealValue(r) => PropertyValue::Real(r),
        Value::Number(n) => PropertyValue::Number(n),
    })
}

pub(crate) fn element_properties_of(
    response: wire::GetElementPropertiesResponse,
) -> ElementProperties {
    ElementProperties {
        element: response
            .element
            .map(element_identity_of)
            .unwrap_or_default(),
        properties: response
            .properties
            .into_iter()
            .map(|p| ElementProperty {
                name: p.name,
                defining_class: p.defining_class,
                many: p.many,
                derived: p.derived,
                values: p.values.into_iter().filter_map(property_value_of).collect(),
                unsupported: p.unsupported,
            })
            .collect(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use wire::element_property_value::Value;

    fn reference(id: &str, name: &str, metaclass: &str) -> wire::ElementRef {
        wire::ElementRef {
            id: id.to_owned(),
            qualified_name: name.to_owned(),
            metaclass: metaclass.to_owned(),
        }
    }

    #[test]
    fn a_response_decodes_into_properties_with_their_values() {
        let response = wire::GetElementPropertiesResponse {
            element: Some(reference("EP__Pair", "EP::Pair", "PartDefinition")),
            properties: vec![
                wire::ElementProperty {
                    name: "inheritedFeature".to_owned(),
                    defining_class: "Type".to_owned(),
                    many: true,
                    derived: true,
                    values: vec![
                        wire::ElementPropertyValue {
                            value: Some(Value::Element(reference(
                                "EP__Unit__p",
                                "EP::Unit::p",
                                "PortUsage",
                            ))),
                        },
                        wire::ElementPropertyValue {
                            value: Some(Value::Element(reference("4e03", "", "PartUsage"))),
                        },
                    ],
                    unsupported: String::new(),
                },
                wire::ElementProperty {
                    name: "isAbstract".to_owned(),
                    defining_class: "Type".to_owned(),
                    values: vec![wire::ElementPropertyValue {
                        value: Some(Value::BoolValue(false)),
                    }],
                    ..Default::default()
                },
                wire::ElementProperty {
                    name: "connectorEnd".to_owned(),
                    unsupported: "PartDefinition declares no property connectorEnd".to_owned(),
                    ..Default::default()
                },
            ],
            error: String::new(),
        };
        let read = element_properties_of(response);
        assert_eq!(read.element.to_string(), "EP::Pair");
        let inherited = read.get("inheritedFeature").unwrap();
        assert!(inherited.many && inherited.derived && inherited.supported());
        let names: Vec<String> = inherited.elements().iter().map(|e| e.to_string()).collect();
        assert_eq!(names, ["EP::Unit::p", "4e03"]);
        assert_eq!(
            read.get("isAbstract").unwrap().value(),
            Some(&PropertyValue::Bool(false))
        );
        assert_eq!(
            read.get("isAbstract").unwrap().value().unwrap().text(),
            "false"
        );
        assert!(!read.get("connectorEnd").unwrap().supported());
        assert!(read.get("name").is_none());
    }

    #[test]
    fn every_value_arm_decodes() {
        let decode = |value| property_value_of(wire::ElementPropertyValue { value: Some(value) });
        assert_eq!(
            decode(Value::StringValue("out".into())),
            Some(PropertyValue::String("out".into()))
        );
        assert_eq!(decode(Value::IntValue(3)), Some(PropertyValue::Int(3)));
        assert_eq!(
            decode(Value::RealValue(2.5)),
            Some(PropertyValue::Real(2.5))
        );
        assert_eq!(
            decode(Value::Number("1".repeat(30))),
            Some(PropertyValue::Number("1".repeat(30)))
        );
        assert_eq!(
            property_value_of(wire::ElementPropertyValue { value: None }),
            None
        );
    }
}
