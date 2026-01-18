use std::fs;
use std::path::Path;

use wuu::bytecode::{BytecodeModule, compile_module, parse_text_module};
use wuu::interpreter::Value;
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

mod selfhost_support;

const OUTPUT_SEP: &str = "\n<OUT>\n";

#[test]
fn stage2_compiler_matches_stage1_output() {
    let parser_vm = load_stage1_vm("selfhost/parser.wuu");
    let compiler_vm = load_stage1_vm("selfhost/compiler.wuu");

    let compiler_source = selfhost_support::load_with_stdlib(Path::new("selfhost/compiler.wuu"));
    let compiler_ast = run_stage1_parse_vm(&parser_vm, &compiler_source);
    let stage2_text = run_stage1_compile_vm(&compiler_vm, compiler_ast);
    let stage2_compiler = parse_text_module(&stage2_text).expect("parse stage2 compiler failed");

    let input = fs::read_to_string("tests/run/01_return_int.wuu").expect("read fixture failed");
    let input_ast = run_stage1_parse_vm(&parser_vm, &input);
    let stage1_text = run_stage1_compile_vm(&compiler_vm, input_ast.clone());
    let stage2_text = match stage2_compiler
        .run_entry("compile_module", vec![Value::String(input_ast)])
        .expect("stage2 compile failed")
    {
        Value::String(value) => value,
        _ => panic!("stage2 compiler returned non-string value"),
    };

    assert_eq!(stage2_text, stage1_text);
}

fn load_stage1_vm(path: &str) -> BytecodeModule {
    let source = selfhost_support::load_with_stdlib(Path::new(path));
    let module = parse_module(&source).expect("parse stage1 module failed");
    check_types(&module).expect("typecheck stage1 module failed");
    compile_module(&module).expect("compile stage1 module failed")
}

fn run_stage1_parse_vm(parser_vm: &BytecodeModule, source: &str) -> String {
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

fn run_stage1_compile_vm(compiler_vm: &BytecodeModule, ast: String) -> String {
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
