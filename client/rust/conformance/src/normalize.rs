use crate::compare::RUNTIME_ID_KEYS;
use serde_json::Value;

pub const MODEL_HASH: &str = "${model_hash}";
pub const VERSION: &str = "${version}";
pub const PATH: &str = "${path}";

pub fn normalize(value: &mut Value, model_hash: &str) {
    normalize_inner(value, model_hash);
}

fn normalize_inner(value: &mut Value, model_hash: &str) {
    match value {
        Value::Object(object) => {
            let is_instance = object.contains_key("type_symbol_id")
                && object.contains_key("feature_values")
                && object.contains_key("id");
            let is_server_info =
                object.contains_key("capabilities") && object.contains_key("version");
            // A function bound to no object leaves self_id at its default, which
            // the wire renders as absent rather than as an id to label.
            if object.get("self_id").and_then(Value::as_i64) == Some(0) {
                object.remove("self_id");
            }
            for (key, child) in object.iter_mut() {
                if is_server_info && key == "version" {
                    *child = Value::String(VERSION.to_owned());
                } else if is_instance && key == "id" {
                    // Runtime ids are labelled in one pass after all other
                    // normalization, where the response-wide map is shared.
                } else if RUNTIME_ID_KEYS.contains(&key.as_str()) {
                    // See compare::label_instance_ids.
                } else {
                    normalize_inner(child, model_hash);
                }
            }
        }
        Value::Array(items) => {
            for item in items {
                normalize_inner(item, model_hash);
            }
        }
        Value::String(text) => {
            if !model_hash.is_empty() && text == model_hash {
                *text = MODEL_HASH.to_owned();
            } else if is_absolute_path(text) {
                *text = PATH.to_owned();
            }
        }
        _ => {}
    }
}

fn is_absolute_path(value: &str) -> bool {
    value.starts_with('/')
        || (value.len() > 2 && value.as_bytes()[1] == b':' && value.as_bytes()[2] == b'\\')
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn a_function_bound_to_no_object_has_no_self_id() {
        let mut unbound = json!({"result": {"function": {"calc_id": "F::Sq", "self_id": 0}}});
        normalize(&mut unbound, "");
        assert_eq!(
            unbound,
            json!({"result": {"function": {"calc_id": "F::Sq"}}})
        );

        let mut bound = json!({"result": {"function": {"calc_id": "F::S::f", "self_id": 7}}});
        normalize(&mut bound, "");
        assert_eq!(bound["result"]["function"]["self_id"], 7);
    }
}
