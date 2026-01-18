use std::fs;
use std::path::Path;

use wuu::bytecode::parse_text_module;
use wuu::interpreter::{Value, run_entry_with_args};
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

mod selfhost_support;

const OUTPUT_SEP: &str = "\n<OUT>\n";

#[test]
fn stage1_compiler_emits_bytecode_for_return_int() {
    let parser_path = Path::new("selfhost/parser.wuu");
    assert!(parser_path.exists(), "missing selfhost/parser.wuu");
    let parser_source = selfhost_support::load_with_stdlib(parser_path);
    let parser_module = parse_module(&parser_source).expect("parse parser.wuu failed");
    check_types(&parser_module).expect("typecheck parser.wuu failed");

    let compiler_path = Path::new("selfhost/compiler.wuu");
    assert!(compiler_path.exists(), "missing selfhost/compiler.wuu");
    let compiler_source = selfhost_support::load_with_stdlib(compiler_path);
    let compiler_module = parse_module(&compiler_source).expect("parse compiler.wuu failed");
    check_types(&compiler_module).expect("typecheck compiler.wuu failed");

    let source = fs::read_to_string("tests/run/01_return_int.wuu").expect("read fixture failed");
    let stage1_value =
        run_entry_with_args(&parser_module, "parse", vec![Value::String(source.clone())])
            .expect("stage1 parse failed");
    let stage1_output = match stage1_value {
        Value::String(value) => value,
        _ => panic!("stage1 parser returned non-string value"),
    };
    let (ast, rest) = split_output(&stage1_output).expect("stage1 parse did not return pair");
    assert!(rest.is_empty(), "stage1 parser left unconsumed tokens");

    let bytecode_value =
        run_entry_with_args(&compiler_module, "compile_module", vec![Value::String(ast)])
            .expect("stage1 compile failed");
    let bytecode_text = match bytecode_value {
        Value::String(value) => value,
        _ => panic!("stage1 compiler returned non-string value"),
    };

    let bytecode = parse_text_module(&bytecode_text).expect("parse bytecode failed");
    let result = bytecode
        .run_entry("main", Vec::new())
        .expect("vm run failed");
    assert_eq!(result, Value::Int(42));
}

fn split_output(value: &str) -> Option<(String, String)> {
    value.find(OUTPUT_SEP).map(|index| {
        let left = value[..index].to_string();
        let right = value[index + OUTPUT_SEP.len()..].to_string();
        (left, right)
    })
}
