use std::fs;
use std::path::Path;

use wuu::format::format_source;
use wuu::interpreter::{Value, run_entry_with_args};
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

const OUTPUT_SEP: &str = "\n<OUT>\n";

#[test]
fn selfhost_format_matches_stage0() {
    let format_path = Path::new("selfhost/format.wuu");
    assert!(format_path.exists(), "missing selfhost/format.wuu");
    let parser_path = Path::new("selfhost/parser.wuu");
    assert!(parser_path.exists(), "missing selfhost/parser.wuu");

    let format_source_text =
        fs::read_to_string(format_path).expect("read selfhost/format.wuu failed");
    let format_module = parse_module(&format_source_text).expect("parse format.wuu failed");
    check_types(&format_module).expect("typecheck format.wuu failed");
    let parser_source_text =
        fs::read_to_string(parser_path).expect("read selfhost/parser.wuu failed");
    let parser_module = parse_module(&parser_source_text).expect("parse parser.wuu failed");
    check_types(&parser_module).expect("typecheck parser.wuu failed");

    let dir = Path::new("tests/golden/fmt");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }
        if path
            .file_name()
            .and_then(|name| name.to_str())
            .map(|n| n.ends_with(".fmt.wuu"))
            == Some(true)
        {
            continue;
        }

        let source = fs::read_to_string(&path).expect("read source failed");
        let stage0 = format_source(&source).expect("stage0 format failed");

        let stage1_output = parse_stage1_ast(&parser_module, &source).expect("stage1 parse failed");
        let stage1 = run_entry_with_args(
            &format_module,
            "format_ast",
            vec![Value::String(stage1_output)],
        )
        .expect("stage1 format_ast failed");

        assert_eq!(
            stage1.to_string(),
            stage0,
            "stage1 format mismatch for {}",
            path.display()
        );

        count += 1;
    }

    assert!(count >= 3, "expected at least 3 format fixtures");
}

#[test]
fn selfhost_format_uses_lex_tokens_wrapper() {
    let format_path = Path::new("selfhost/format.wuu");
    let format_source_text =
        fs::read_to_string(format_path).expect("read selfhost/format.wuu failed");
    assert!(
        format_source_text.contains("let tokens = lex_tokens"),
        "selfhost/format.wuu should route through lex_tokens"
    );
    assert!(
        format_source_text.contains("__lex_tokens"),
        "selfhost/format.wuu should provide a host-backed lex_tokens fallback"
    );
}

fn parse_stage1_ast(parser_module: &wuu::ast::Module, source: &str) -> anyhow::Result<String> {
    let stage1_value = run_entry_with_args(
        parser_module,
        "parse",
        vec![Value::String(source.to_string())],
    )?;
    let stage1_output = match stage1_value {
        Value::String(value) => value,
        _ => anyhow::bail!("stage1 parser returned non-string value"),
    };
    let (ast, rest) = split_output(&stage1_output)
        .ok_or_else(|| anyhow::anyhow!("stage1 parser returned invalid output"))?;
    if !rest.is_empty() {
        anyhow::bail!("stage1 parser left unconsumed tokens");
    }
    Ok(ast)
}

fn split_output(value: &str) -> Option<(String, String)> {
    value.find(OUTPUT_SEP).map(|index| {
        let left = value[..index].to_string();
        let right = value[index + OUTPUT_SEP.len()..].to_string();
        (left, right)
    })
}
