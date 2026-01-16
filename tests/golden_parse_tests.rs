use std::fs;
use std::path::Path;

use wuu::parser::parse_module;

#[test]
fn golden_parse_files() {
    let dir = Path::new("tests/golden/parse");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }
        count += 1;
        let source = fs::read_to_string(&path).expect("read_to_string failed");
        parse_module(&source).unwrap_or_else(|err| {
            panic!("failed to parse {}: {err}", path.display());
        });
    }

    assert!(count >= 10, "expected at least 10 golden parse files");
}
