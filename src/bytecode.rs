use std::collections::HashMap;
use std::fmt;

use crate::ast::{Block, Expr, FnDecl, Item, Module, Stmt};
use crate::interpreter::Value;

#[derive(Debug, Clone)]
pub struct BytecodeModule {
    functions: Vec<Function>,
    name_to_index: HashMap<String, usize>,
}

#[derive(Debug, Clone)]
struct Function {
    name: String,
    params: usize,
    locals: usize,
    code: Vec<Instr>,
}

#[derive(Debug, Clone)]
enum Instr {
    ConstInt(i64),
    ConstBool(bool),
    ConstString(String),
    ConstUnit,
    LoadLocal(u32),
    StoreLocal(u32),
    Pop,
    Call { func: usize, argc: usize },
    CallBuiltin { name: String, argc: usize },
    Jump(usize),
    JumpIfFalse(usize),
    Return,
}

#[derive(Debug)]
pub struct CompileError {
    message: String,
}

impl fmt::Display for CompileError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for CompileError {}

#[derive(Debug)]
pub struct VmError {
    message: String,
}

impl fmt::Display for VmError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for VmError {}

pub fn compile_module(module: &Module) -> Result<BytecodeModule, CompileError> {
    let mut functions = Vec::new();
    let mut name_to_index = HashMap::new();

    for item in &module.items {
        match item {
            Item::Fn(func) => {
                if name_to_index.contains_key(&func.name) {
                    return Err(CompileError {
                        message: format!("duplicate function '{}'", func.name),
                    });
                }
                let index = functions.len();
                name_to_index.insert(func.name.clone(), index);
                functions.push(Function {
                    name: func.name.clone(),
                    params: 0,
                    locals: 0,
                    code: Vec::new(),
                });
            }
            Item::Workflow(_) => {
                return Err(CompileError {
                    message: "bytecode VM does not support workflows".to_string(),
                });
            }
        }
    }

    for item in &module.items {
        let Item::Fn(func) = item else {
            continue;
        };
        let index = *name_to_index.get(&func.name).ok_or_else(|| CompileError {
            message: format!("missing function '{}' during compile", func.name),
        })?;
        let compiled = compile_function(func, &name_to_index)?;
        functions[index] = compiled;
    }

    Ok(BytecodeModule {
        functions,
        name_to_index,
    })
}

impl BytecodeModule {
    pub fn run_entry(&self, name: &str, args: Vec<Value>) -> Result<Value, VmError> {
        let index = *self.name_to_index.get(name).ok_or_else(|| VmError {
            message: format!("entry function '{}' not found", name),
        })?;
        let mut vm = Vm::new(self);
        vm.run(index, args)
    }
}

fn compile_function(
    func: &FnDecl,
    name_to_index: &HashMap<String, usize>,
) -> Result<Function, CompileError> {
    let mut locals = HashMap::new();
    let mut local_count = 0u32;
    for param in &func.params {
        locals.insert(param.name.clone(), local_count);
        local_count += 1;
    }

    let mut code = Vec::new();
    compile_block(
        &func.body,
        &mut locals,
        &mut local_count,
        name_to_index,
        &mut code,
    )?;
    code.push(Instr::ConstUnit);
    code.push(Instr::Return);

    Ok(Function {
        name: func.name.clone(),
        params: func.params.len(),
        locals: local_count as usize,
        code,
    })
}

fn compile_block(
    block: &Block,
    locals: &mut HashMap<String, u32>,
    local_count: &mut u32,
    name_to_index: &HashMap<String, usize>,
    code: &mut Vec<Instr>,
) -> Result<(), CompileError> {
    for stmt in &block.stmts {
        compile_stmt(stmt, locals, local_count, name_to_index, code)?;
    }
    Ok(())
}

fn compile_stmt(
    stmt: &Stmt,
    locals: &mut HashMap<String, u32>,
    local_count: &mut u32,
    name_to_index: &HashMap<String, usize>,
    code: &mut Vec<Instr>,
) -> Result<(), CompileError> {
    match stmt {
        Stmt::Let(let_stmt) => {
            compile_expr(&let_stmt.expr, locals, name_to_index, code)?;
            let index = *locals.entry(let_stmt.name.clone()).or_insert_with(|| {
                let next = *local_count;
                *local_count += 1;
                next
            });
            code.push(Instr::StoreLocal(index));
        }
        Stmt::Return(ret) => {
            if let Some(expr) = &ret.expr {
                compile_expr(expr, locals, name_to_index, code)?;
            } else {
                code.push(Instr::ConstUnit);
            }
            code.push(Instr::Return);
        }
        Stmt::Expr(expr) => {
            compile_expr(expr, locals, name_to_index, code)?;
            code.push(Instr::Pop);
        }
        Stmt::If(if_stmt) => {
            compile_expr(&if_stmt.condition, locals, name_to_index, code)?;
            let jump_false_at = code.len();
            code.push(Instr::JumpIfFalse(usize::MAX));
            compile_block(
                &if_stmt.then_block,
                locals,
                local_count,
                name_to_index,
                code,
            )?;
            let jump_end_at = code.len();
            code.push(Instr::Jump(usize::MAX));
            let else_start = code.len();
            if let Some(else_block) = &if_stmt.else_block {
                compile_block(else_block, locals, local_count, name_to_index, code)?;
            }
            let end = code.len();
            code[jump_false_at] = Instr::JumpIfFalse(else_start);
            code[jump_end_at] = Instr::Jump(end);
        }
        Stmt::Loop(_) => {
            return Err(CompileError {
                message: "bytecode VM does not support loop yet".to_string(),
            });
        }
        Stmt::Step(_) => {
            return Err(CompileError {
                message: "bytecode VM does not support step yet".to_string(),
            });
        }
    }
    Ok(())
}

fn compile_expr(
    expr: &Expr,
    locals: &HashMap<String, u32>,
    name_to_index: &HashMap<String, usize>,
    code: &mut Vec<Instr>,
) -> Result<(), CompileError> {
    match expr {
        Expr::Int(value) => code.push(Instr::ConstInt(*value)),
        Expr::Bool(value) => code.push(Instr::ConstBool(*value)),
        Expr::String(value) => code.push(Instr::ConstString(value.clone())),
        Expr::Ident(name) => {
            let index = locals.get(name).ok_or_else(|| CompileError {
                message: format!("unknown variable '{name}'"),
            })?;
            code.push(Instr::LoadLocal(*index));
        }
        Expr::Path(path) => {
            if path.len() == 1 {
                let name = &path[0];
                let index = locals.get(name).ok_or_else(|| CompileError {
                    message: format!("unknown variable '{name}'"),
                })?;
                code.push(Instr::LoadLocal(*index));
            } else {
                return Err(CompileError {
                    message: "qualified paths are not supported".to_string(),
                });
            }
        }
        Expr::Call { callee, args } => {
            for arg in args {
                compile_expr(arg, locals, name_to_index, code)?;
            }
            if callee.len() != 1 {
                return Err(CompileError {
                    message: "qualified function calls are not supported".to_string(),
                });
            }
            let name = &callee[0];
            if name.starts_with("__") {
                code.push(Instr::CallBuiltin {
                    name: name.clone(),
                    argc: args.len(),
                });
                return Ok(());
            }
            let func = name_to_index.get(name).ok_or_else(|| CompileError {
                message: format!("unknown function '{name}'"),
            })?;
            code.push(Instr::Call {
                func: *func,
                argc: args.len(),
            });
        }
    }
    Ok(())
}

struct Vm<'a> {
    module: &'a BytecodeModule,
    frames: Vec<Frame>,
}

struct Frame {
    func: usize,
    ip: usize,
    locals: Vec<Value>,
    stack: Vec<Value>,
}

impl<'a> Vm<'a> {
    fn new(module: &'a BytecodeModule) -> Self {
        Self {
            module,
            frames: Vec::new(),
        }
    }

    fn run(&mut self, entry: usize, args: Vec<Value>) -> Result<Value, VmError> {
        let func = self.module.functions.get(entry).ok_or_else(|| VmError {
            message: "entry function index out of range".to_string(),
        })?;
        if func.params != args.len() {
            return Err(VmError {
                message: format!(
                    "function '{}' expects {} args but got {}",
                    func.name,
                    func.params,
                    args.len()
                ),
            });
        }
        let mut locals = vec![Value::Unit; func.locals];
        for (index, arg) in args.into_iter().enumerate() {
            locals[index] = arg;
        }
        self.frames.push(Frame {
            func: entry,
            ip: 0,
            locals,
            stack: Vec::new(),
        });

        loop {
            let frame = self.frames.last_mut().ok_or_else(|| VmError {
                message: "vm frame stack underflow".to_string(),
            })?;
            let func = &self.module.functions[frame.func];
            if frame.ip >= func.code.len() {
                return Err(VmError {
                    message: format!("instruction pointer out of range in '{}'", func.name),
                });
            }
            let instr = func.code[frame.ip].clone();
            frame.ip += 1;
            match instr {
                Instr::ConstInt(value) => frame.stack.push(Value::Int(value)),
                Instr::ConstBool(value) => frame.stack.push(Value::Bool(value)),
                Instr::ConstString(value) => frame.stack.push(Value::String(value)),
                Instr::ConstUnit => frame.stack.push(Value::Unit),
                Instr::LoadLocal(index) => {
                    let value = frame.locals.get(index as usize).ok_or_else(|| VmError {
                        message: "local index out of range".to_string(),
                    })?;
                    frame.stack.push(value.clone());
                }
                Instr::StoreLocal(index) => {
                    let value = frame.stack.pop().ok_or_else(|| VmError {
                        message: "stack underflow on store".to_string(),
                    })?;
                    let slot = frame
                        .locals
                        .get_mut(index as usize)
                        .ok_or_else(|| VmError {
                            message: "local index out of range".to_string(),
                        })?;
                    *slot = value;
                }
                Instr::Pop => {
                    frame.stack.pop().ok_or_else(|| VmError {
                        message: "stack underflow on pop".to_string(),
                    })?;
                }
                Instr::Call { func, argc } => {
                    let mut args = Vec::with_capacity(argc);
                    for _ in 0..argc {
                        args.push(frame.stack.pop().ok_or_else(|| VmError {
                            message: "stack underflow on call".to_string(),
                        })?);
                    }
                    args.reverse();
                    let callee = self.module.functions.get(func).ok_or_else(|| VmError {
                        message: "function index out of range".to_string(),
                    })?;
                    if callee.params != argc {
                        return Err(VmError {
                            message: format!(
                                "function '{}' expects {} args but got {}",
                                callee.name, callee.params, argc
                            ),
                        });
                    }
                    let mut locals = vec![Value::Unit; callee.locals];
                    for (index, arg) in args.into_iter().enumerate() {
                        locals[index] = arg;
                    }
                    self.frames.push(Frame {
                        func,
                        ip: 0,
                        locals,
                        stack: Vec::new(),
                    });
                }
                Instr::CallBuiltin { name, argc } => {
                    let mut args = Vec::with_capacity(argc);
                    for _ in 0..argc {
                        args.push(frame.stack.pop().ok_or_else(|| VmError {
                            message: "stack underflow on builtin call".to_string(),
                        })?);
                    }
                    args.reverse();
                    match crate::interpreter::eval_builtin(&name, &args) {
                        Some(result) => {
                            let value = result.map_err(|err| VmError {
                                message: err.to_string(),
                            })?;
                            frame.stack.push(value);
                        }
                        None => {
                            return Err(VmError {
                                message: format!("unknown builtin '{name}'"),
                            });
                        }
                    }
                }
                Instr::Jump(target) => {
                    frame.ip = target;
                }
                Instr::JumpIfFalse(target) => {
                    let value = frame.stack.pop().ok_or_else(|| VmError {
                        message: "stack underflow on jump".to_string(),
                    })?;
                    match value {
                        Value::Bool(false) => frame.ip = target,
                        Value::Bool(true) => {}
                        _ => {
                            return Err(VmError {
                                message: "if condition must be boolean".to_string(),
                            });
                        }
                    }
                }
                Instr::Return => {
                    let value = frame.stack.pop().unwrap_or(Value::Unit);
                    self.frames.pop();
                    if let Some(parent) = self.frames.last_mut() {
                        parent.stack.push(value);
                    } else {
                        return Ok(value);
                    }
                }
            }
        }
    }
}
