use std::collections::{BTreeMap, HashMap};
use std::fmt;

use serde_cbor::Value;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorkflowStart {
    pub workflow_name: String,
    pub args: Vec<u8>,
    pub run_id: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StepStart {
    pub step_id: u64,
    pub step_name: String,
    pub attempt: u32,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EffectCall {
    pub call_id: u64,
    pub capability: String,
    pub op: String,
    pub input: Vec<u8>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EffectResult {
    pub call_id: u64,
    pub outcome: Outcome,
    pub output: Vec<u8>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StepEnd {
    pub step_id: u64,
    pub outcome: Outcome,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WorkflowEnd {
    pub outcome: Outcome,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Outcome {
    Ok,
    Err,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LogRecord {
    WorkflowStart(WorkflowStart),
    StepStart(StepStart),
    EffectCall(EffectCall),
    EffectResult(EffectResult),
    StepEnd(StepEnd),
    WorkflowEnd(WorkflowEnd),
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LogError {
    message: String,
}

impl fmt::Display for LogError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for LogError {}

pub fn encode_record(record: &LogRecord) -> Result<Vec<u8>, LogError> {
    let value = encode_record_value(record);
    serde_cbor::to_vec(&value).map_err(|err| LogError {
        message: format!("encode error: {err}"),
    })
}

pub fn decode_record(bytes: &[u8]) -> Result<LogRecord, LogError> {
    let value: Value = serde_cbor::from_slice(bytes).map_err(|err| LogError {
        message: format!("decode error: {err}"),
    })?;
    decode_record_value(value)
}

fn encode_record_value(record: &LogRecord) -> Value {
    match record {
        LogRecord::WorkflowStart(start) => map_from_pairs(vec![
            (0, Value::Integer(0)),
            (1, Value::Text(start.workflow_name.clone())),
            (2, Value::Bytes(start.args.clone())),
            (3, Value::Text(start.run_id.clone())),
        ]),
        LogRecord::StepStart(start) => map_from_pairs(vec![
            (0, Value::Integer(1)),
            (1, Value::Integer(start.step_id as i128)),
            (2, Value::Text(start.step_name.clone())),
            (3, Value::Integer(start.attempt as i128)),
        ]),
        LogRecord::EffectCall(call) => map_from_pairs(vec![
            (0, Value::Integer(2)),
            (1, Value::Integer(call.call_id as i128)),
            (2, Value::Text(call.capability.clone())),
            (3, Value::Text(call.op.clone())),
            (4, Value::Bytes(call.input.clone())),
        ]),
        LogRecord::EffectResult(result) => map_from_pairs(vec![
            (0, Value::Integer(3)),
            (1, Value::Integer(result.call_id as i128)),
            (2, Value::Integer(outcome_to_int(result.outcome) as i128)),
            (3, Value::Bytes(result.output.clone())),
        ]),
        LogRecord::StepEnd(end) => map_from_pairs(vec![
            (0, Value::Integer(4)),
            (1, Value::Integer(end.step_id as i128)),
            (2, Value::Integer(outcome_to_int(end.outcome) as i128)),
        ]),
        LogRecord::WorkflowEnd(end) => map_from_pairs(vec![
            (0, Value::Integer(5)),
            (1, Value::Integer(outcome_to_int(end.outcome) as i128)),
        ]),
    }
}

fn decode_record_value(value: Value) -> Result<LogRecord, LogError> {
    let map = match value {
        Value::Map(map) => map,
        _ => {
            return Err(LogError {
                message: "decode error: expected map".to_string(),
            });
        }
    };
    let fields = collect_fields(map)?;
    let kind = required_int(&fields, 0, "kind")? as u64;

    match kind {
        0 => Ok(LogRecord::WorkflowStart(WorkflowStart {
            workflow_name: required_text(&fields, 1, "workflow_name")?,
            args: required_bytes(&fields, 2, "args")?,
            run_id: required_text(&fields, 3, "run_id")?,
        })),
        1 => Ok(LogRecord::StepStart(StepStart {
            step_id: required_u64(&fields, 1, "step_id")?,
            step_name: required_text(&fields, 2, "step_name")?,
            attempt: required_u32(&fields, 3, "attempt")?,
        })),
        2 => Ok(LogRecord::EffectCall(EffectCall {
            call_id: required_u64(&fields, 1, "call_id")?,
            capability: required_text(&fields, 2, "capability")?,
            op: required_text(&fields, 3, "op")?,
            input: required_bytes(&fields, 4, "input")?,
        })),
        3 => Ok(LogRecord::EffectResult(EffectResult {
            call_id: required_u64(&fields, 1, "call_id")?,
            outcome: required_outcome(&fields, 2, "outcome")?,
            output: required_bytes(&fields, 3, "output")?,
        })),
        4 => Ok(LogRecord::StepEnd(StepEnd {
            step_id: required_u64(&fields, 1, "step_id")?,
            outcome: required_outcome(&fields, 2, "outcome")?,
        })),
        5 => Ok(LogRecord::WorkflowEnd(WorkflowEnd {
            outcome: required_outcome(&fields, 1, "outcome")?,
        })),
        _ => Err(LogError {
            message: format!("decode error: unknown record kind {kind}"),
        }),
    }
}

fn collect_fields(map: BTreeMap<Value, Value>) -> Result<HashMap<u64, Value>, LogError> {
    let mut fields = HashMap::new();
    for (key, value) in map {
        let key = match key {
            Value::Integer(int) => int,
            _ => {
                return Err(LogError {
                    message: "decode error: map key must be integer".to_string(),
                });
            }
        };
        let key = u64::try_from(key).map_err(|_| LogError {
            message: "decode error: map key out of range".to_string(),
        })?;
        fields.insert(key, value);
    }
    Ok(fields)
}

fn required_value<'a>(
    fields: &'a HashMap<u64, Value>,
    key: u64,
    name: &str,
) -> Result<&'a Value, LogError> {
    fields.get(&key).ok_or_else(|| LogError {
        message: format!("decode error: missing field {name}"),
    })
}

fn required_text(fields: &HashMap<u64, Value>, key: u64, name: &str) -> Result<String, LogError> {
    match required_value(fields, key, name)? {
        Value::Text(text) => Ok(text.clone()),
        _ => Err(LogError {
            message: format!("decode error: field {name} must be text"),
        }),
    }
}

fn required_bytes(fields: &HashMap<u64, Value>, key: u64, name: &str) -> Result<Vec<u8>, LogError> {
    match required_value(fields, key, name)? {
        Value::Bytes(bytes) => Ok(bytes.clone()),
        _ => Err(LogError {
            message: format!("decode error: field {name} must be bytes"),
        }),
    }
}

fn required_int(fields: &HashMap<u64, Value>, key: u64, name: &str) -> Result<i128, LogError> {
    match required_value(fields, key, name)? {
        Value::Integer(value) => Ok(*value),
        _ => Err(LogError {
            message: format!("decode error: field {name} must be integer"),
        }),
    }
}

fn required_u64(fields: &HashMap<u64, Value>, key: u64, name: &str) -> Result<u64, LogError> {
    let value = required_int(fields, key, name)?;
    u64::try_from(value).map_err(|_| LogError {
        message: format!("decode error: field {name} out of range"),
    })
}

fn required_u32(fields: &HashMap<u64, Value>, key: u64, name: &str) -> Result<u32, LogError> {
    let value = required_int(fields, key, name)?;
    u32::try_from(value).map_err(|_| LogError {
        message: format!("decode error: field {name} out of range"),
    })
}

fn required_outcome(
    fields: &HashMap<u64, Value>,
    key: u64,
    name: &str,
) -> Result<Outcome, LogError> {
    let value = required_int(fields, key, name)?;
    match value {
        0 => Ok(Outcome::Ok),
        1 => Ok(Outcome::Err),
        _ => Err(LogError {
            message: format!("decode error: field {name} invalid outcome"),
        }),
    }
}

fn outcome_to_int(outcome: Outcome) -> u8 {
    match outcome {
        Outcome::Ok => 0,
        Outcome::Err => 1,
    }
}

fn map_from_pairs(pairs: Vec<(i128, Value)>) -> Value {
    let mut map = BTreeMap::new();
    for (key, value) in pairs {
        map.insert(Value::Integer(key), value);
    }
    Value::Map(map)
}
