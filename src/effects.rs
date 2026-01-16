use std::collections::{BTreeSet, HashMap};
use std::fmt;

use crate::ast::{Block, EffectsDecl, Expr, Item, Module, Stmt};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EffectError {
    message: String,
}

impl fmt::Display for EffectError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for EffectError {}

pub fn check_module(module: &Module) -> Result<(), EffectError> {
    let mut effect_map = HashMap::new();
    for item in &module.items {
        match item {
            Item::Fn(func) => {
                effect_map.insert(func.name.clone(), effects_from_decl(func.effects.as_ref()));
            }
            Item::Workflow(flow) => {
                effect_map.insert(flow.name.clone(), effects_from_decl(flow.effects.as_ref()));
            }
        }
    }

    for item in &module.items {
        match item {
            Item::Fn(func) => {
                let declared = effects_from_decl(func.effects.as_ref());
                check_block(&func.body, &func.name, &declared, &effect_map)?;
            }
            Item::Workflow(flow) => {
                let declared = effects_from_decl(flow.effects.as_ref());
                check_block(&flow.body, &flow.name, &declared, &effect_map)?;
            }
        }
    }

    Ok(())
}

fn check_block(
    block: &Block,
    caller: &str,
    declared: &BTreeSet<String>,
    effect_map: &HashMap<String, BTreeSet<String>>,
) -> Result<(), EffectError> {
    for stmt in &block.stmts {
        check_stmt(stmt, caller, declared, effect_map)?;
    }
    Ok(())
}

fn check_stmt(
    stmt: &Stmt,
    caller: &str,
    declared: &BTreeSet<String>,
    effect_map: &HashMap<String, BTreeSet<String>>,
) -> Result<(), EffectError> {
    match stmt {
        Stmt::Let(let_stmt) => check_expr(&let_stmt.expr, caller, declared, effect_map),
        Stmt::Return(ret) => {
            if let Some(expr) = &ret.expr {
                check_expr(expr, caller, declared, effect_map)?;
            }
            Ok(())
        }
        Stmt::Expr(expr) => check_expr(expr, caller, declared, effect_map),
        Stmt::If(if_stmt) => {
            check_expr(&if_stmt.condition, caller, declared, effect_map)?;
            check_block(&if_stmt.then_block, caller, declared, effect_map)?;
            if let Some(else_block) = &if_stmt.else_block {
                check_block(else_block, caller, declared, effect_map)?;
            }
            Ok(())
        }
        Stmt::Loop(loop_stmt) => check_block(&loop_stmt.body, caller, declared, effect_map),
        Stmt::Step(step_stmt) => check_block(&step_stmt.body, caller, declared, effect_map),
    }
}

fn check_expr(
    expr: &Expr,
    caller: &str,
    declared: &BTreeSet<String>,
    effect_map: &HashMap<String, BTreeSet<String>>,
) -> Result<(), EffectError> {
    match expr {
        Expr::Call { callee, args } => {
            for arg in args {
                check_expr(arg, caller, declared, effect_map)?;
            }

            if callee.len() == 1 {
                let name = &callee[0];
                if let Some(required) = effect_map.get(name)
                    && !required.is_subset(declared)
                {
                    return Err(EffectError {
                        message: format!(
                            "effect error: {caller} calls {name} requiring {} but declares {}",
                            format_effect_set(required),
                            format_effect_set(declared)
                        ),
                    });
                }
            }
            Ok(())
        }
        Expr::Ident(_) | Expr::String(_) | Expr::Path(_) => Ok(()),
    }
}

fn effects_from_decl(decl: Option<&EffectsDecl>) -> BTreeSet<String> {
    let mut set = BTreeSet::new();
    match decl {
        Some(EffectsDecl::Effects(paths)) => {
            for path in paths {
                set.insert(path.join("."));
            }
        }
        Some(EffectsDecl::Requires(pairs)) => {
            for (left, right) in pairs {
                set.insert(format!("{left}.{right}"));
            }
        }
        None => {}
    }
    set
}

fn format_effect_set(set: &BTreeSet<String>) -> String {
    if set.is_empty() {
        "{}".to_string()
    } else {
        format!(
            "{{ {} }}",
            set.iter().cloned().collect::<Vec<_>>().join(", ")
        )
    }
}
