use crate::ast::{
    Block, Contract, EffectsDecl, Expr, FnDecl, Item, Module, Param, Stmt, TypeRef, WorkflowDecl,
};
use crate::error::ParseError;
use crate::parser::parse_module;

pub fn format_module(module: &Module) -> String {
    let mut formatter = Formatter::new();
    formatter.format_module(module);
    formatter.finish()
}

pub fn format_source(source: &str) -> Result<String, ParseError> {
    let module = parse_module(source)?;
    Ok(format_module(&module))
}

pub fn format_source_bytes(input: &[u8]) -> Result<String, ParseError> {
    let source = std::str::from_utf8(input).map_err(|_| ParseError::new("invalid utf-8"))?;
    format_source(source)
}

struct Formatter {
    out: String,
    indent: usize,
}

impl Formatter {
    fn new() -> Self {
        Self {
            out: String::new(),
            indent: 0,
        }
    }

    fn finish(self) -> String {
        self.out
    }

    fn format_module(&mut self, module: &Module) {
        for (index, item) in module.items.iter().enumerate() {
            self.format_item(item);
            if index + 1 < module.items.len() {
                self.out.push('\n');
            }
        }
    }

    fn format_item(&mut self, item: &Item) {
        match item {
            Item::Fn(func) => self.format_fn_like("fn", func),
            Item::Workflow(flow) => self.format_workflow(flow),
        }
    }

    fn format_fn_like(&mut self, keyword: &str, func: &FnDecl) {
        let signature = format!(
            "{keyword} {}({}){}",
            func.name,
            format_params(&func.params),
            format_return_type(&func.return_type)
        );
        self.write_line(&signature);
        self.format_effects_and_contracts(func.effects.as_ref(), &func.contracts);
        self.format_block(&func.body);
    }

    fn format_workflow(&mut self, flow: &WorkflowDecl) {
        let signature = format!(
            "workflow {}({}){}",
            flow.name,
            format_params(&flow.params),
            format_return_type(&flow.return_type)
        );
        self.write_line(&signature);

        self.format_effects_and_contracts(flow.effects.as_ref(), &flow.contracts);
        self.format_block(&flow.body);
    }

    fn format_effects_and_contracts(
        &mut self,
        effects: Option<&EffectsDecl>,
        contracts: &[Contract],
    ) {
        if let Some(effects) = effects {
            self.write_line(&format_effects_decl(effects));
        }
        for contract in contracts {
            self.write_line(&format_contract(contract));
        }
    }

    fn format_block(&mut self, block: &Block) {
        self.write_line("{");
        self.indent += 1;
        for stmt in &block.stmts {
            self.format_stmt(stmt);
        }
        self.indent = self.indent.saturating_sub(1);
        self.write_line("}");
    }

    fn format_stmt(&mut self, stmt: &Stmt) {
        match stmt {
            Stmt::Let(let_stmt) => {
                let mut line = format!("let {}", let_stmt.name);
                if let Some(ty) = &let_stmt.ty {
                    line.push_str(": ");
                    line.push_str(&format_type(ty));
                }
                line.push_str(" = ");
                line.push_str(&format_expr(&let_stmt.expr));
                line.push(';');
                self.write_line(&line);
            }
            Stmt::Return(ret) => {
                let mut line = String::from("return");
                if let Some(expr) = &ret.expr {
                    line.push(' ');
                    line.push_str(&format_expr(expr));
                }
                line.push(';');
                self.write_line(&line);
            }
            Stmt::Expr(expr) => {
                let line = format!("{};", format_expr(expr));
                self.write_line(&line);
            }
            Stmt::If(if_stmt) => {
                let header = format!("if {}", format_expr(&if_stmt.condition));
                self.write_line(&header);
                self.format_block(&if_stmt.then_block);
                if let Some(else_block) = &if_stmt.else_block {
                    self.write_line("else");
                    self.format_block(else_block);
                }
            }
            Stmt::Loop(loop_stmt) => {
                self.write_line("loop");
                self.format_block(&loop_stmt.body);
            }
            Stmt::Step(step_stmt) => {
                let header = format!("step {}", format_string_literal(&step_stmt.label));
                self.write_line(&header);
                self.format_block(&step_stmt.body);
            }
        }
    }

    fn write_line(&mut self, line: &str) {
        for _ in 0..self.indent {
            self.out.push_str("    ");
        }
        self.out.push_str(line);
        self.out.push('\n');
    }
}

fn format_params(params: &[Param]) -> String {
    let mut parts = Vec::with_capacity(params.len());
    for param in params {
        if let Some(ty) = &param.ty {
            parts.push(format!("{}: {}", param.name, format_type(ty)));
        } else {
            parts.push(param.name.clone());
        }
    }
    parts.join(", ")
}

fn format_return_type(return_type: &Option<TypeRef>) -> String {
    match return_type {
        Some(ty) => format!(" -> {}", format_type(ty)),
        None => String::new(),
    }
}

fn format_effects_decl(effects: &EffectsDecl) -> String {
    match effects {
        EffectsDecl::Effects(paths) => {
            if paths.is_empty() {
                "effects {}".to_string()
            } else {
                format!("effects {{ {} }}", format_paths(paths))
            }
        }
        EffectsDecl::Requires(pairs) => {
            if pairs.is_empty() {
                "requires {}".to_string()
            } else {
                let list = pairs
                    .iter()
                    .map(|(left, right)| format!("{left}:{right}"))
                    .collect::<Vec<_>>()
                    .join(", ");
                format!("requires {{ {list} }}")
            }
        }
    }
}

fn format_contract(contract: &Contract) -> String {
    let prefix = match contract.kind {
        crate::ast::ContractKind::Pre => "pre",
        crate::ast::ContractKind::Post => "post",
        crate::ast::ContractKind::Invariant => "invariant",
    };
    format!("{prefix}: {}", format_expr(&contract.expr))
}

fn format_type(ty: &TypeRef) -> String {
    match ty {
        TypeRef::Path(path) => format_path(path),
    }
}

fn format_paths(paths: &[crate::ast::Path]) -> String {
    paths.iter().map(format_path).collect::<Vec<_>>().join(", ")
}

fn format_path(path: &crate::ast::Path) -> String {
    path.join(".")
}

fn format_expr(expr: &Expr) -> String {
    match expr {
        Expr::Ident(name) => name.clone(),
        Expr::Path(path) => format_path(path),
        Expr::String(value) => format_string_literal(value),
    }
}

fn format_string_literal(value: &str) -> String {
    format!("\"{value}\"")
}
