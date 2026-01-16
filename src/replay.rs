use std::fmt;
use std::path::Path;

use serde_cbor::Value;

use crate::ast::{Expr, Item, Module, Stmt};
use crate::log::{LogRecord, decode_log};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ReplayError {
    message: String,
}

impl fmt::Display for ReplayError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for ReplayError {}

pub fn replay_workflow(module: &Module, entry: &str, log_path: &Path) -> Result<(), ReplayError> {
    let log_bytes = std::fs::read(log_path).map_err(|err| ReplayError {
        message: format!("replay error: failed to read log: {err}"),
    })?;
    let records = decode_log(&log_bytes).map_err(|err| ReplayError {
        message: format!("replay error: failed to decode log: {err}"),
    })?;

    let workflow = module.items.iter().find_map(|item| match item {
        Item::Workflow(flow) if flow.name == entry => Some(flow),
        _ => None,
    });
    let workflow = workflow.ok_or_else(|| ReplayError {
        message: format!("replay error: workflow '{entry}' not found"),
    })?;

    let steps = workflow
        .body
        .stmts
        .iter()
        .map(|stmt| match stmt {
            Stmt::Step(step) => Ok(step),
            _ => Err(ReplayError {
                message: "replay error: only step statements are supported in workflow body"
                    .to_string(),
            }),
        })
        .collect::<Result<Vec<_>, _>>()?;

    let mut index = 0usize;

    match next_record(&records, &mut index)? {
        LogRecord::WorkflowStart(start) => {
            if start.workflow_name != entry {
                return Err(ReplayError {
                    message: format!("replay error: workflow start mismatch (expected {entry})"),
                });
            }
        }
        _ => {
            return Err(ReplayError {
                message: "replay error: expected WorkflowStart".to_string(),
            });
        }
    }

    for step in steps {
        let step_id = match next_record(&records, &mut index)? {
            LogRecord::StepStart(start) => {
                if start.step_name != step.label {
                    return Err(ReplayError {
                        message: format!(
                            "replay error: step name mismatch (expected {})",
                            step.label
                        ),
                    });
                }
                start.step_id
            }
            _ => {
                return Err(ReplayError {
                    message: "replay error: expected StepStart".to_string(),
                });
            }
        };

        let mut expected_calls = Vec::new();
        for stmt in &step.body.stmts {
            collect_effect_calls_stmt(stmt, &mut expected_calls)?;
        }

        for call in expected_calls {
            match next_record(&records, &mut index)? {
                LogRecord::EffectCall(log_call) => {
                    if log_call.capability != call.capability
                        || log_call.op != call.op
                        || log_call.input != call.input
                    {
                        return Err(ReplayError {
                            message: format!(
                                "replay error: effect call mismatch (expected {}.{})",
                                call.capability, call.op
                            ),
                        });
                    }
                    let call_id = log_call.call_id;
                    match next_record(&records, &mut index)? {
                        LogRecord::EffectResult(result) => {
                            if result.call_id != call_id {
                                return Err(ReplayError {
                                    message: "replay error: effect result call_id mismatch"
                                        .to_string(),
                                });
                            }
                        }
                        _ => {
                            return Err(ReplayError {
                                message: "replay error: expected EffectResult".to_string(),
                            });
                        }
                    }
                }
                _ => {
                    return Err(ReplayError {
                        message: "replay error: expected EffectCall".to_string(),
                    });
                }
            }
        }

        match next_record(&records, &mut index)? {
            LogRecord::StepEnd(end) => {
                if end.step_id != step_id {
                    return Err(ReplayError {
                        message: "replay error: step end id mismatch".to_string(),
                    });
                }
            }
            _ => {
                return Err(ReplayError {
                    message: "replay error: expected StepEnd".to_string(),
                });
            }
        }
    }

    match next_record(&records, &mut index)? {
        LogRecord::WorkflowEnd(_) => {}
        _ => {
            return Err(ReplayError {
                message: "replay error: expected WorkflowEnd".to_string(),
            });
        }
    }

    if index != records.len() {
        return Err(ReplayError {
            message: "replay error: log has extra records".to_string(),
        });
    }

    Ok(())
}

fn next_record<'a>(
    records: &'a [LogRecord],
    index: &mut usize,
) -> Result<&'a LogRecord, ReplayError> {
    let record = records.get(*index).ok_or_else(|| ReplayError {
        message: "replay error: log is shorter than expected".to_string(),
    })?;
    *index += 1;
    Ok(record)
}

#[derive(Debug)]
struct ExpectedEffectCall {
    capability: String,
    op: String,
    input: Vec<u8>,
}

fn collect_effect_calls_stmt(
    stmt: &Stmt,
    calls: &mut Vec<ExpectedEffectCall>,
) -> Result<(), ReplayError> {
    match stmt {
        Stmt::Expr(expr) => collect_effect_calls_expr(expr, calls),
        Stmt::Let(let_stmt) => collect_effect_calls_expr(&let_stmt.expr, calls),
        Stmt::Return(ret) => {
            if let Some(expr) = &ret.expr {
                collect_effect_calls_expr(expr, calls)?;
            }
            Ok(())
        }
        Stmt::If(_) | Stmt::Loop(_) | Stmt::Step(_) => Err(ReplayError {
            message: "replay error: control flow not supported in steps".to_string(),
        }),
    }
}

fn collect_effect_calls_expr(
    expr: &Expr,
    calls: &mut Vec<ExpectedEffectCall>,
) -> Result<(), ReplayError> {
    match expr {
        Expr::Call { callee, args } => {
            for arg in args {
                collect_effect_calls_expr(arg, calls)?;
            }
            if callee.len() >= 2 {
                let op = callee
                    .last()
                    .cloned()
                    .unwrap_or_else(|| "unknown".to_string());
                let capability = callee[..callee.len() - 1].join(".");
                let input = encode_args(args)?;
                calls.push(ExpectedEffectCall {
                    capability,
                    op,
                    input,
                });
            }
            Ok(())
        }
        Expr::Int(_) | Expr::Bool(_) | Expr::String(_) | Expr::Ident(_) | Expr::Path(_) => Ok(()),
    }
}

fn encode_args(args: &[Expr]) -> Result<Vec<u8>, ReplayError> {
    let mut values = Vec::with_capacity(args.len());
    for arg in args {
        let value = match arg {
            Expr::Int(value) => Value::Integer(*value as i128),
            Expr::Bool(value) => Value::Bool(*value),
            Expr::String(value) => Value::Text(value.clone()),
            Expr::Ident(_) | Expr::Path(_) | Expr::Call { .. } => {
                return Err(ReplayError {
                    message: "replay error: unsupported effect argument".to_string(),
                });
            }
        };
        values.push(value);
    }
    serde_cbor::to_vec(&Value::Array(values)).map_err(|err| ReplayError {
        message: format!("replay error: failed to encode arguments: {err}"),
    })
}
