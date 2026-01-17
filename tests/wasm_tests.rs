use std::fs;
use std::path::Path;

use wuu::interpreter::run_entry;
use wuu::parser::parse_module;
use wuu::typeck::check_module;
use wuu::wasm::run_entry as run_entry_wasm;

#[test]
fn wasm_fixtures_match_interpreter() {
    let dir = Path::new("tests/wasm");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }

        let expected_path = path.with_extension("out");
        let expected_raw = fs::read_to_string(&expected_path).expect("read expected failed");
        let expected = normalize_newlines(&expected_raw).trim().to_string();

        let source = fs::read_to_string(&path).expect("read source failed");
        let module = parse_module(&source).expect("parse failed");
        check_module(&module).expect("typecheck failed");

        let interp = run_entry(&module, "main").expect("interpreter failed");
        let wasm = run_entry_wasm(&module, "main").unwrap_or_else(|err| {
            panic!("wasm failed for {}: {err}", path.display());
        });

        assert_eq!(interp, wasm, "mismatched value for {}", path.display());
        assert_eq!(
            wasm.to_string(),
            expected,
            "unexpected output for {}",
            path.display()
        );

        count += 1;
    }

    assert!(count >= 3, "expected at least 3 wasm fixtures");
}

#[test]
fn wasm_rejects_unsupported_fixtures() {
    let dir = Path::new("tests/wasm_errors");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }

        let expected_path = path.with_extension("err");
        let expected_raw = fs::read_to_string(&expected_path).expect("read expected failed");
        let expected = normalize_newlines(&expected_raw).trim().to_string();

        let source = fs::read_to_string(&path).expect("read source failed");
        let module = parse_module(&source).expect("parse failed");

        let err = run_entry_wasm(&module, "main").expect_err("expected wasm error");
        assert_eq!(
            err.to_string(),
            expected,
            "unexpected wasm error for {}",
            path.display()
        );

        count += 1;
    }

    assert!(count >= 1, "expected at least 1 wasm error fixture");
}

fn normalize_newlines(input: &str) -> String {
    input.replace("\r\n", "\n").replace('\r', "\n")
}
