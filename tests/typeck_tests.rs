use std::fs;
use std::path::Path;

use wuu::parser::parse_module;
use wuu::typeck::check_module;

#[test]
fn typeck_fixtures_match_expectations() {
    let dir = Path::new("tests/typeck");
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
            let err = check_module(&module).expect_err("expected type error");
            assert_eq!(
                err.to_string(),
                expected,
                "unexpected type error for {}",
                path.display()
            );
        } else {
            check_module(&module).unwrap_or_else(|err| {
                panic!("unexpected type error for {}: {err}", path.display());
            });
        }

        count += 1;
    }

    assert!(count >= 7, "expected at least 7 typecheck fixtures");
}

fn normalize_newlines(input: &str) -> String {
    input.replace("\r\n", "\n").replace('\r', "\n")
}
