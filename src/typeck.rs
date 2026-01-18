use std::collections::HashMap;
use std::fmt;

use crate::ast::{Block, Contract, Expr, Item, Module, Stmt, TypeRef};

#[derive(Debug, Clone, PartialEq, Eq)]
enum Type {
    Unit,
    Path(Vec<String>),
}

impl Type {
    fn from_ref(ty: &TypeRef) -> Self {
        match ty {
            TypeRef::Path(path) => {
                if path.len() == 1 && path[0] == "Unit" {
                    Type::Unit
                } else {
                    Type::Path(path.clone())
                }
            }
        }
    }

    fn int() -> Self {
        Type::Path(vec!["Int".to_string()])
    }

    fn bool() -> Self {
        Type::Path(vec!["Bool".to_string()])
    }

    fn string() -> Self {
        Type::Path(vec!["String".to_string()])
    }
}

impl fmt::Display for Type {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Type::Unit => write!(f, "Unit"),
            Type::Path(path) => write!(f, "{}", path.join(".")),
        }
    }
}

#[derive(Debug, Clone)]
struct Signature {
    params: Vec<Type>,
    return_type: Type,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TypeError {
    message: String,
}

impl fmt::Display for TypeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for TypeError {}

pub fn check_module(module: &Module) -> Result<(), TypeError> {
    let mut signatures = HashMap::new();
    insert_builtin_signatures(&mut signatures)?;
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

    let checker = TypeChecker { signatures };
    for item in &module.items {
        checker.check_item(item)?;
    }
    Ok(())
}

pub fn intrinsic_names() -> Vec<String> {
    vec![
        "__str_eq",
        "__str_is_empty",
        "__str_concat",
        "__str_head",
        "__str_tail",
        "__str_starts_with",
        "__str_strip_prefix",
        "__str_take_whitespace",
        "__str_take_ident",
        "__str_take_number",
        "__str_take_string_literal",
        "__str_take_line_comment",
        "__str_take_block_comment",
        "__str_is_ident_start",
        "__str_is_digit",
        "__str_is_ascii",
        "__pair_left",
        "__pair_right",
        "__lex_tokens",
        "__lex_tokens_spanned",
        "__ast_escape",
        "__ast_unescape",
        "__ast_left",
        "__ast_right",
    ]
    .into_iter()
    .map(str::to_string)
    .collect()
}

fn insert_builtin_signatures(signatures: &mut HashMap<String, Signature>) -> Result<(), TypeError> {
    let string = Type::string();
    let bool_ty = Type::bool();

    signatures.insert(
        "__str_eq".to_string(),
        Signature {
            params: vec![string.clone(), string.clone()],
            return_type: bool_ty.clone(),
        },
    );
    signatures.insert(
        "__str_is_empty".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: bool_ty.clone(),
        },
    );
    signatures.insert(
        "__str_concat".to_string(),
        Signature {
            params: vec![string.clone(), string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__str_head".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__str_tail".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__str_starts_with".to_string(),
        Signature {
            params: vec![string.clone(), string.clone()],
            return_type: bool_ty.clone(),
        },
    );
    signatures.insert(
        "__str_strip_prefix".to_string(),
        Signature {
            params: vec![string.clone(), string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__str_take_whitespace".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__str_take_ident".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__str_take_number".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__str_take_string_literal".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__str_take_line_comment".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__str_take_block_comment".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__str_is_ident_start".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: bool_ty.clone(),
        },
    );
    signatures.insert(
        "__str_is_digit".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: bool_ty.clone(),
        },
    );
    signatures.insert(
        "__str_is_ascii".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: bool_ty.clone(),
        },
    );
    signatures.insert(
        "__pair_left".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__pair_right".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__lex_tokens".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__lex_tokens_spanned".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__ast_escape".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__ast_unescape".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__ast_left".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    signatures.insert(
        "__ast_right".to_string(),
        Signature {
            params: vec![string.clone()],
            return_type: string.clone(),
        },
    );
    Ok(())
}

fn insert_signature(
    signatures: &mut HashMap<String, Signature>,
    name: &str,
    params: &[crate::ast::Param],
    return_type: &Option<TypeRef>,
) -> Result<(), TypeError> {
    if signatures.contains_key(name) {
        return Err(TypeError {
            message: format!("type error: duplicate item '{name}'"),
        });
    }

    let mut param_types = Vec::with_capacity(params.len());
    for param in params {
        let ty = param.ty.as_ref().ok_or_else(|| TypeError {
            message: format!("type error: parameter '{}' missing type", param.name),
        })?;
        param_types.push(Type::from_ref(ty));
    }

    let ret_type = return_type
        .as_ref()
        .map(Type::from_ref)
        .unwrap_or(Type::Unit);

    signatures.insert(
        name.to_string(),
        Signature {
            params: param_types,
            return_type: ret_type,
        },
    );
    Ok(())
}

struct TypeChecker {
    signatures: HashMap<String, Signature>,
}

impl TypeChecker {
    fn check_item(&self, item: &Item) -> Result<(), TypeError> {
        match item {
            Item::Fn(func) => {
                self.check_callable(&func.name, &func.params, &func.contracts, &func.body)
            }
            Item::Workflow(flow) => {
                self.check_callable(&flow.name, &flow.params, &flow.contracts, &flow.body)
            }
        }
    }

    fn check_callable(
        &self,
        name: &str,
        params: &[crate::ast::Param],
        contracts: &[Contract],
        body: &Block,
    ) -> Result<(), TypeError> {
        let signature = self.signatures.get(name).ok_or_else(|| TypeError {
            message: format!("type error: missing signature for '{name}'"),
        })?;

        let mut env = HashMap::new();
        for param in params {
            let ty = param.ty.as_ref().ok_or_else(|| TypeError {
                message: format!("type error: parameter '{}' missing type", param.name),
            })?;
            env.insert(param.name.clone(), Type::from_ref(ty));
        }

        for contract in contracts {
            let ty = self.check_expr(&contract.expr, &env)?;
            if ty != Type::bool() {
                return Err(TypeError {
                    message: format!("type error: contract expects Bool but got {ty}"),
                });
            }
        }

        self.check_block(body, &mut env, &signature.return_type)
    }

    fn check_block(
        &self,
        block: &Block,
        env: &mut HashMap<String, Type>,
        expected_return: &Type,
    ) -> Result<(), TypeError> {
        for stmt in &block.stmts {
            self.check_stmt(stmt, env, expected_return)?;
        }
        Ok(())
    }

    fn check_stmt(
        &self,
        stmt: &Stmt,
        env: &mut HashMap<String, Type>,
        expected_return: &Type,
    ) -> Result<(), TypeError> {
        match stmt {
            Stmt::Let(let_stmt) => {
                let expr_type = self.check_expr(&let_stmt.expr, env)?;
                let bound_type = if let Some(ty) = &let_stmt.ty {
                    let declared = Type::from_ref(ty);
                    if declared != expr_type {
                        return Err(TypeError {
                            message: format!(
                                "type error: let '{}' expects {declared} but got {expr_type}",
                                let_stmt.name
                            ),
                        });
                    }
                    declared
                } else {
                    expr_type
                };
                env.insert(let_stmt.name.clone(), bound_type);
                Ok(())
            }
            Stmt::Return(ret) => {
                let expr_type = match &ret.expr {
                    Some(expr) => self.check_expr(expr, env)?,
                    None => Type::Unit,
                };
                if &expr_type != expected_return {
                    return Err(TypeError {
                        message: format!(
                            "type error: return expects {expected_return} but got {expr_type}"
                        ),
                    });
                }
                Ok(())
            }
            Stmt::Expr(expr) => {
                let _ = self.check_expr(expr, env)?;
                Ok(())
            }
            Stmt::If(if_stmt) => {
                let cond_type = self.check_expr(&if_stmt.condition, env)?;
                if cond_type != Type::bool() {
                    return Err(TypeError {
                        message: format!(
                            "type error: if condition expects Bool but got {cond_type}"
                        ),
                    });
                }
                let mut then_env = env.clone();
                self.check_block(&if_stmt.then_block, &mut then_env, expected_return)?;
                if let Some(else_block) = &if_stmt.else_block {
                    let mut else_env = env.clone();
                    self.check_block(else_block, &mut else_env, expected_return)?;
                }
                Ok(())
            }
            Stmt::Loop(loop_stmt) => {
                let mut loop_env = env.clone();
                self.check_block(&loop_stmt.body, &mut loop_env, expected_return)
            }
            Stmt::Step(step_stmt) => {
                let mut step_env = env.clone();
                self.check_block(&step_stmt.body, &mut step_env, expected_return)
            }
        }
    }

    fn check_expr(&self, expr: &Expr, env: &HashMap<String, Type>) -> Result<Type, TypeError> {
        match expr {
            Expr::Int(_) => Ok(Type::int()),
            Expr::Bool(_) => Ok(Type::bool()),
            Expr::String(_) => Ok(Type::string()),
            Expr::Ident(name) => env.get(name).cloned().ok_or_else(|| TypeError {
                message: format!("type error: unknown variable '{name}'"),
            }),
            Expr::Path(path) => {
                if path.len() == 1 {
                    let name = &path[0];
                    env.get(name).cloned().ok_or_else(|| TypeError {
                        message: format!("type error: unknown variable '{name}'"),
                    })
                } else {
                    Err(TypeError {
                        message: "type error: qualified paths are not supported in expressions"
                            .to_string(),
                    })
                }
            }
            Expr::Call { callee, args } => {
                if callee.len() != 1 {
                    return Err(TypeError {
                        message: "type error: qualified function calls are not supported"
                            .to_string(),
                    });
                }
                let name = &callee[0];
                let signature = self.signatures.get(name).ok_or_else(|| TypeError {
                    message: format!("type error: unknown function '{name}'"),
                })?;
                if signature.params.len() != args.len() {
                    return Err(TypeError {
                        message: format!(
                            "type error: function '{}' expects {} args but got {}",
                            name,
                            signature.params.len(),
                            args.len()
                        ),
                    });
                }
                for (index, (arg, expected)) in args.iter().zip(signature.params.iter()).enumerate()
                {
                    let arg_type = self.check_expr(arg, env)?;
                    if &arg_type != expected {
                        return Err(TypeError {
                            message: format!(
                                "type error: argument {} of '{}' expects {expected} but got {arg_type}",
                                index + 1,
                                name
                            ),
                        });
                    }
                }
                Ok(signature.return_type.clone())
            }
        }
    }
}
