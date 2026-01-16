use std::fs;
use std::path::Path;

use wuu::interpreter::run_entry;
use wuu::parser::parse_module;

#[test]
fn run_fixtures() {
    let dir = Path::new("tests/run");
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
        let value = run_entry(&module, "main").expect("run failed");

        assert_eq!(
            value.to_string(),
            expected,
            "unexpected output for {}",
            path.display()
        );
        count += 1;
    }

    assert!(count >= 4, "expected at least 4 run fixtures");
}

fn normalize_newlines(input: &str) -> String {
    input.replace("\r\n", "\n").replace('\r', "\n")
}
