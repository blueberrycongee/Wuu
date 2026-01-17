use std::fmt;
use std::fs;
use std::path::{Path, PathBuf};
use std::time::Instant;

use crate::ast::{Item, Param, TypeRef};
use crate::effects::check_module as check_effects;
use crate::interpreter::{Value, run_entry, run_entry_with_args};
use crate::parser::parse_module;
use crate::typeck::check_module as check_types;
use crate::wasm;

#[derive(Debug, Clone)]
pub struct Evidence {
    pub examples: Vec<Example>,
    pub properties: Vec<Property>,
    pub benches: Vec<Bench>,
}

#[derive(Debug, Clone)]
pub struct Example {
    pub name: String,
    pub source: String,
    pub expect: Value,
    origin: Origin,
}

#[derive(Debug, Clone)]
pub struct Property {
    pub name: String,
    pub source: String,
    pub cases: Vec<PropertyCase>,
    origin: Origin,
}

#[derive(Debug, Clone)]
pub struct PropertyCase {
    pub args: Vec<Value>,
    pub expect: Value,
}

#[derive(Debug, Clone)]
pub struct Bench {
    pub name: String,
    pub source: String,
    pub iterations: usize,
    pub max_ms: u64,
    pub backend: BenchBackend,
    origin: Origin,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BenchBackend {
    Interpreter,
    Wasm,
}

#[derive(Debug, Clone)]
struct Origin {
    path: PathBuf,
    line: usize,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EvidenceError {
    message: String,
}

impl fmt::Display for EvidenceError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for EvidenceError {}

impl EvidenceError {
    fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
        }
    }

    fn with_origin(origin: &Origin, message: impl Into<String>) -> Self {
        let message = message.into();
        let path = origin.path.display();
        let line = origin.line;
        Self {
            message: format!("{path}:{line}: {message}"),
        }
    }

    fn from_io(path: &Path, err: impl fmt::Display) -> Self {
        Self {
            message: format!("{}: {err}", path.display()),
        }
    }
}

pub fn collect_evidence(dir: &Path) -> Result<Evidence, EvidenceError> {
    let mut evidence = Evidence {
        examples: Vec::new(),
        properties: Vec::new(),
        benches: Vec::new(),
    };

    let entries = fs::read_dir(dir).map_err(|err| EvidenceError::from_io(dir, err))?;
    for entry in entries {
        let entry = entry.map_err(|err| EvidenceError::from_io(dir, err))?;
        let path = entry.path();
        if path.is_dir() {
            continue;
        }
        if path.extension().and_then(|ext| ext.to_str()) != Some("md") {
            continue;
        }
        let content =
            fs::read_to_string(&path).map_err(|err| EvidenceError::from_io(&path, err))?;
        parse_file(&path, &content, &mut evidence)?;
    }

    Ok(evidence)
}

pub fn run_examples(evidence: &Evidence) -> Result<(), EvidenceError> {
    for example in &evidence.examples {
        let module = parse_module(&example.source)
            .map_err(|err| EvidenceError::with_origin(&example.origin, err.to_string()))?;
        check_types(&module)
            .map_err(|err| EvidenceError::with_origin(&example.origin, err.to_string()))?;
        check_effects(&module)
            .map_err(|err| EvidenceError::with_origin(&example.origin, err.to_string()))?;
        let value = run_entry(&module, "main")
            .map_err(|err| EvidenceError::with_origin(&example.origin, err.to_string()))?;
        if value != example.expect {
            return Err(EvidenceError::with_origin(
                &example.origin,
                format!(
                    "example '{}' expected {expected} but got {value}",
                    example.name,
                    expected = format_value(&example.expect)
                ),
            ));
        }
    }
    Ok(())
}

pub fn run_properties(evidence: &Evidence) -> Result<(), EvidenceError> {
    for property in &evidence.properties {
        let module = parse_module(&property.source)
            .map_err(|err| EvidenceError::with_origin(&property.origin, err.to_string()))?;
        check_types(&module)
            .map_err(|err| EvidenceError::with_origin(&property.origin, err.to_string()))?;
        check_effects(&module)
            .map_err(|err| EvidenceError::with_origin(&property.origin, err.to_string()))?;

        let params = find_params(&module, "main")
            .ok_or_else(|| EvidenceError::with_origin(&property.origin, "missing main function"))?;

        for case in &property.cases {
            ensure_arg_types(&property.origin, params, &case.args)?;
            let value = run_entry_with_args(&module, "main", case.args.clone())
                .map_err(|err| EvidenceError::with_origin(&property.origin, err.to_string()))?;
            if value != case.expect {
                return Err(EvidenceError::with_origin(
                    &property.origin,
                    format!(
                        "property '{}' expected {expected} but got {value}",
                        property.name,
                        expected = format_value(&case.expect)
                    ),
                ));
            }
        }
    }
    Ok(())
}

pub fn run_benches(evidence: &Evidence) -> Result<(), EvidenceError> {
    for bench in &evidence.benches {
        let module = parse_module(&bench.source)
            .map_err(|err| EvidenceError::with_origin(&bench.origin, err.to_string()))?;
        check_types(&module)
            .map_err(|err| EvidenceError::with_origin(&bench.origin, err.to_string()))?;
        check_effects(&module)
            .map_err(|err| EvidenceError::with_origin(&bench.origin, err.to_string()))?;

        if let Some(params) = find_params(&module, "main")
            && !params.is_empty()
        {
            return Err(EvidenceError::with_origin(
                &bench.origin,
                "bench main must have zero params",
            ));
        }

        let start = Instant::now();
        for _ in 0..bench.iterations {
            match bench.backend {
                BenchBackend::Interpreter => {
                    run_entry(&module, "main").map_err(|err| {
                        EvidenceError::with_origin(&bench.origin, err.to_string())
                    })?;
                }
                BenchBackend::Wasm => {
                    wasm::run_entry(&module, "main").map_err(|err| {
                        EvidenceError::with_origin(&bench.origin, err.to_string())
                    })?;
                }
            }
        }
        let elapsed = start.elapsed().as_millis() as u64;
        if elapsed > bench.max_ms {
            return Err(EvidenceError::with_origin(
                &bench.origin,
                format!(
                    "bench '{}' exceeded {max_ms}ms (took {elapsed}ms)",
                    bench.name,
                    max_ms = bench.max_ms
                ),
            ));
        }
    }
    Ok(())
}

fn parse_file(path: &Path, text: &str, evidence: &mut Evidence) -> Result<(), EvidenceError> {
    let lines: Vec<&str> = text.lines().collect();
    let mut index = 0usize;

    while index < lines.len() {
        let line = lines[index].trim();
        if let Some(name) = line.strip_prefix("example:") {
            let name = name.trim().to_string();
            if name.is_empty() {
                return Err(EvidenceError::new(format!(
                    "{}:{}: example name is required",
                    path.display(),
                    index + 1
                )));
            }
            let origin = Origin {
                path: path.to_path_buf(),
                line: index + 1,
            };
            let (source, next) = parse_code_block(path, &lines, index + 1)?;
            let (expect, next) = parse_expect_line(path, &lines, next)?;
            evidence.examples.push(Example {
                name,
                source,
                expect,
                origin,
            });
            index = next;
            continue;
        }

        if let Some(name) = line.strip_prefix("property:") {
            let name = name.trim().to_string();
            if name.is_empty() {
                return Err(EvidenceError::new(format!(
                    "{}:{}: property name is required",
                    path.display(),
                    index + 1
                )));
            }
            let origin = Origin {
                path: path.to_path_buf(),
                line: index + 1,
            };
            let (source, next) = parse_code_block(path, &lines, index + 1)?;
            let (cases, next) = parse_property_cases(path, &lines, next)?;
            evidence.properties.push(Property {
                name,
                source,
                cases,
                origin,
            });
            index = next;
            continue;
        }

        if let Some(name) = line.strip_prefix("bench:") {
            let name = name.trim().to_string();
            if name.is_empty() {
                return Err(EvidenceError::new(format!(
                    "{}:{}: bench name is required",
                    path.display(),
                    index + 1
                )));
            }
            let origin = Origin {
                path: path.to_path_buf(),
                line: index + 1,
            };
            let (source, next) = parse_code_block(path, &lines, index + 1)?;
            let (iterations, max_ms, backend, next) = parse_bench_config(path, &lines, next)?;
            evidence.benches.push(Bench {
                name,
                source,
                iterations,
                max_ms,
                backend,
                origin,
            });
            index = next;
            continue;
        }

        index += 1;
    }

    Ok(())
}

fn parse_code_block(
    path: &Path,
    lines: &[&str],
    mut index: usize,
) -> Result<(String, usize), EvidenceError> {
    let (start, fence) = next_non_empty_line(lines, index).ok_or_else(|| {
        EvidenceError::new(format!(
            "{}:{}: expected code block",
            path.display(),
            index + 1
        ))
    })?;
    let fence_trim = fence.trim();
    if !fence_trim.starts_with("```") {
        return Err(EvidenceError::new(format!(
            "{}:{}: expected code fence",
            path.display(),
            start + 1
        )));
    }
    let lang = fence_trim.trim_start_matches("```").trim();
    if lang != "wuu" {
        return Err(EvidenceError::new(format!(
            "{}:{}: expected ```wuu code fence",
            path.display(),
            start + 1
        )));
    }

    let mut end = start + 1;
    while end < lines.len() {
        if lines[end].trim() == "```" {
            break;
        }
        end += 1;
    }
    if end >= lines.len() {
        return Err(EvidenceError::new(format!(
            "{}:{}: unterminated code fence",
            path.display(),
            start + 1
        )));
    }

    let source = if end > start + 1 {
        let mut joined = lines[start + 1..end].join("\n");
        joined.push('\n');
        joined
    } else {
        String::new()
    };

    index = end + 1;
    Ok((source, index))
}

fn parse_expect_line(
    path: &Path,
    lines: &[&str],
    index: usize,
) -> Result<(Value, usize), EvidenceError> {
    let (line_index, line) = next_non_empty_line(lines, index).ok_or_else(|| {
        EvidenceError::new(format!(
            "{}:{}: expected expect line",
            path.display(),
            index + 1
        ))
    })?;
    let trimmed = line.trim();
    let expect_text = trimmed
        .strip_prefix("expect:")
        .ok_or_else(|| {
            EvidenceError::new(format!(
                "{}:{}: expected expect line",
                path.display(),
                line_index + 1
            ))
        })?
        .trim();

    let value = parse_value(path, line_index + 1, expect_text)?;
    Ok((value, line_index + 1))
}

fn parse_property_cases(
    path: &Path,
    lines: &[&str],
    mut index: usize,
) -> Result<(Vec<PropertyCase>, usize), EvidenceError> {
    let mut cases = Vec::new();

    loop {
        let Some((line_index, line)) = next_non_empty_line(lines, index) else {
            break;
        };
        let trimmed = line.trim();
        if !trimmed.starts_with("case:") {
            if cases.is_empty() {
                return Err(EvidenceError::new(format!(
                    "{}:{}: expected at least one case line",
                    path.display(),
                    line_index + 1
                )));
            }
            index = line_index;
            break;
        }

        let rest = trimmed.trim_start_matches("case:").trim();
        let (args_text, expect_text) = rest.split_once("=>").ok_or_else(|| {
            EvidenceError::new(format!(
                "{}:{}: case line must include '=>'",
                path.display(),
                line_index + 1
            ))
        })?;
        let args = parse_args(path, line_index + 1, args_text.trim())?;
        let expect = parse_value(path, line_index + 1, expect_text.trim())?;
        cases.push(PropertyCase { args, expect });
        index = line_index + 1;
    }

    if cases.is_empty() {
        return Err(EvidenceError::new(format!(
            "{}:{}: expected at least one case line",
            path.display(),
            index + 1
        )));
    }

    Ok((cases, index))
}

fn parse_bench_config(
    path: &Path,
    lines: &[&str],
    index: usize,
) -> Result<(usize, u64, BenchBackend, usize), EvidenceError> {
    let (iter_index, iter_line) = next_non_empty_line(lines, index).ok_or_else(|| {
        EvidenceError::new(format!(
            "{}:{}: expected iterations line",
            path.display(),
            index + 1
        ))
    })?;
    let iterations_text = iter_line
        .trim()
        .strip_prefix("iterations:")
        .ok_or_else(|| {
            EvidenceError::new(format!(
                "{}:{}: expected iterations line",
                path.display(),
                iter_index + 1
            ))
        })?
        .trim();
    let iterations = iterations_text.parse::<usize>().map_err(|_| {
        EvidenceError::new(format!(
            "{}:{}: invalid iterations value",
            path.display(),
            iter_index + 1
        ))
    })?;
    if iterations == 0 {
        return Err(EvidenceError::new(format!(
            "{}:{}: iterations must be >= 1",
            path.display(),
            iter_index + 1
        )));
    }

    let (max_index, max_line) = next_non_empty_line(lines, iter_index + 1).ok_or_else(|| {
        EvidenceError::new(format!(
            "{}:{}: expected max_ms line",
            path.display(),
            iter_index + 2
        ))
    })?;
    let max_text = max_line
        .trim()
        .strip_prefix("max_ms:")
        .ok_or_else(|| {
            EvidenceError::new(format!(
                "{}:{}: expected max_ms line",
                path.display(),
                max_index + 1
            ))
        })?
        .trim();
    let max_ms = max_text.parse::<u64>().map_err(|_| {
        EvidenceError::new(format!(
            "{}:{}: invalid max_ms value",
            path.display(),
            max_index + 1
        ))
    })?;

    let mut backend = BenchBackend::Interpreter;
    let Some((backend_index, backend_line)) = next_non_empty_line(lines, max_index + 1) else {
        return Ok((iterations, max_ms, backend, max_index + 1));
    };
    let trimmed = backend_line.trim();
    if let Some(rest) = trimmed.strip_prefix("backend:") {
        let value = rest.trim();
        backend = match value {
            "interpreter" => BenchBackend::Interpreter,
            "wasm" => BenchBackend::Wasm,
            _ => {
                return Err(EvidenceError::new(format!(
                    "{}:{}: unknown bench backend '{value}'",
                    path.display(),
                    backend_index + 1
                )));
            }
        };
        Ok((iterations, max_ms, backend, backend_index + 1))
    } else {
        Ok((iterations, max_ms, backend, max_index + 1))
    }
}

fn next_non_empty_line<'a>(lines: &'a [&'a str], mut index: usize) -> Option<(usize, &'a str)> {
    while index < lines.len() {
        if !lines[index].trim().is_empty() {
            return Some((index, lines[index]));
        }
        index += 1;
    }
    None
}

fn parse_args(path: &Path, line: usize, text: &str) -> Result<Vec<Value>, EvidenceError> {
    let trimmed = text.trim();
    if !trimmed.starts_with('[') || !trimmed.ends_with(']') {
        return Err(EvidenceError::new(format!(
            "{}:{}: case args must be in [..]",
            path.display(),
            line
        )));
    }
    let inner = trimmed.trim_start_matches('[').trim_end_matches(']').trim();
    if inner.is_empty() {
        return Ok(Vec::new());
    }
    inner
        .split(',')
        .map(|part| parse_value(path, line, part.trim()))
        .collect()
}

fn parse_value(path: &Path, line: usize, text: &str) -> Result<Value, EvidenceError> {
    let trimmed = text.trim();
    if trimmed.is_empty() || trimmed == "unit" {
        return Ok(Value::Unit);
    }
    if trimmed == "true" {
        return Ok(Value::Bool(true));
    }
    if trimmed == "false" {
        return Ok(Value::Bool(false));
    }
    if let Some(stripped) = trimmed.strip_prefix('"').and_then(|s| s.strip_suffix('"')) {
        return Ok(Value::String(stripped.to_string()));
    }
    if let Ok(value) = trimmed.parse::<i64>() {
        return Ok(Value::Int(value));
    }
    Err(EvidenceError::new(format!(
        "{}:{}: unsupported literal '{trimmed}'",
        path.display(),
        line
    )))
}

fn find_params<'a>(module: &'a crate::ast::Module, name: &str) -> Option<&'a [Param]> {
    for item in &module.items {
        if let Item::Fn(func) = item
            && func.name == name
        {
            return Some(&func.params);
        }
    }
    None
}

fn ensure_arg_types(
    origin: &Origin,
    params: &[Param],
    args: &[Value],
) -> Result<(), EvidenceError> {
    if params.len() != args.len() {
        return Err(EvidenceError::with_origin(
            origin,
            format!(
                "property args length {} does not match params {}",
                args.len(),
                params.len()
            ),
        ));
    }
    for (param, arg) in params.iter().zip(args.iter()) {
        let Some(param_ty) = param.ty.as_ref() else {
            continue;
        };
        if !value_matches_type(arg, param_ty) {
            return Err(EvidenceError::with_origin(
                origin,
                format!(
                    "property arg '{}' expects {} but got {}",
                    param.name,
                    format_type_ref(param_ty),
                    value_type_name(arg)
                ),
            ));
        }
    }
    Ok(())
}

fn value_matches_type(value: &Value, ty: &TypeRef) -> bool {
    match ty {
        TypeRef::Path(path) if path.len() == 1 => matches!(
            (path[0].as_str(), value),
            ("Int", Value::Int(_))
                | ("Bool", Value::Bool(_))
                | ("String", Value::String(_))
                | ("Unit", Value::Unit)
        ),
        _ => false,
    }
}

fn format_type_ref(ty: &TypeRef) -> String {
    match ty {
        TypeRef::Path(path) => path.join("."),
    }
}

fn value_type_name(value: &Value) -> &'static str {
    match value {
        Value::Int(_) => "Int",
        Value::Bool(_) => "Bool",
        Value::String(_) => "String",
        Value::Unit => "Unit",
    }
}

fn format_value(value: &Value) -> String {
    match value {
        Value::String(text) => format!("\"{text}\""),
        Value::Unit => "unit".to_string(),
        _ => value.to_string(),
    }
}
