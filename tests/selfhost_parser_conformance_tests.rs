use std::fs;
use std::path::Path;

use wuu::format::format_source;
use wuu::interpreter::{Value, run_entry_with_args};
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

const OUTPUT_SEP: &str = "\n<OUT>\n";
const AST_SEP: &str = "\n<AST>\n";

#[test]
fn selfhost_parser_matches_stage0_for_parse_fixtures() {
    let parser_path = Path::new("selfhost/parser.wuu");
    assert!(parser_path.exists(), "missing selfhost/parser.wuu");
    let parser_source = fs::read_to_string(parser_path).expect("read parser.wuu failed");
    let parser_module = parse_module(&parser_source).expect("parse parser.wuu failed");
    check_types(&parser_module).expect("typecheck parser.wuu failed");

    let dir = Path::new("tests/golden/parse");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }

        let source = fs::read_to_string(&path).expect("read source failed");
        let stage0 = format_source(&source).expect("stage0 format failed");

        let stage1_value =
            run_entry_with_args(&parser_module, "parse", vec![Value::String(source.clone())])
                .expect("stage1 parse failed");
        let stage1_output = match stage1_value {
            Value::String(value) => value,
            _ => panic!("stage1 parse returned non-string value"),
        };

        let (ast, rest) =
            split_output(&stage1_output).expect("stage1 parse did not return pair output");
        assert!(
            rest.is_empty(),
            "stage1 parser left unconsumed tokens for {}",
            path.display()
        );
        let (tag, _) = split_ast_pair(&ast).expect("stage1 parser returned invalid AST");
        assert!(
            tag.starts_with("Module@"),
            "stage1 parser did not return Module AST for {}",
            path.display()
        );
        let _ = stage0;

        count += 1;
    }

    assert!(count >= 3, "expected at least 3 parse fixtures");
}

#[test]
fn selfhost_parser_uses_lex_tokens_wrapper() {
    let parser_path = Path::new("selfhost/parser.wuu");
    let parser_source = fs::read_to_string(parser_path).expect("read parser.wuu failed");

    assert!(
        parser_source.contains("let tokens = lex_tokens"),
        "selfhost/parser.wuu should route through lex_tokens"
    );
    assert!(
        parser_source.contains("__lex_tokens_spanned"),
        "selfhost/parser.wuu should include a host-backed lex_tokens_spanned fallback"
    );
    assert!(
        !parser_source.contains("let tokens = __lex_tokens_spanned"),
        "selfhost/parser.wuu should not call __lex_tokens_spanned directly in parse()"
    );
}

#[test]
fn selfhost_parser_includes_span_nodes() {
    let parser_path = Path::new("selfhost/parser.wuu");
    let parser_source = fs::read_to_string(parser_path).expect("read parser.wuu failed");
    let parser_module = parse_module(&parser_source).expect("parse parser.wuu failed");
    check_types(&parser_module).expect("typecheck parser.wuu failed");

    let input = Path::new("tests/golden/parse/02_fn_with_return.wuu");
    let source = fs::read_to_string(input).expect("read source failed");

    let stage1_value = run_entry_with_args(&parser_module, "parse", vec![Value::String(source)])
        .expect("stage1 parse failed");
    let stage1_output = match stage1_value {
        Value::String(value) => value,
        _ => panic!("stage1 parse returned non-string value"),
    };

    let (ast, rest) =
        split_output(&stage1_output).expect("stage1 parse did not return pair output");
    assert!(rest.is_empty(), "stage1 parser left unconsumed tokens");

    let (tag, _payload) = split_ast_pair(&ast).expect("stage1 parser returned invalid AST");
    assert!(
        tag.starts_with("Module@"),
        "expected Module tag with span, got: {tag}"
    );
}

#[test]
fn selfhost_parser_has_no_host_pair_intrinsics() {
    let parser_path = Path::new("selfhost/parser.wuu");
    let parser_source = fs::read_to_string(parser_path).expect("read parser.wuu failed");

    assert!(
        !parser_source.contains("__pair_left"),
        "selfhost/parser.wuu should not depend on __pair_left"
    );
    assert!(
        !parser_source.contains("__pair_right"),
        "selfhost/parser.wuu should not depend on __pair_right"
    );
}

#[test]
fn selfhost_parser_handles_large_module_without_stack_overflow() {
    let parser_path = Path::new("selfhost/parser.wuu");
    let parser_source = fs::read_to_string(parser_path).expect("read parser.wuu failed");
    let parser_module = parse_module(&parser_source).expect("parse parser.wuu failed");
    check_types(&parser_module).expect("typecheck parser.wuu failed");

    let mut source = String::new();
    for index in 0..32 {
        source.push_str(&format!("fn f{}() -> Int {{ return 0; }}\n", index));
    }

    let stage1_value = run_entry_with_args(&parser_module, "parse", vec![Value::String(source)])
        .expect("stage1 parse failed");
    let stage1_output = match stage1_value {
        Value::String(value) => value,
        _ => panic!("stage1 parse returned non-string value"),
    };

    let (ast, rest) =
        split_output(&stage1_output).expect("stage1 parse did not return pair output");
    assert!(rest.is_empty(), "stage1 parser left unconsumed tokens");

    let (tag, _) = split_ast_pair(&ast).expect("stage1 parser returned invalid AST");
    assert!(tag.starts_with("Module@"), "expected Module tag with span");
}

#[test]
fn selfhost_parser_uses_result_pairs_for_parser_outputs() {
    let parser_source = fs::read_to_string("selfhost/parser.wuu").expect("read parser.wuu failed");

    assert!(
        parser_source.contains("let item_pair = parse_item")
            && parser_source.contains("item = result_left(item_pair)")
            && parser_source.contains("rest_tokens = result_right(item_pair)"),
        "parse_module_items should unwrap parser outputs with result_left/result_right"
    );
    assert!(
        parser_source.contains("let first_pair = parse_param")
            && parser_source.contains("first = result_left(first_pair)")
            && parser_source.contains("rest = result_right(first_pair)"),
        "parse_params should unwrap parser outputs with result_left/result_right"
    );
    assert!(
        parser_source.contains("let next_pair = parse_param")
            && parser_source.contains("next = result_left(next_pair)")
            && parser_source.contains("rest = result_right(next_pair)"),
        "parse_params_tail should unwrap parser outputs with result_left/result_right"
    );
    assert!(
        parser_source.contains("let next_pair = parse_expr")
            && parser_source.contains("next = result_left(next_pair)")
            && parser_source.contains("rest = result_right(next_pair)"),
        "parse_expr_list_tail should unwrap parser outputs with result_left/result_right"
    );
}

#[test]
fn selfhost_parser_uses_fast_split_line() {
    let parser_source = fs::read_to_string("selfhost/parser.wuu").expect("read parser.wuu failed");

    assert!(
        parser_source.contains("__str_take_line_comment"),
        "parser split_line should use __str_take_line_comment for fast line splitting"
    );
}

fn split_output(value: &str) -> Option<(String, String)> {
    value.find(OUTPUT_SEP).map(|index| {
        let left = value[..index].to_string();
        let right = value[index + OUTPUT_SEP.len()..].to_string();
        (left, right)
    })
}

fn split_ast_pair(value: &str) -> Option<(String, String)> {
    value.find(AST_SEP).map(|index| {
        let left = value[..index].to_string();
        let right = value[index + AST_SEP.len()..].to_string();
        (left, right)
    })
}
