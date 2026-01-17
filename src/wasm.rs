use std::collections::HashMap;
use std::fmt;

use wasm_encoder::{
    BlockType, CodeSection, ExportKind, ExportSection, Function, FunctionSection, Instruction,
    Module, TypeSection, ValType,
};
use wasmi::{Engine, Instance, Linker, Module as WasmiModule, Store, Val};

use crate::ast::Module as AstModule;
use crate::interpreter::Value;
use crate::ir::{FunctionIr, Instr, ModuleIr, Type, lower_module};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WasmError {
    message: String,
}

impl fmt::Display for WasmError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for WasmError {}

impl From<crate::ir::LowerError> for WasmError {
    fn from(err: crate::ir::LowerError) -> Self {
        Self {
            message: err.to_string(),
        }
    }
}

#[derive(Debug, Default)]
pub struct HostAbi;

impl HostAbi {
    pub fn link(&self, _linker: &mut Linker<()>) -> Result<(), WasmError> {
        Ok(())
    }
}

pub fn emit_wasm(module: &AstModule) -> Result<Vec<u8>, WasmError> {
    let ir_module = lower_module(module)?;
    encode_module(&ir_module)
}

pub fn run_entry(module: &AstModule, entry: &str) -> Result<Value, WasmError> {
    let ir_module = lower_module(module)?;
    let wasm = encode_module(&ir_module)?;
    let signature = ir_module
        .functions
        .iter()
        .find(|func| func.name == entry)
        .ok_or_else(|| WasmError {
            message: format!("wasm backend entry '{entry}' not found"),
        })?
        .clone();

    if !signature.params.is_empty() {
        return Err(WasmError {
            message: format!(
                "wasm backend entry '{entry}' expects {} args",
                signature.params.len()
            ),
        });
    }

    let engine = Engine::default();
    let module = WasmiModule::new(&engine, &wasm).map_err(|err| WasmError {
        message: format!("wasm backend module error: {err}"),
    })?;
    let mut store = Store::new(&engine, ());
    let mut linker = Linker::new(&engine);
    HostAbi.link(&mut linker)?;

    let instance = linker
        .instantiate_and_start(&mut store, &module)
        .map_err(|err| WasmError {
            message: format!("wasm backend instantiation error: {err}"),
        })?;

    call_entry(&mut store, &instance, &signature, entry)
}

fn call_entry(
    store: &mut Store<()>,
    instance: &Instance,
    signature: &FunctionIr,
    entry: &str,
) -> Result<Value, WasmError> {
    let func = instance
        .get_func(&mut *store, entry)
        .ok_or_else(|| WasmError {
            message: format!("wasm backend entry '{entry}' not exported"),
        })?;

    let results = match signature.result {
        Type::Unit => Vec::new(),
        Type::I64 => vec![Val::I64(0)],
        Type::I32 => vec![Val::I32(0)],
    };
    let mut results = results;
    func.call(&mut *store, &[], &mut results)
        .map_err(|err| WasmError {
            message: format!("wasm backend call error: {err}"),
        })?;

    match signature.result {
        Type::Unit => Ok(Value::Unit),
        Type::I64 => match results.first() {
            Some(Val::I64(value)) => Ok(Value::Int(*value)),
            other => Err(WasmError {
                message: format!("wasm backend expected i64 result but got {other:?}"),
            }),
        },
        Type::I32 => match results.first() {
            Some(Val::I32(value)) => Ok(Value::Bool(*value != 0)),
            other => Err(WasmError {
                message: format!("wasm backend expected i32 result but got {other:?}"),
            }),
        },
    }
}

fn encode_module(ir_module: &ModuleIr) -> Result<Vec<u8>, WasmError> {
    let mut module = Module::new();
    let mut types = TypeSection::new();
    let mut functions = FunctionSection::new();
    let mut exports = ExportSection::new();
    let mut code = CodeSection::new();

    let mut func_indices = HashMap::new();
    for (index, func) in ir_module.functions.iter().enumerate() {
        let params = func
            .params
            .iter()
            .map(map_val_type)
            .collect::<Result<Vec<_>, _>>()?;
        let results = match func.result {
            Type::Unit => Vec::new(),
            _ => vec![map_val_type(&func.result)?],
        };
        let type_index = index as u32;
        types.ty().function(params, results);
        functions.function(type_index);
        let func_index = index as u32;
        func_indices.insert(func.name.clone(), func_index);
    }

    for func in &ir_module.functions {
        let func_index = func_indices.get(&func.name).ok_or_else(|| WasmError {
            message: format!("wasm backend missing function index for '{}'", func.name),
        })?;
        exports.export(&func.name, ExportKind::Func, *func_index);
    }

    for func in &ir_module.functions {
        let locals = group_locals(&func.locals)?;
        let mut body = Function::new(locals);
        emit_instrs(&func.body, &mut body, &func_indices)?;
        body.instruction(&Instruction::End);
        code.function(&body);
    }

    module.section(&types);
    module.section(&functions);
    module.section(&exports);
    module.section(&code);

    Ok(module.finish())
}

fn map_val_type(ty: &Type) -> Result<ValType, WasmError> {
    match ty {
        Type::I64 => Ok(ValType::I64),
        Type::I32 => Ok(ValType::I32),
        Type::Unit => Err(WasmError {
            message: "wasm backend does not support Unit in locals or params".to_string(),
        }),
    }
}

fn group_locals(locals: &[Type]) -> Result<Vec<(u32, ValType)>, WasmError> {
    let mut grouped = Vec::new();
    let mut current: Option<(ValType, u32)> = None;

    for ty in locals {
        let val = map_val_type(ty)?;
        match current {
            Some((current_ty, count)) if current_ty == val => {
                current = Some((current_ty, count + 1));
            }
            Some((current_ty, count)) => {
                grouped.push((count, current_ty));
                current = Some((val, 1));
            }
            None => current = Some((val, 1)),
        }
    }

    if let Some((ty, count)) = current {
        grouped.push((count, ty));
    }

    Ok(grouped)
}

fn emit_instrs(
    instrs: &[Instr],
    body: &mut Function,
    func_indices: &HashMap<String, u32>,
) -> Result<(), WasmError> {
    for instr in instrs {
        match instr {
            Instr::ConstI64(value) => {
                body.instruction(&Instruction::I64Const(*value));
            }
            Instr::ConstI32(value) => {
                body.instruction(&Instruction::I32Const(*value));
            }
            Instr::LocalGet(index) => {
                body.instruction(&Instruction::LocalGet(*index));
            }
            Instr::LocalSet(index) => {
                body.instruction(&Instruction::LocalSet(*index));
            }
            Instr::Call(name) => {
                let index = func_indices.get(name).ok_or_else(|| WasmError {
                    message: format!("wasm backend unknown function '{name}'"),
                })?;
                body.instruction(&Instruction::Call(*index));
            }
            Instr::If {
                condition,
                then_body,
                else_body,
            } => {
                emit_instrs(condition, body, func_indices)?;
                body.instruction(&Instruction::If(BlockType::Empty));
                emit_instrs(then_body, body, func_indices)?;
                if !else_body.is_empty() {
                    body.instruction(&Instruction::Else);
                    emit_instrs(else_body, body, func_indices)?;
                }
                body.instruction(&Instruction::End);
            }
            Instr::Return => {
                body.instruction(&Instruction::Return);
            }
            Instr::Drop => {
                body.instruction(&Instruction::Drop);
            }
            Instr::Unreachable => {
                body.instruction(&Instruction::Unreachable);
            }
        }
    }
    Ok(())
}
