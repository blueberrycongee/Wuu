use std::fs;
use std::path::Path;

use wuu::format::format_source;
use wuu::interpreter::{Value, run_entry_with_args};
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

mod selfhost_support;

const OUTPUT_SEP: &str = "\n<OUT>\n";

#[test]
fn stage_pipeline_matches_and_is_idempotent() {
    let format_path = Path::new("selfhost/format.wuu");
    assert!(format_path.exists(), "missing selfhost/format.wuu");
    let parser_path = Path::new("selfhost/parser.wuu");
    assert!(parser_path.exists(), "missing selfhost/parser.wuu");

    let format_source_text = selfhost_support::load_with_stdlib(format_path);
    let format_module = parse_module(&format_source_text).expect("parse format.wuu failed");
    check_types(&format_module).expect("typecheck format.wuu failed");
    let parser_source_text = selfhost_support::load_with_stdlib(parser_path);
    let parser_module = parse_module(&parser_source_text).expect("parse parser.wuu failed");
    check_types(&parser_module).expect("typecheck parser.wuu failed");

    let sources = [
        "selfhost/lexer.wuu",
        "selfhost/parser.wuu",
        "selfhost/format.wuu",
    ];

    for path in sources {
        let source = fs::read_to_string(path).expect("read source failed");
        let stage0 = format_source(&source).expect("stage0 format failed");
        let stage1_text = format_stage1_ast(&parser_module, &format_module, &stage0)
            .expect("stage1 format_ast failed");

        assert_eq!(stage1_text, stage0, "stage1 output mismatch for {path}");

        let stage2_text = format_stage1_ast(&parser_module, &format_module, &stage1_text)
            .expect("stage2 format_ast failed");

        assert_eq!(
            stage2_text, stage1_text,
            "stage2 output mismatch for {path}"
        );
    }
}

fn format_stage1_ast(
    parser_module: &wuu::ast::Module,
    format_module: &wuu::ast::Module,
    source: &str,
) -> anyhow::Result<String> {
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
    let format_value = run_entry_with_args(format_module, "format_ast", vec![Value::String(ast)])?;
    match format_value {
        Value::String(value) => Ok(value),
        other => Err(anyhow::anyhow!(
            "stage1 formatter returned non-string value: {other:?}"
        )),
    }
}

fn split_output(value: &str) -> Option<(String, String)> {
    value.find(OUTPUT_SEP).map(|index| {
        let left = value[..index].to_string();
        let right = value[index + OUTPUT_SEP.len()..].to_string();
        (left, right)
    })
}
