use std::fs;
use std::path::Path;

use wuu::format::format_source;

#[test]
fn golden_format_files_match_expected_output() {
    let dir = Path::new("tests/golden/fmt");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        let file_name = match path.file_name().and_then(|name| name.to_str()) {
            Some(name) => name,
            None => continue,
        };
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }
        if file_name.ends_with(".fmt.wuu") {
            continue;
        }

        let expected_path = path.with_extension("fmt.wuu");
        let input = fs::read_to_string(&path).expect("read input failed");
        let expected_raw = fs::read_to_string(&expected_path).expect("read expected failed");
        let expected = normalize_newlines(&expected_raw);
        let formatted = format_source(&input).expect("format failed");

        assert_eq!(
            formatted,
            expected,
            "formatted output mismatch for {}",
            path.display()
        );

        let formatted_again = format_source(&expected).expect("format idempotence failed");
        assert_eq!(
            formatted_again,
            expected,
            "formatting should be idempotent for {}",
            expected_path.display()
        );

        count += 1;
    }

    assert!(count >= 6, "expected at least 6 fmt golden files");
}

fn normalize_newlines(input: &str) -> String {
    input.replace("\r\n", "\n").replace('\r', "\n")
}
