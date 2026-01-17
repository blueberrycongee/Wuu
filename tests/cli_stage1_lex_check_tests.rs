use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

fn temp_file_path(name: &str) -> PathBuf {
    let mut base = std::env::temp_dir();
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("time before epoch")
        .as_nanos();
    base.push(format!("wuu_stage1_lex_{nonce}_{}", std::process::id()));
    fs::create_dir_all(&base).expect("create temp dir");
    base.push(name);
    base
}

#[test]
fn stage1_lex_check_succeeds_on_fixture() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let input = Path::new("tests/golden/lexer/02_string_call.wuu");

    let output = Command::new(bin)
        .args([
            "lex",
            "--stage1",
            "--check",
            input.to_str().expect("input path utf-8"),
        ])
        .output()
        .expect("run wuu lex --stage1 --check failed");

    assert!(output.status.success(), "stage1 lex --check failed");
    assert!(output.stdout.is_empty(), "expected empty stdout");
}

#[test]
fn stage1_lex_check_succeeds_on_escape_fixture() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let input = Path::new("tests/golden/lexer/04_escapes.wuu");

    let output = Command::new(bin)
        .args([
            "lex",
            "--stage1",
            "--check",
            input.to_str().expect("input path utf-8"),
        ])
        .output()
        .expect("run wuu lex --stage1 --check failed");

    assert!(output.status.success(), "stage1 lex --check failed");
    assert!(output.stdout.is_empty(), "expected empty stdout");
}

#[test]
fn stage1_lex_check_fails_on_invalid_utf8() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let temp_path = temp_file_path("stage1_lex_invalid.wuu");
    fs::write(&temp_path, [0xFF, 0xFE, 0x00]).expect("write temp file failed");

    let output = Command::new(bin)
        .args([
            "lex",
            "--stage1",
            "--check",
            temp_path.to_str().expect("temp path utf-8"),
        ])
        .output()
        .expect("run wuu lex --stage1 --check failed");

    assert!(!output.status.success(), "expected lex check failure");
    let stderr = String::from_utf8(output.stderr).expect("stderr utf-8");
    assert!(
        stderr.contains("invalid utf-8"),
        "unexpected stderr: {stderr}"
    );
}
