use std::fs;
use std::path::Path;
use std::process::Command;

fn normalize_newlines(input: &str) -> String {
    input.replace("\r\n", "\n").replace('\r', "\n")
}

#[test]
fn stage1_lex_matches_golden_tokens() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let input = Path::new("tests/golden/lexer/02_string_call.wuu");
    let expected_path = input.with_extension("tok");

    let expected_raw = fs::read_to_string(&expected_path).expect("read expected failed");
    let expected = normalize_newlines(&expected_raw).trim().to_string();

    let output = Command::new(bin)
        .args(["lex", "--stage1", input.to_str().expect("input path utf-8")])
        .output()
        .expect("run wuu lex --stage1 failed");

    assert!(output.status.success(), "stage1 lex failed");
    let stdout = String::from_utf8(output.stdout).expect("stdout utf-8");
    let actual = normalize_newlines(&stdout).trim().to_string();
    assert_eq!(actual, expected, "stage1 lex mismatch");
}

#[test]
fn stage1_lex_matches_escape_fixture() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let input = Path::new("tests/golden/lexer/04_escapes.wuu");
    let expected_path = input.with_extension("tok");

    let expected_raw = fs::read_to_string(&expected_path).expect("read expected failed");
    let expected = normalize_newlines(&expected_raw).trim().to_string();

    let output = Command::new(bin)
        .args(["lex", "--stage1", input.to_str().expect("input path utf-8")])
        .output()
        .expect("run wuu lex --stage1 failed");

    assert!(output.status.success(), "stage1 lex failed");
    let stdout = String::from_utf8(output.stdout).expect("stdout utf-8");
    let actual = normalize_newlines(&stdout).trim().to_string();
    assert_eq!(actual, expected, "stage1 lex escape mismatch");
}
