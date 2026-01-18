use std::fs;
use std::path::Path;

use wuu::bytecode::parse_text_module;
use wuu::interpreter::{Value, run_entry, run_entry_with_args};
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

mod selfhost_support;

const OUTPUT_SEP: &str = "\n<OUT>\n";

#[test]
fn stage1_compiler_emits_bytecode_for_return_int() {
    let source = fs::read_to_string("tests/run/01_return_int.wuu").expect("read fixture failed");
    let result = stage1_compile_and_run(&source);
    assert_eq!(result, Value::Int(42));
}

#[test]
fn stage1_compiler_handles_return_string() {
    let source = fs::read_to_string("tests/run/04_return_string.wuu").expect("read fixture failed");
    let expected = stage0_run(&source);
    let result = stage1_compile_and_run(&source);
    assert_eq!(result, expected);
}

#[test]
fn stage1_compiler_handles_let_and_call() {
    let source = fs::read_to_string("tests/run/03_call_and_let.wuu").expect("read fixture failed");
    let expected = stage0_run(&source);
    let result = stage1_compile_and_run(&source);
    assert_eq!(result, expected);
}

fn stage1_compile_and_run(source: &str) -> Value {
    let parser_module = load_stage1_module("selfhost/parser.wuu");
    let compiler_module = load_stage1_module("selfhost/compiler.wuu");
    let stage1_output = run_stage1_parse(&parser_module, source);
    let bytecode_text = run_stage1_compile(&compiler_module, stage1_output);
    let bytecode = parse_text_module(&bytecode_text).expect("parse bytecode failed");
    bytecode
        .run_entry("main", Vec::new())
        .expect("vm run failed")
}

fn stage0_run(source: &str) -> Value {
    let module = parse_module(source).expect("parse stage0 source failed");
    run_entry(&module, "main").expect("stage0 run failed")
}

fn load_stage1_module(path: &str) -> wuu::ast::Module {
    let source = selfhost_support::load_with_stdlib(Path::new(path));
    let module = parse_module(&source).expect("parse selfhost module failed");
    check_types(&module).expect("typecheck selfhost module failed");
    module
}

fn run_stage1_parse(parser_module: &wuu::ast::Module, source: &str) -> String {
    let stage1_value = run_entry_with_args(
        parser_module,
        "parse",
        vec![Value::String(source.to_string())],
    )
    .expect("stage1 parse failed");
    let stage1_output = match stage1_value {
        Value::String(value) => value,
        _ => panic!("stage1 parser returned non-string value"),
    };
    let (ast, rest) = split_output(&stage1_output).expect("stage1 parse did not return pair");
    assert!(rest.is_empty(), "stage1 parser left unconsumed tokens");
    ast
}

fn run_stage1_compile(compiler_module: &wuu::ast::Module, ast: String) -> String {
    let bytecode_value =
        run_entry_with_args(compiler_module, "compile_module", vec![Value::String(ast)])
            .expect("stage1 compile failed");
    match bytecode_value {
        Value::String(value) => value,
        _ => panic!("stage1 compiler returned non-string value"),
    }
}

fn split_output(value: &str) -> Option<(String, String)> {
    value.find(OUTPUT_SEP).map(|index| {
        let left = value[..index].to_string();
        let right = value[index + OUTPUT_SEP.len()..].to_string();
        (left, right)
    })
}
