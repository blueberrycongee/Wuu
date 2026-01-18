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

#[derive(Debug)]
pub struct DecodeError {
    message: String,
}

impl fmt::Display for DecodeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for DecodeError {}

#[derive(Debug, Clone)]
enum TextInstr {
    Arg,
    ConstInt(i64),
    ConstBool(bool),
    ConstString(String),
    ConstUnit,
    Load(String),
    Store(String),
    Pop,
    Call {
        name: String,
        argc: Option<usize>,
        builtin: bool,
    },
    Return,
}

#[derive(Debug, Clone)]
struct TextFunction {
    name: String,
    params: Vec<String>,
    instrs: Vec<TextInstr>,
}

pub fn parse_text_module(source: &str) -> Result<BytecodeModule, DecodeError> {
    let mut functions = Vec::new();
    let mut current: Option<TextFunction> = None;

    for (line_no, raw) in source.lines().enumerate() {
        let line = raw.trim();
        if line.is_empty() {
            continue;
        }

        if let Some(rest) = line.strip_prefix("fn ") {
            if current.is_some() {
                return Err(DecodeError {
                    message: format!("line {}: nested function header", line_no + 1),
                });
            }
            let mut parts = rest.split_whitespace();
            let name = parts.next().ok_or_else(|| DecodeError {
                message: format!("line {}: missing function name", line_no + 1),
            })?;
            let extras: Vec<&str> = parts.collect();
            if extras.len() > 2 {
                return Err(DecodeError {
                    message: format!("line {}: too many fields in function header", line_no + 1),
                });
            }
            for extra in extras {
                if extra.parse::<usize>().is_err() {
                    return Err(DecodeError {
                        message: format!("line {}: invalid numeric field in header", line_no + 1),
                    });
                }
            }
            current = Some(TextFunction {
                name: name.to_string(),
                params: Vec::new(),
                instrs: Vec::new(),
            });
            continue;
        }

        if line == "end" {
            let builder = current.take().ok_or_else(|| DecodeError {
                message: format!("line {}: end without function header", line_no + 1),
            })?;
            functions.push(builder);
            continue;
        }

        let builder = current.as_mut().ok_or_else(|| DecodeError {
            message: format!("line {}: instruction outside of function", line_no + 1),
        })?;
        if let Some(rest) = line.strip_prefix("param ") {
            let name = rest.trim();
            if name.is_empty() {
                return Err(DecodeError {
                    message: format!("line {}: empty param name", line_no + 1),
                });
            }
            builder.params.push(name.to_string());
            continue;
        }
        let instr = decode_text_instruction(line, line_no + 1)?;
        builder.instrs.push(instr);
    }

    if current.is_some() {
        return Err(DecodeError {
            message: "unterminated function at end of bytecode".to_string(),
        });
    }
    if functions.is_empty() {
        return Err(DecodeError {
            message: "bytecode text contained no functions".to_string(),
        });
    }

    let mut name_to_index = HashMap::new();
    for (index, func) in functions.iter().enumerate() {
        if name_to_index.contains_key(&func.name) {
            return Err(DecodeError {
                message: format!("duplicate function '{}'", func.name),
            });
        }
        name_to_index.insert(func.name.clone(), index);
    }

    let mut resolved = Vec::with_capacity(functions.len());
    for func in functions {
        resolved.push(resolve_text_function(func, &name_to_index)?);
    }

    Ok(BytecodeModule {
        functions: resolved,
        name_to_index,
    })
}

fn resolve_text_function(
    func: TextFunction,
    name_to_index: &HashMap<String, usize>,
) -> Result<Function, DecodeError> {
    let mut locals = HashMap::new();
    for (index, name) in func.params.iter().enumerate() {
        if locals.insert(name.clone(), index as u32).is_some() {
            return Err(DecodeError {
                message: format!("duplicate param '{}' in function '{}'", name, func.name),
            });
        }
    }
    let mut next_local = locals.len() as u32;
    let mut code = Vec::new();
    let mut pending_args = 0usize;

    for instr in func.instrs {
        match instr {
            TextInstr::Arg => {
                pending_args += 1;
            }
            TextInstr::Call {
                name,
                argc,
                builtin,
            } => {
                let argc = match argc {
                    Some(value) => {
                        if pending_args > 0 && pending_args != value {
                            return Err(DecodeError {
                                message: format!(
                                    "call arg mismatch in '{}': expected {value}, saw {pending_args}",
                                    func.name
                                ),
                            });
                        }
                        value
                    }
                    None => pending_args,
                };
                pending_args = 0;
                if builtin {
                    code.push(Instr::CallBuiltin { name, argc });
                } else {
                    let func_index = *name_to_index.get(&name).ok_or_else(|| DecodeError {
                        message: format!("unknown function '{name}'"),
                    })?;
                    code.push(Instr::Call {
                        func: func_index,
                        argc,
                    });
                }
            }
            TextInstr::ConstInt(value) => {
                code.push(Instr::ConstInt(value));
            }
            TextInstr::ConstBool(value) => {
                code.push(Instr::ConstBool(value));
            }
            TextInstr::ConstString(value) => {
                code.push(Instr::ConstString(value));
            }
            TextInstr::ConstUnit => {
                code.push(Instr::ConstUnit);
            }
            TextInstr::Load(name) => {
                let index = locals.get(&name).ok_or_else(|| DecodeError {
                    message: format!("unknown local '{name}' in '{}'", func.name),
                })?;
                code.push(Instr::LoadLocal(*index));
            }
            TextInstr::Store(name) => {
                let index = if let Some(index) = locals.get(&name) {
                    *index
                } else {
                    let index = next_local;
                    locals.insert(name, index);
                    next_local += 1;
                    index
                };
                code.push(Instr::StoreLocal(index));
            }
            TextInstr::Pop => {
                code.push(Instr::Pop);
            }
            TextInstr::Return => {
                if pending_args > 0 {
                    return Err(DecodeError {
                        message: format!("dangling arg marker in '{}'", func.name),
                    });
                }
                code.push(Instr::Return);
            }
        }
    }

    if pending_args > 0 {
        return Err(DecodeError {
            message: format!("dangling arg marker in '{}'", func.name),
        });
    }

    Ok(Function {
        name: func.name,
        params: func.params.len(),
        locals: locals.len(),
        code,
    })
}

fn decode_text_instruction(line: &str, line_no: usize) -> Result<TextInstr, DecodeError> {
    if let Some(rest) = line.strip_prefix("const_int ") {
        let value = rest.trim().parse::<i64>().map_err(|_| DecodeError {
            message: format!("line {}: invalid const_int", line_no),
        })?;
        return Ok(TextInstr::ConstInt(value));
    }
    if let Some(rest) = line.strip_prefix("const_bool ") {
        return match rest.trim() {
            "true" => Ok(TextInstr::ConstBool(true)),
            "false" => Ok(TextInstr::ConstBool(false)),
            _ => Err(DecodeError {
                message: format!("line {}: invalid const_bool", line_no),
            }),
        };
    }
    if let Some(rest) = line.strip_prefix("const_string ") {
        let value = decode_text_string(rest).map_err(|err| DecodeError {
            message: format!("line {}: {err}", line_no),
        })?;
        return Ok(TextInstr::ConstString(value));
    }
    if line == "const_unit" {
        return Ok(TextInstr::ConstUnit);
    }
    if let Some(rest) = line.strip_prefix("load ") {
        let name = rest.trim();
        if name.is_empty() {
            return Err(DecodeError {
                message: format!("line {}: empty load target", line_no),
            });
        }
        return Ok(TextInstr::Load(name.to_string()));
    }
    if let Some(rest) = line.strip_prefix("store ") {
        let name = rest.trim();
        if name.is_empty() {
            return Err(DecodeError {
                message: format!("line {}: empty store target", line_no),
            });
        }
        return Ok(TextInstr::Store(name.to_string()));
    }
    if line == "pop" {
        return Ok(TextInstr::Pop);
    }
    if line == "arg" {
        return Ok(TextInstr::Arg);
    }
    if let Some(rest) = line.strip_prefix("call_builtin ") {
        return decode_call(rest, line_no, true);
    }
    if let Some(rest) = line.strip_prefix("call ") {
        return decode_call(rest, line_no, false);
    }
    if line == "return" {
        return Ok(TextInstr::Return);
    }
    Err(DecodeError {
        message: format!("line {}: unknown instruction '{line}'", line_no),
    })
}

fn decode_call(rest: &str, line_no: usize, builtin: bool) -> Result<TextInstr, DecodeError> {
    let mut parts = rest.split_whitespace();
    let name = parts.next().ok_or_else(|| DecodeError {
        message: format!("line {}: missing call target", line_no),
    })?;
    let argc = match parts.next() {
        Some(value) => Some(value.parse::<usize>().map_err(|_| DecodeError {
            message: format!("line {}: invalid call arg count", line_no),
        })?),
        None => None,
    };
    if parts.next().is_some() {
        return Err(DecodeError {
            message: format!("line {}: too many call fields", line_no),
        });
    }
    Ok(TextInstr::Call {
        name: name.to_string(),
        argc,
        builtin,
    })
}

fn decode_text_string(input: &str) -> Result<String, &'static str> {
    let mut out = String::new();
    let mut chars = input.chars();
    while let Some(ch) = chars.next() {
        if ch != '\\' {
            out.push(ch);
            continue;
        }
        let escaped = chars.next().ok_or("dangling escape")?;
        match escaped {
            'n' => out.push('\n'),
            'r' => out.push('\r'),
            't' => out.push('\t'),
            '\\' => out.push('\\'),
            '"' => out.push('"'),
            _ => return Err("unsupported escape"),
        }
    }
    Ok(out)
}

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
