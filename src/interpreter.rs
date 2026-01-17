use std::collections::HashMap;
use std::fmt;

use crate::ast::{Block, Expr, FnDecl, Item, Module, Stmt};

#[derive(Debug, Clone, PartialEq)]
pub enum Value {
    Int(i64),
    Bool(bool),
    String(String),
    Unit,
}

impl Value {
    pub fn is_unit(&self) -> bool {
        matches!(self, Value::Unit)
    }
}

impl fmt::Display for Value {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Value::Int(value) => write!(f, "{value}"),
            Value::Bool(value) => write!(f, "{value}"),
            Value::String(value) => write!(f, "{value}"),
            Value::Unit => Ok(()),
        }
    }
}

#[derive(Debug)]
pub struct RunError {
    message: String,
}

impl fmt::Display for RunError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for RunError {}

pub fn run_entry(module: &Module, entry: &str) -> Result<Value, RunError> {
    run_entry_with_args(module, entry, Vec::new())
}

pub fn run_entry_with_args(
    module: &Module,
    entry: &str,
    args: Vec<Value>,
) -> Result<Value, RunError> {
    let mut functions = HashMap::new();
    for item in &module.items {
        if let Item::Fn(func) = item {
            functions.insert(func.name.as_str(), func);
        }
    }

    let mut interpreter = Interpreter { functions };
    let entry_fn = *interpreter.functions.get(entry).ok_or_else(|| RunError {
        message: format!("entry function '{entry}' not found"),
    })?;

    interpreter.eval_fn(entry_fn, args)
}

struct Interpreter<'a> {
    functions: HashMap<&'a str, &'a FnDecl>,
}

enum Control {
    Continue,
    Return(Value),
}

impl<'a> Interpreter<'a> {
    fn eval_fn(&mut self, func: &'a FnDecl, args: Vec<Value>) -> Result<Value, RunError> {
        if func.params.len() != args.len() {
            return Err(RunError {
                message: format!(
                    "function '{}' expects {} args but got {}",
                    func.name,
                    func.params.len(),
                    args.len()
                ),
            });
        }

        let mut env = HashMap::new();
        for (param, arg) in func.params.iter().zip(args) {
            env.insert(param.name.clone(), arg);
        }

        match self.eval_block(&func.body, &mut env)? {
            Control::Continue => Ok(Value::Unit),
            Control::Return(value) => Ok(value),
        }
    }

    fn eval_block(
        &mut self,
        block: &'a Block,
        env: &mut HashMap<String, Value>,
    ) -> Result<Control, RunError> {
        for stmt in &block.stmts {
            match self.eval_stmt(stmt, env)? {
                Control::Continue => {}
                Control::Return(value) => return Ok(Control::Return(value)),
            }
        }
        Ok(Control::Continue)
    }

    fn eval_stmt(
        &mut self,
        stmt: &'a Stmt,
        env: &mut HashMap<String, Value>,
    ) -> Result<Control, RunError> {
        match stmt {
            Stmt::Let(let_stmt) => {
                let value = self.eval_expr(&let_stmt.expr, env)?;
                env.insert(let_stmt.name.clone(), value);
                Ok(Control::Continue)
            }
            Stmt::Return(ret) => {
                let value = if let Some(expr) = &ret.expr {
                    self.eval_expr(expr, env)?
                } else {
                    Value::Unit
                };
                Ok(Control::Return(value))
            }
            Stmt::Expr(expr) => {
                let _ = self.eval_expr(expr, env)?;
                Ok(Control::Continue)
            }
            Stmt::If(if_stmt) => {
                let cond = self.eval_expr(&if_stmt.condition, env)?;
                match cond {
                    Value::Bool(true) => self.eval_block(&if_stmt.then_block, env),
                    Value::Bool(false) => {
                        if let Some(else_block) = &if_stmt.else_block {
                            self.eval_block(else_block, env)
                        } else {
                            Ok(Control::Continue)
                        }
                    }
                    _ => Err(RunError {
                        message: "if condition must be boolean".to_string(),
                    }),
                }
            }
            Stmt::Loop(_) => Err(RunError {
                message: "loop is not supported in the interpreter yet".to_string(),
            }),
            Stmt::Step(_) => Err(RunError {
                message: "step is not supported in the interpreter".to_string(),
            }),
        }
    }

    fn eval_expr(
        &mut self,
        expr: &'a Expr,
        env: &mut HashMap<String, Value>,
    ) -> Result<Value, RunError> {
        match expr {
            Expr::Int(value) => Ok(Value::Int(*value)),
            Expr::Bool(value) => Ok(Value::Bool(*value)),
            Expr::String(value) => Ok(Value::String(value.clone())),
            Expr::Ident(name) => env.get(name).cloned().ok_or_else(|| RunError {
                message: format!("unknown variable '{name}'"),
            }),
            Expr::Path(path) => {
                if path.len() == 1 {
                    let name = &path[0];
                    env.get(name).cloned().ok_or_else(|| RunError {
                        message: format!("unknown variable '{name}'"),
                    })
                } else {
                    Err(RunError {
                        message: "qualified paths are not supported in expressions".to_string(),
                    })
                }
            }
            Expr::Call { callee, args } => {
                let mut eval_args = Vec::with_capacity(args.len());
                for arg in args {
                    eval_args.push(self.eval_expr(arg, env)?);
                }

                if callee.len() != 1 {
                    return Err(RunError {
                        message: "qualified function calls are not supported".to_string(),
                    });
                }
                let name = &callee[0];
                if let Some(result) = eval_builtin(name, &eval_args) {
                    return result;
                }
                let func = self.functions.get(name.as_str()).ok_or_else(|| RunError {
                    message: format!("unknown function '{name}'"),
                })?;
                self.eval_fn(func, eval_args)
            }
        }
    }
}

fn eval_builtin(name: &str, args: &[Value]) -> Option<Result<Value, RunError>> {
    match name {
        "__str_eq" => Some(eval_str_eq(args)),
        _ => None,
    }
}

fn eval_str_eq(args: &[Value]) -> Result<Value, RunError> {
    if args.len() != 2 {
        return Err(RunError {
            message: format!("__str_eq expects 2 args but got {}", args.len()),
        });
    }
    let left = match &args[0] {
        Value::String(value) => value,
        _ => {
            return Err(RunError {
                message: "__str_eq expects String args".to_string(),
            });
        }
    };
    let right = match &args[1] {
        Value::String(value) => value,
        _ => {
            return Err(RunError {
                message: "__str_eq expects String args".to_string(),
            });
        }
    };
    Ok(Value::Bool(left == right))
}
