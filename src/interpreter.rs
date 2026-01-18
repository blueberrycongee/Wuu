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
        "__str_is_empty" => Some(eval_str_is_empty(args)),
        "__str_concat" => Some(eval_str_concat(args)),
        "__str_head" => Some(eval_str_head(args)),
        "__str_tail" => Some(eval_str_tail(args)),
        "__str_starts_with" => Some(eval_str_starts_with(args)),
        "__str_strip_prefix" => Some(eval_str_strip_prefix(args)),
        "__str_take_whitespace" => Some(eval_str_take_whitespace(args)),
        "__str_take_ident" => Some(eval_str_take_ident(args)),
        "__str_take_number" => Some(eval_str_take_number(args)),
        "__str_take_string_literal" => Some(eval_str_take_string_literal(args)),
        "__str_take_line_comment" => Some(eval_str_take_line_comment(args)),
        "__str_take_block_comment" => Some(eval_str_take_block_comment(args)),
        "__str_is_ident_start" => Some(eval_str_is_ident_start(args)),
        "__str_is_digit" => Some(eval_str_is_digit(args)),
        "__str_is_ascii" => Some(eval_str_is_ascii(args)),
        "__pair_left" => Some(eval_pair_left(args)),
        "__pair_right" => Some(eval_pair_right(args)),
        "__lex_tokens" => Some(eval_lex_tokens(args)),
        "__lex_tokens_spanned" => Some(eval_lex_tokens_spanned(args)),
        "__ast_escape" => Some(eval_ast_escape(args)),
        "__ast_unescape" => Some(eval_ast_unescape(args)),
        "__ast_left" => Some(eval_ast_left(args)),
        "__ast_right" => Some(eval_ast_right(args)),
        _ => None,
    }
}

fn expect_arg_count(args: &[Value], expected: usize, name: &str) -> Result<(), RunError> {
    if args.len() != expected {
        return Err(RunError {
            message: format!("{name} expects {expected} args but got {}", args.len()),
        });
    }
    Ok(())
}

fn expect_string_arg<'a>(args: &'a [Value], index: usize, name: &str) -> Result<&'a str, RunError> {
    match args.get(index) {
        Some(Value::String(value)) => Ok(value),
        _ => Err(RunError {
            message: format!("{name} expects String args"),
        }),
    }
}

fn expect_single_char(s: &str, name: &str) -> Result<char, RunError> {
    let mut chars = s.chars();
    let first = chars.next().ok_or_else(|| RunError {
        message: format!("{name} expects a non-empty string"),
    })?;
    if chars.next().is_some() {
        return Err(RunError {
            message: format!("{name} expects a single-character string"),
        });
    }
    Ok(first)
}

fn eval_str_eq(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 2, "__str_eq")?;
    let left = expect_string_arg(args, 0, "__str_eq")?;
    let right = expect_string_arg(args, 1, "__str_eq")?;
    Ok(Value::Bool(left == right))
}

fn eval_str_is_empty(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_is_empty")?;
    let value = expect_string_arg(args, 0, "__str_is_empty")?;
    Ok(Value::Bool(value.is_empty()))
}

fn eval_str_concat(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 2, "__str_concat")?;
    let left = expect_string_arg(args, 0, "__str_concat")?;
    let right = expect_string_arg(args, 1, "__str_concat")?;
    Ok(Value::String(format!("{left}{right}")))
}

fn eval_str_head(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_head")?;
    let value = expect_string_arg(args, 0, "__str_head")?;
    let mut chars = value.chars();
    let head = chars.next().ok_or_else(|| RunError {
        message: "__str_head expects non-empty string".to_string(),
    })?;
    Ok(Value::String(head.to_string()))
}

fn eval_str_tail(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_tail")?;
    let value = expect_string_arg(args, 0, "__str_tail")?;
    let mut chars = value.chars();
    let _ = chars.next().ok_or_else(|| RunError {
        message: "__str_tail expects non-empty string".to_string(),
    })?;
    Ok(Value::String(chars.as_str().to_string()))
}

fn eval_str_starts_with(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 2, "__str_starts_with")?;
    let value = expect_string_arg(args, 0, "__str_starts_with")?;
    let prefix = expect_string_arg(args, 1, "__str_starts_with")?;
    Ok(Value::Bool(value.starts_with(prefix)))
}

fn eval_str_strip_prefix(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 2, "__str_strip_prefix")?;
    let value = expect_string_arg(args, 0, "__str_strip_prefix")?;
    let prefix = expect_string_arg(args, 1, "__str_strip_prefix")?;
    match value.strip_prefix(prefix) {
        Some(rest) => Ok(Value::String(rest.to_string())),
        None => Err(RunError {
            message: "__str_strip_prefix prefix mismatch".to_string(),
        }),
    }
}

fn eval_str_take_whitespace(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_take_whitespace")?;
    let value = expect_string_arg(args, 0, "__str_take_whitespace")?;
    let bytes = value.as_bytes();
    let mut end = 0usize;
    while end < bytes.len() && bytes[end].is_ascii_whitespace() {
        end += 1;
    }
    Ok(Value::String(value[..end].to_string()))
}

fn eval_str_take_ident(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_take_ident")?;
    let value = expect_string_arg(args, 0, "__str_take_ident")?;
    let bytes = value.as_bytes();
    if bytes.is_empty() || !is_ident_start(bytes[0]) {
        return Ok(Value::String(String::new()));
    }
    let mut end = 1usize;
    while end < bytes.len() && is_ident_continue(bytes[end]) {
        end += 1;
    }
    Ok(Value::String(value[..end].to_string()))
}

fn eval_str_take_number(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_take_number")?;
    let value = expect_string_arg(args, 0, "__str_take_number")?;
    let bytes = value.as_bytes();
    if bytes.is_empty() || !bytes[0].is_ascii_digit() {
        return Ok(Value::String(String::new()));
    }
    let mut end = 1usize;
    while end < bytes.len() && bytes[end].is_ascii_digit() {
        end += 1;
    }
    Ok(Value::String(value[..end].to_string()))
}

fn eval_str_take_string_literal(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_take_string_literal")?;
    let value = expect_string_arg(args, 0, "__str_take_string_literal")?;
    let bytes = value.as_bytes();
    if bytes.is_empty() || bytes[0] != b'"' {
        return Err(RunError {
            message: "__str_take_string_literal expects opening quote".to_string(),
        });
    }
    let mut i = 1usize;
    let mut closed = false;
    while i < bytes.len() {
        match bytes[i] {
            b'\\' => {
                i += 1;
                if i < bytes.len() {
                    i += 1;
                } else {
                    return Err(RunError {
                        message: "unterminated string literal".to_string(),
                    });
                }
            }
            b'"' => {
                i += 1;
                closed = true;
                break;
            }
            _ => {
                i += 1;
            }
        }
    }
    if !closed {
        return Err(RunError {
            message: "unterminated string literal".to_string(),
        });
    }
    Ok(Value::String(value[..i].to_string()))
}

fn eval_str_take_line_comment(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_take_line_comment")?;
    let value = expect_string_arg(args, 0, "__str_take_line_comment")?;
    let bytes = value.as_bytes();
    if bytes.len() < 2 || bytes[0] != b'/' || bytes[1] != b'/' {
        return Err(RunError {
            message: "__str_take_line_comment expects //".to_string(),
        });
    }
    let mut end = 2usize;
    while end < bytes.len() && bytes[end] != b'\n' {
        end += 1;
    }
    Ok(Value::String(value[..end].to_string()))
}

fn eval_str_take_block_comment(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_take_block_comment")?;
    let value = expect_string_arg(args, 0, "__str_take_block_comment")?;
    let bytes = value.as_bytes();
    if bytes.len() < 2 || bytes[0] != b'/' || bytes[1] != b'*' {
        return Err(RunError {
            message: "__str_take_block_comment expects /*".to_string(),
        });
    }
    let mut i = 2usize;
    let mut closed = false;
    while i + 1 < bytes.len() {
        if bytes[i] == b'*' && bytes[i + 1] == b'/' {
            i += 2;
            closed = true;
            break;
        }
        i += 1;
    }
    if !closed {
        return Err(RunError {
            message: "unterminated block comment".to_string(),
        });
    }
    Ok(Value::String(value[..i].to_string()))
}

fn eval_str_is_ident_start(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_is_ident_start")?;
    let value = expect_string_arg(args, 0, "__str_is_ident_start")?;
    let ch = expect_single_char(value, "__str_is_ident_start")?;
    Ok(Value::Bool(ch.is_ascii_alphabetic() || ch == '_'))
}

fn eval_str_is_digit(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_is_digit")?;
    let value = expect_string_arg(args, 0, "__str_is_digit")?;
    let ch = expect_single_char(value, "__str_is_digit")?;
    Ok(Value::Bool(ch.is_ascii_digit()))
}

fn eval_str_is_ascii(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__str_is_ascii")?;
    let value = expect_string_arg(args, 0, "__str_is_ascii")?;
    let ch = expect_single_char(value, "__str_is_ascii")?;
    Ok(Value::Bool(ch.is_ascii()))
}

fn eval_pair_left(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__pair_left")?;
    let value = expect_string_arg(args, 0, "__pair_left")?;
    let sep = "\n<SEP>\n";
    match value.find(sep) {
        Some(index) => Ok(Value::String(value[..index].to_string())),
        None => Ok(Value::String(value.to_string())),
    }
}

fn eval_pair_right(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__pair_right")?;
    let value = expect_string_arg(args, 0, "__pair_right")?;
    let sep = "\n<SEP>\n";
    match value.find(sep) {
        Some(index) => Ok(Value::String(value[index + sep.len()..].to_string())),
        None => Ok(Value::String(String::new())),
    }
}

fn eval_lex_tokens(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__lex_tokens")?;
    let source = expect_string_arg(args, 0, "__lex_tokens")?;
    let tokens = crate::lexer::lex(source).map_err(|err| RunError {
        message: format!("__lex_tokens failed: {err}"),
    })?;

    let mut lines = Vec::new();
    for token in tokens {
        match token.kind {
            crate::lexer::TokenKind::Whitespace | crate::lexer::TokenKind::Comment => continue,
            crate::lexer::TokenKind::Keyword(_) => {
                let text = token_text(source, &token);
                lines.push(format!("Keyword {text}"));
            }
            crate::lexer::TokenKind::Ident => {
                let text = token_text(source, &token);
                lines.push(format!("Ident {text}"));
            }
            crate::lexer::TokenKind::Number => {
                let text = token_text(source, &token);
                lines.push(format!("Number {text}"));
            }
            crate::lexer::TokenKind::StringLiteral => {
                let text = token_text(source, &token);
                lines.push(format!("StringLiteral {text}"));
            }
            crate::lexer::TokenKind::Punct(ch) => {
                lines.push(format!("Punct {ch}"));
            }
            crate::lexer::TokenKind::Other => {
                let text = token_text(source, &token);
                lines.push(format!("Other {text}"));
            }
        }
    }

    Ok(Value::String(lines.join("\n")))
}

fn eval_lex_tokens_spanned(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__lex_tokens_spanned")?;
    let source = expect_string_arg(args, 0, "__lex_tokens_spanned")?;
    let tokens = crate::lexer::lex(source).map_err(|err| RunError {
        message: format!("__lex_tokens_spanned failed: {err}"),
    })?;

    let mut lines = Vec::new();
    for token in tokens {
        match token.kind {
            crate::lexer::TokenKind::Whitespace | crate::lexer::TokenKind::Comment => continue,
            crate::lexer::TokenKind::Keyword(_) => {
                let text = token_text(source, &token);
                lines.push(format!(
                    "@{}:{} Keyword {text}",
                    token.span.start, token.span.end
                ));
            }
            crate::lexer::TokenKind::Ident => {
                let text = token_text(source, &token);
                lines.push(format!(
                    "@{}:{} Ident {text}",
                    token.span.start, token.span.end
                ));
            }
            crate::lexer::TokenKind::Number => {
                let text = token_text(source, &token);
                lines.push(format!(
                    "@{}:{} Number {text}",
                    token.span.start, token.span.end
                ));
            }
            crate::lexer::TokenKind::StringLiteral => {
                let text = token_text(source, &token);
                lines.push(format!(
                    "@{}:{} StringLiteral {text}",
                    token.span.start, token.span.end
                ));
            }
            crate::lexer::TokenKind::Punct(ch) => {
                lines.push(format!(
                    "@{}:{} Punct {ch}",
                    token.span.start, token.span.end
                ));
            }
            crate::lexer::TokenKind::Other => {
                let text = token_text(source, &token);
                lines.push(format!(
                    "@{}:{} Other {text}",
                    token.span.start, token.span.end
                ));
            }
        }
    }

    Ok(Value::String(lines.join("\n")))
}

fn eval_ast_escape(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__ast_escape")?;
    let value = expect_string_arg(args, 0, "__ast_escape")?;
    Ok(Value::String(ast_escape_level(value)))
}

fn eval_ast_unescape(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__ast_unescape")?;
    let value = expect_string_arg(args, 0, "__ast_unescape")?;
    Ok(Value::String(ast_unescape_level(value)))
}

fn eval_ast_left(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__ast_left")?;
    let value = expect_string_arg(args, 0, "__ast_left")?;
    match find_ast_sep(value) {
        Some((index, _level, _len)) => Ok(Value::String(value[..index].to_string())),
        None => Ok(Value::String(value.to_string())),
    }
}

fn eval_ast_right(args: &[Value]) -> Result<Value, RunError> {
    expect_arg_count(args, 1, "__ast_right")?;
    let value = expect_string_arg(args, 0, "__ast_right")?;
    match find_ast_sep(value) {
        Some((index, _level, len)) => Ok(Value::String(value[index + len..].to_string())),
        None => Ok(Value::String(String::new())),
    }
}

fn find_ast_sep(value: &str) -> Option<(usize, usize, usize)> {
    let mut best: Option<(usize, usize, usize)> = None;
    let mut i = 0usize;
    while let Some(pos) = value[i..].find("\n<AST") {
        let start = i + pos;
        if let Some((level, len)) = parse_ast_sep(&value[start..]) {
            match best {
                None => {
                    best = Some((start, level, len));
                    if level == 0 {
                        return best;
                    }
                }
                Some((best_index, best_level, _)) => {
                    if level < best_level || (level == best_level && start < best_index) {
                        best = Some((start, level, len));
                        if level == 0 {
                            return best;
                        }
                    }
                }
            }
        }
        i = start + 1;
    }
    best
}

fn parse_ast_sep(value: &str) -> Option<(usize, usize)> {
    let prefix = "\n<AST";
    if !value.starts_with(prefix) {
        return None;
    }
    let mut level = 0usize;
    let mut idx = prefix.len();
    while value[idx..].starts_with("_ESC") {
        level += 1;
        idx += "_ESC".len();
    }
    if value[idx..].starts_with(">\n") {
        Some((level, idx + ">\n".len()))
    } else {
        None
    }
}

fn ast_escape_level(value: &str) -> String {
    let mut out = String::new();
    let mut i = 0usize;
    while let Some(pos) = value[i..].find("\n<AST") {
        let start = i + pos;
        out.push_str(&value[i..start]);
        if let Some((level, len)) = parse_ast_sep(&value[start..]) {
            out.push_str("\n<AST");
            for _ in 0..(level + 1) {
                out.push_str("_ESC");
            }
            out.push_str(">\n");
            i = start + len;
        } else {
            out.push_str("\n<AST");
            i = start + "\n<AST".len();
        }
    }
    out.push_str(&value[i..]);
    out
}

fn ast_unescape_level(value: &str) -> String {
    let mut out = String::new();
    let mut i = 0usize;
    while let Some(pos) = value[i..].find("\n<AST") {
        let start = i + pos;
        out.push_str(&value[i..start]);
        if let Some((level, len)) = parse_ast_sep(&value[start..]) {
            out.push_str("\n<AST");
            let new_level = level.saturating_sub(1);
            for _ in 0..new_level {
                out.push_str("_ESC");
            }
            out.push_str(">\n");
            i = start + len;
        } else {
            out.push_str("\n<AST");
            i = start + "\n<AST".len();
        }
    }
    out.push_str(&value[i..]);
    out
}

fn token_text<'a>(source: &'a str, token: &crate::lexer::Token) -> &'a str {
    &source[token.span.start..token.span.end]
}

fn is_ident_start(b: u8) -> bool {
    b.is_ascii_alphabetic() || b == b'_'
}

fn is_ident_continue(b: u8) -> bool {
    is_ident_start(b) || b.is_ascii_digit()
}
