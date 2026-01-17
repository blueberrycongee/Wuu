use std::fs;
use std::path::Path;

use wuu::format::format_source;
use wuu::interpreter::run_entry_with_args;
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

#[test]
fn selfhost_format_matches_stage0() {
    let format_path = Path::new("selfhost/format.wuu");
    assert!(format_path.exists(), "missing selfhost/format.wuu");

    let format_source_text =
        fs::read_to_string(format_path).expect("read selfhost/format.wuu failed");
    let format_module = parse_module(&format_source_text).expect("parse format.wuu failed");
    check_types(&format_module).expect("typecheck format.wuu failed");

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

        let stage1 = run_entry_with_args(
            &format_module,
            "format",
            vec![wuu::interpreter::Value::String(source.clone())],
        )
        .expect("stage1 format failed");

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
