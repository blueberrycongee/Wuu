use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

use wuu::format::format_source;

fn temp_file_path(name: &str) -> PathBuf {
    let mut base = std::env::temp_dir();
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("time before epoch")
        .as_nanos();
    base.push(format!("wuu_stage1_parse_{nonce}_{}", std::process::id()));
    fs::create_dir_all(&base).expect("create temp dir");
    base.push(name);
    base
}

#[test]
fn stage1_parse_matches_stage0_format() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let input = Path::new("tests/golden/parse/02_fn_with_return.wuu");

    let source = fs::read_to_string(input).expect("read input failed");
    let expected = format_source(&source).expect("stage0 format failed");

    let output = Command::new(bin)
        .args([
            "parse",
            "--stage1",
            input.to_str().expect("input path utf-8"),
        ])
        .output()
        .expect("run wuu parse --stage1 failed");

    assert!(output.status.success(), "stage1 parse failed");
    let stdout = String::from_utf8(output.stdout).expect("stdout utf-8");
    assert_eq!(stdout, expected, "stage1 parse mismatch");
}

#[test]
fn stage1_parse_fails_on_leftover_tokens() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let temp_path = temp_file_path("stage1_parse_invalid.wuu");
    fs::write(&temp_path, "type Foo {}").expect("write temp file failed");

    let output = Command::new(bin)
        .args([
            "parse",
            "--stage1",
            temp_path.to_str().expect("temp path utf-8"),
        ])
        .output()
        .expect("run wuu parse --stage1 failed");

    assert!(!output.status.success(), "expected parse failure");
    let stderr = String::from_utf8(output.stderr).expect("stderr utf-8");
    assert!(
        stderr.contains("stage1 parser left unconsumed tokens"),
        "unexpected stderr: {stderr}"
    );
    assert!(
        stderr.contains("1:1:"),
        "expected line/col span in stderr: {stderr}"
    );
}

#[test]
fn stage1_parse_reports_span_on_second_line() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let temp_path = temp_file_path("stage1_parse_invalid_line2.wuu");
    let source = "fn ok() {}\ntype Foo {}";
    fs::write(&temp_path, source).expect("write temp file failed");

    let output = Command::new(bin)
        .args([
            "parse",
            "--stage1",
            temp_path.to_str().expect("temp path utf-8"),
        ])
        .output()
        .expect("run wuu parse --stage1 failed");

    assert!(!output.status.success(), "expected parse failure");
    let stderr = String::from_utf8(output.stderr).expect("stderr utf-8");
    assert!(
        stderr.contains("stage1 parser left unconsumed tokens"),
        "unexpected stderr: {stderr}"
    );
    assert!(
        stderr.contains("2:1:"),
        "expected line/col span in stderr: {stderr}"
    );
}
