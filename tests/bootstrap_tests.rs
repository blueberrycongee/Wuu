use std::fs;
use std::path::Path;

use wuu::format::format_source;
use wuu::interpreter::{Value, run_entry_with_args};
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

#[test]
fn stage_pipeline_matches_and_is_idempotent() {
    let format_path = Path::new("selfhost/format.wuu");
    assert!(format_path.exists(), "missing selfhost/format.wuu");

    let format_source_text =
        fs::read_to_string(format_path).expect("read selfhost/format.wuu failed");
    let format_module = parse_module(&format_source_text).expect("parse format.wuu failed");
    check_types(&format_module).expect("typecheck format.wuu failed");

    let sources = [
        "selfhost/lexer.wuu",
        "selfhost/parser.wuu",
        "selfhost/format.wuu",
    ];

    for path in sources {
        let source = fs::read_to_string(path).expect("read source failed");
        let stage0 = format_source(&source).expect("stage0 format failed");
        let stage1 = run_entry_with_args(
            &format_module,
            "format",
            vec![Value::String(stage0.clone())],
        )
        .expect("stage1 format failed");
        let stage1_text = match stage1 {
            Value::String(value) => value,
            other => panic!("expected stage1 string result for {path}, got {other:?}"),
        };

        assert_eq!(stage1_text, stage0, "stage1 output mismatch for {path}");

        let stage2 = run_entry_with_args(
            &format_module,
            "format",
            vec![Value::String(stage1_text.clone())],
        )
        .expect("stage2 format failed");
        let stage2_text = match stage2 {
            Value::String(value) => value,
            other => panic!("expected stage2 string result for {path}, got {other:?}"),
        };

        assert_eq!(
            stage2_text, stage1_text,
            "stage2 output mismatch for {path}"
        );
    }
}
