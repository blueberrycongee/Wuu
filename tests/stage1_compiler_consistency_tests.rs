use std::fs;
use std::path::Path;

use wuu::bytecode::{compile_module, parse_text_module};
use wuu::interpreter::{Value, run_entry_with_args};
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

mod selfhost_support;

const OUTPUT_SEP: &str = "\n<OUT>\n";

#[test]
fn stage1_compiler_lexer_matches_interpreter() {
    let input =
        fs::read_to_string("tests/golden/lexer/01_basic.wuu").expect("read lexer fixture failed");
    let expected = run_stage1_interpreter("selfhost/lexer.wuu", "lex", vec![input.clone()]);
    let actual = run_stage1_compiled("selfhost/lexer.wuu", "lex", vec![input]);
    assert_eq!(actual, expected);
}

#[test]
fn stage1_compiler_parser_matches_interpreter() {
    let input = fs::read_to_string("tests/golden/parse/01_empty_fn.wuu")
        .expect("read parse fixture failed");
    let expected = run_stage1_interpreter("selfhost/parser.wuu", "parse", vec![input.clone()]);
    let actual = run_stage1_compiled("selfhost/parser.wuu", "parse", vec![input]);
    assert_eq!(actual, expected);
}

#[test]
fn stage1_compiler_format_matches_interpreter() {
    let input =
        fs::read_to_string("tests/golden/fmt/01_simple_fn.wuu").expect("read fmt fixture failed");
    let expected = run_stage1_interpreter("selfhost/format.wuu", "format", vec![input.clone()]);
    let actual = run_stage1_compiled("selfhost/format.wuu", "format", vec![input]);
    assert_eq!(actual, expected);
}

fn run_stage1_compiled(path: &str, entry: &str, args: Vec<String>) -> String {
    let parser_vm = load_stage1_vm("selfhost/parser.wuu");
    let compiler_vm = load_stage1_vm("selfhost/compiler.wuu");
    let source = selfhost_support::load_with_stdlib(Path::new(path));
    let ast = run_stage1_parse_vm(&parser_vm, &source);
    let bytecode_text = run_stage1_compile_vm(&compiler_vm, ast);
    let bytecode = parse_text_module(&bytecode_text).expect("parse bytecode failed");
    let value_args = args.into_iter().map(Value::String).collect();
    let value = bytecode
        .run_entry(entry, value_args)
        .expect("vm run failed");
    match value {
        Value::String(output) => output,
        _ => panic!("stage1 compiled output was not a string"),
    }
}

fn run_stage1_interpreter(path: &str, entry: &str, args: Vec<String>) -> String {
    let source = selfhost_support::load_with_stdlib(Path::new(path));
    let module = parse_module(&source).expect("parse stage1 module failed");
    check_types(&module).expect("typecheck stage1 module failed");
    let value_args = args.into_iter().map(Value::String).collect();
    let value = run_entry_with_args(&module, entry, value_args).expect("stage1 run failed");
    match value {
        Value::String(output) => output,
        _ => panic!("stage1 output was not a string"),
    }
}

fn load_stage1_module(path: &str) -> wuu::ast::Module {
    let source = selfhost_support::load_with_stdlib(Path::new(path));
    let module = parse_module(&source).expect("parse stage1 module failed");
    check_types(&module).expect("typecheck stage1 module failed");
    module
}

fn load_stage1_vm(path: &str) -> wuu::bytecode::BytecodeModule {
    let module = load_stage1_module(path);
    compile_module(&module).expect("compile stage1 module failed")
}

fn run_stage1_parse_vm(parser_vm: &wuu::bytecode::BytecodeModule, source: &str) -> String {
    let stage1_output = match parser_vm
        .run_entry("parse", vec![Value::String(source.to_string())])
        .expect("stage1 parse failed")
    {
        Value::String(value) => value,
        _ => panic!("stage1 parser returned non-string value"),
    };
    let (ast, rest) = split_output(&stage1_output).expect("stage1 parse did not return pair");
    assert!(rest.is_empty(), "stage1 parser left unconsumed tokens");
    ast
}

fn run_stage1_compile_vm(compiler_vm: &wuu::bytecode::BytecodeModule, ast: String) -> String {
    match compiler_vm
        .run_entry("compile_module", vec![Value::String(ast)])
        .expect("stage1 compile failed")
    {
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
