use std::fs;
use std::path::Path;

use wuu::effects::check_module;
use wuu::parser::parse_module;

#[test]
fn effect_fixtures_match_expectations() {
    let dir = Path::new("tests/effects");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }

        let source = fs::read_to_string(&path).expect("read source failed");
        let module = parse_module(&source).expect("parse failed");

        let err_path = path.with_extension("err");
        if err_path.exists() {
            let expected_raw = fs::read_to_string(&err_path).expect("read err failed");
            let expected = normalize_newlines(&expected_raw).trim().to_string();
            let err = check_module(&module).expect_err("expected effect error");
            assert_eq!(
                err.to_string(),
                expected,
                "unexpected effect error for {}",
                path.display()
            );
        } else {
            check_module(&module).unwrap_or_else(|err| {
                panic!("unexpected effect error for {}: {err}", path.display());
            });
        }

        count += 1;
    }

    assert!(count >= 5, "expected at least 5 effect fixtures");
}

fn normalize_newlines(input: &str) -> String {
    input.replace("\r\n", "\n").replace('\r', "\n")
}
