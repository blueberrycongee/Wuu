use std::fs;
use std::path::Path;

use wuu::format::format_source;
use wuu::interpreter::{Value, run_entry_with_args};
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

const PAIR_SEP: &str = "\n<SEP>\n";

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

        let (formatted, rest) =
            split_pair(&stage1_output).expect("stage1 parse did not return pair output");
        assert!(
            rest.is_empty(),
            "stage1 parser left unconsumed tokens for {}",
            path.display()
        );
        assert_eq!(
            formatted,
            stage0,
            "stage1 parse formatting mismatch for {}",
            path.display()
        );

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
        parser_source.contains("__lex_tokens"),
        "selfhost/parser.wuu should include a host-backed lex_tokens fallback"
    );
    assert!(
        !parser_source.contains("let tokens = __lex_tokens"),
        "selfhost/parser.wuu should not call __lex_tokens directly in parse()"
    );
}

fn split_pair(value: &str) -> Option<(String, String)> {
    value.find(PAIR_SEP).map(|index| {
        let left = value[..index].to_string();
        let right = value[index + PAIR_SEP.len()..].to_string();
        (left, right)
    })
}
