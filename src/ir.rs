use std::collections::HashMap;
use std::fmt;

use crate::ast::{Block, Expr, IfStmt, Item, LetStmt, Module, Param, ReturnStmt, Stmt, TypeRef};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Type {
    I64,
    I32,
    Unit,
}

#[derive(Debug, Clone)]
pub struct ModuleIr {
    pub functions: Vec<FunctionIr>,
}

#[derive(Debug, Clone)]
pub struct FunctionIr {
    pub name: String,
    pub params: Vec<Type>,
    pub result: Type,
    pub locals: Vec<Type>,
    pub body: Vec<Instr>,
}

#[derive(Debug, Clone)]
pub enum Instr {
    ConstI64(i64),
    ConstI32(i32),
    LocalGet(u32),
    LocalSet(u32),
    Call(String),
    If {
        condition: Vec<Instr>,
        then_body: Vec<Instr>,
        else_body: Vec<Instr>,
    },
    Return,
    Drop,
    Unreachable,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LowerError {
    message: String,
}

impl fmt::Display for LowerError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for LowerError {}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Ty {
    Int,
    Bool,
    Unit,
}

impl fmt::Display for Ty {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Ty::Int => write!(f, "Int"),
            Ty::Bool => write!(f, "Bool"),
            Ty::Unit => write!(f, "Unit"),
        }
    }
}

#[derive(Debug, Clone)]
struct Signature {
    params: Vec<Ty>,
    result: Ty,
}

#[derive(Debug, Clone)]
struct VarInfo {
    index: u32,
    ty: Ty,
}

#[derive(Debug, Clone)]
struct Env {
    vars: HashMap<String, VarInfo>,
}

impl Env {
    fn new() -> Self {
        Self {
            vars: HashMap::new(),
        }
    }
}

pub fn lower_module(module: &Module) -> Result<ModuleIr, LowerError> {
    let signatures = collect_signatures(module)?;
    let mut functions = Vec::new();

    for item in &module.items {
        match item {
            Item::Fn(func) => {
                let signature = signatures.get(&func.name).ok_or_else(|| LowerError {
                    message: format!("wasm backend missing signature for '{}'", func.name),
                })?;
                let mut lowerer = Lowerer::new(&signatures, signature.clone());
                let function = lowerer.lower_function(&func.name, &func.params, &func.body)?;
                functions.push(function);
            }
            Item::Workflow(_) => {
                return Err(LowerError {
                    message: "wasm backend does not support workflows yet".to_string(),
                });
            }
        }
    }

    Ok(ModuleIr { functions })
}

fn collect_signatures(module: &Module) -> Result<HashMap<String, Signature>, LowerError> {
    let mut signatures = HashMap::new();
    for item in &module.items {
        match item {
            Item::Fn(func) => {
                insert_signature(&mut signatures, &func.name, &func.params, &func.return_type)?
            }
            Item::Workflow(flow) => {
                insert_signature(&mut signatures, &flow.name, &flow.params, &flow.return_type)?
            }
        }
    }
    Ok(signatures)
}

fn insert_signature(
    signatures: &mut HashMap<String, Signature>,
    name: &str,
    params: &[Param],
    return_type: &Option<TypeRef>,
) -> Result<(), LowerError> {
    if signatures.contains_key(name) {
        return Err(LowerError {
            message: format!("wasm backend duplicate item '{}'", name),
        });
    }

    let mut param_types = Vec::with_capacity(params.len());
    for param in params {
        let ty_ref = param.ty.as_ref().ok_or_else(|| LowerError {
            message: format!("wasm backend parameter '{}' missing type", param.name),
        })?;
        param_types.push(parse_ty(ty_ref)?);
    }

    let result = match return_type {
        Some(ty) => parse_ty(ty)?,
        None => Ty::Unit,
    };

    signatures.insert(
        name.to_string(),
        Signature {
            params: param_types,
            result,
        },
    );
    Ok(())
}

fn parse_ty(ty: &TypeRef) -> Result<Ty, LowerError> {
    match ty {
        TypeRef::Path(path) => {
            if path.len() == 1 {
                match path[0].as_str() {
                    "Int" => Ok(Ty::Int),
                    "Bool" => Ok(Ty::Bool),
                    "Unit" => Ok(Ty::Unit),
                    "String" => Err(LowerError {
                        message: "wasm backend does not support type \"String\"".to_string(),
                    }),
                    other => Err(LowerError {
                        message: format!("wasm backend does not support type \"{other}\""),
                    }),
                }
            } else {
                Err(LowerError {
                    message: format!("wasm backend does not support type \"{}\"", path.join(".")),
                })
            }
        }
    }
}

fn ir_type(ty: Ty) -> Type {
    match ty {
        Ty::Int => Type::I64,
        Ty::Bool => Type::I32,
        Ty::Unit => Type::Unit,
    }
}

fn block_returns(block: &Block) -> bool {
    for stmt in &block.stmts {
        if stmt_returns(stmt) {
            return true;
        }
    }
    false
}

fn stmt_returns(stmt: &Stmt) -> bool {
    match stmt {
        Stmt::Return(_) => true,
        Stmt::If(IfStmt {
            then_block,
            else_block: Some(else_block),
            ..
        }) => block_returns(then_block) && block_returns(else_block),
        Stmt::If(_) => false,
        _ => false,
    }
}

struct Lowerer<'a> {
    signatures: &'a HashMap<String, Signature>,
    signature: Signature,
    locals: Vec<Type>,
}

impl<'a> Lowerer<'a> {
    fn new(signatures: &'a HashMap<String, Signature>, signature: Signature) -> Self {
        Self {
            signatures,
            signature,
            locals: Vec::new(),
        }
    }

    fn lower_function(
        &mut self,
        name: &str,
        params: &[Param],
        body: &Block,
    ) -> Result<FunctionIr, LowerError> {
        if self.signature.result != Ty::Unit && !block_returns(body) {
            return Err(LowerError {
                message: format!("wasm backend requires explicit return for function '{name}'"),
            });
        }

        let mut env = Env::new();
        for (index, (param, param_ty)) in
            params.iter().zip(self.signature.params.iter()).enumerate()
        {
            env.vars.insert(
                param.name.clone(),
                VarInfo {
                    index: index as u32,
                    ty: *param_ty,
                },
            );
        }

        let mut body_instrs = Vec::new();
        self.lower_block(body, &mut env, &mut body_instrs)?;

        Ok(FunctionIr {
            name: name.to_string(),
            params: self.signature.params.iter().copied().map(ir_type).collect(),
            result: ir_type(self.signature.result),
            locals: self.locals.clone(),
            body: body_instrs,
        })
    }

    fn lower_block(
        &mut self,
        block: &Block,
        env: &mut Env,
        instrs: &mut Vec<Instr>,
    ) -> Result<(), LowerError> {
        for stmt in &block.stmts {
            self.lower_stmt(stmt, env, instrs)?;
        }
        Ok(())
    }

    fn lower_stmt(
        &mut self,
        stmt: &Stmt,
        env: &mut Env,
        instrs: &mut Vec<Instr>,
    ) -> Result<(), LowerError> {
        match stmt {
            Stmt::Let(let_stmt) => self.lower_let(let_stmt, env, instrs),
            Stmt::Return(ret) => self.lower_return(ret, env, instrs),
            Stmt::Expr(expr) => {
                let expr_ty = self.lower_expr(expr, env, instrs)?;
                if expr_ty != Ty::Unit {
                    instrs.push(Instr::Drop);
                }
                Ok(())
            }
            Stmt::If(if_stmt) => {
                let returns = stmt_returns(stmt);
                self.lower_if(if_stmt, env, instrs)?;
                if returns {
                    instrs.push(Instr::Unreachable);
                }
                Ok(())
            }
            Stmt::Loop(_) => Err(LowerError {
                message: "wasm backend does not support loop yet".to_string(),
            }),
            Stmt::Step(_) => Err(LowerError {
                message: "wasm backend does not support step yet".to_string(),
            }),
        }
    }

    fn lower_let(
        &mut self,
        stmt: &LetStmt,
        env: &mut Env,
        instrs: &mut Vec<Instr>,
    ) -> Result<(), LowerError> {
        let expr_ty = self.lower_expr(&stmt.expr, env, instrs)?;
        let bound_ty = if let Some(ty_ref) = &stmt.ty {
            let declared = parse_ty(ty_ref)?;
            if declared != expr_ty {
                return Err(LowerError {
                    message: format!(
                        "wasm backend let '{}' expects {declared} but got {expr_ty}",
                        stmt.name
                    ),
                });
            }
            declared
        } else {
            expr_ty
        };

        let local_index = self.alloc_local(ir_type(bound_ty));
        env.vars.insert(
            stmt.name.clone(),
            VarInfo {
                index: local_index,
                ty: bound_ty,
            },
        );
        instrs.push(Instr::LocalSet(local_index));
        Ok(())
    }

    fn lower_return(
        &mut self,
        stmt: &ReturnStmt,
        env: &mut Env,
        instrs: &mut Vec<Instr>,
    ) -> Result<(), LowerError> {
        let expr_ty = match &stmt.expr {
            Some(expr) => self.lower_expr(expr, env, instrs)?,
            None => Ty::Unit,
        };
        if expr_ty != self.signature.result {
            return Err(LowerError {
                message: format!(
                    "wasm backend return expects {} but got {expr_ty}",
                    self.signature.result
                ),
            });
        }
        instrs.push(Instr::Return);
        Ok(())
    }

    fn lower_if(
        &mut self,
        stmt: &IfStmt,
        env: &mut Env,
        instrs: &mut Vec<Instr>,
    ) -> Result<(), LowerError> {
        let mut condition_instrs = Vec::new();
        let cond_ty = self.lower_expr(&stmt.condition, env, &mut condition_instrs)?;
        if cond_ty != Ty::Bool {
            return Err(LowerError {
                message: format!("wasm backend if expects Bool but got {cond_ty}"),
            });
        }

        let mut then_env = env.clone();
        let mut then_instrs = Vec::new();
        self.lower_block(&stmt.then_block, &mut then_env, &mut then_instrs)?;

        let mut else_instrs = Vec::new();
        if let Some(else_block) = &stmt.else_block {
            let mut else_env = env.clone();
            self.lower_block(else_block, &mut else_env, &mut else_instrs)?;
        }

        instrs.push(Instr::If {
            condition: condition_instrs,
            then_body: then_instrs,
            else_body: else_instrs,
        });
        Ok(())
    }

    fn lower_expr(
        &mut self,
        expr: &Expr,
        env: &Env,
        instrs: &mut Vec<Instr>,
    ) -> Result<Ty, LowerError> {
        match expr {
            Expr::Int(value) => {
                instrs.push(Instr::ConstI64(*value));
                Ok(Ty::Int)
            }
            Expr::Bool(value) => {
                instrs.push(Instr::ConstI32(if *value { 1 } else { 0 }));
                Ok(Ty::Bool)
            }
            Expr::String(_) => Err(LowerError {
                message: "wasm backend does not support type \"String\"".to_string(),
            }),
            Expr::Ident(name) => {
                let var = env.vars.get(name).ok_or_else(|| LowerError {
                    message: format!("wasm backend unknown variable '{name}'"),
                })?;
                instrs.push(Instr::LocalGet(var.index));
                Ok(var.ty)
            }
            Expr::Path(path) => {
                if path.len() == 1 {
                    let name = &path[0];
                    let var = env.vars.get(name).ok_or_else(|| LowerError {
                        message: format!("wasm backend unknown variable '{name}'"),
                    })?;
                    instrs.push(Instr::LocalGet(var.index));
                    Ok(var.ty)
                } else {
                    Err(LowerError {
                        message: "wasm backend does not support qualified paths".to_string(),
                    })
                }
            }
            Expr::Call { callee, args } => {
                if callee.len() != 1 {
                    return Err(LowerError {
                        message: "wasm backend does not support qualified calls".to_string(),
                    });
                }
                let name = &callee[0];
                let signature = self.signatures.get(name).ok_or_else(|| LowerError {
                    message: format!("wasm backend unknown function '{name}'"),
                })?;

                if signature.params.len() != args.len() {
                    return Err(LowerError {
                        message: format!(
                            "wasm backend function '{}' expects {} args but got {}",
                            name,
                            signature.params.len(),
                            args.len()
                        ),
                    });
                }

                for (arg, expected) in args.iter().zip(signature.params.iter()) {
                    let arg_ty = self.lower_expr(arg, env, instrs)?;
                    if &arg_ty != expected {
                        return Err(LowerError {
                            message: format!(
                                "wasm backend argument expects {expected} but got {arg_ty}"
                            ),
                        });
                    }
                }

                instrs.push(Instr::Call(name.clone()));
                Ok(signature.result)
            }
        }
    }

    fn alloc_local(&mut self, ty: Type) -> u32 {
        let index = self.locals.len() as u32 + self.signature.params.len() as u32;
        self.locals.push(ty);
        index
    }
}
