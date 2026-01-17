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
    base.push(format!(
        "wuu_stage1_fmt_check_{nonce}_{}",
        std::process::id()
    ));
    fs::create_dir_all(&base).expect("create temp dir");
    base.push(name);
    base
}

#[test]
fn stage1_fmt_matches_golden_output() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let input = Path::new("tests/golden/fmt/07_call_args.wuu");
    let expected_path = input.with_extension("fmt.wuu");

    let expected = fs::read_to_string(&expected_path).expect("read expected failed");

    let output = Command::new(bin)
        .args(["fmt", "--stage1", input.to_str().expect("input path utf-8")])
        .output()
        .expect("run wuu fmt --stage1 failed");

    assert!(output.status.success(), "stage1 fmt failed");
    let stdout = String::from_utf8(output.stdout).expect("stdout utf-8");
    assert_eq!(stdout, expected, "stage1 fmt mismatch");
}

#[test]
fn stage1_fmt_check_matches_stage0() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let input = Path::new("tests/golden/fmt/07_call_args.wuu");

    let output = Command::new(bin)
        .args([
            "fmt",
            "--stage1",
            "--check",
            input.to_str().expect("input path utf-8"),
        ])
        .output()
        .expect("run wuu fmt --stage1 --check failed");

    assert!(output.status.success(), "expected stage1 check success");
}

#[test]
fn stage1_fmt_check_fails_on_parity_mismatch() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let contents = "fn main() {\n    let value = \"\\x\";\n    return;\n}\n";
    let temp_path = temp_file_path("stage1_fmt_parity_mismatch.wuu");
    fs::write(&temp_path, contents).expect("write temp file failed");

    let output = Command::new(bin)
        .args([
            "fmt",
            "--stage1",
            "--check",
            temp_path.to_str().expect("temp path utf-8"),
        ])
        .output()
        .expect("run wuu fmt --stage1 --check failed");

    assert!(!output.status.success(), "expected stage1 parity failure");
    let stderr = String::from_utf8(output.stderr).expect("stderr utf-8");
    assert!(
        stderr.contains("stage1 formatter output differs from stage0"),
        "unexpected stderr: {stderr}"
    );
}
