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
    base.push(format!("wuu_stage1_fmt_{nonce}_{}", std::process::id()));
    fs::create_dir_all(&base).expect("create temp dir");
    base.push(name);
    base
}

fn read_fixture(path: &Path) -> String {
    fs::read_to_string(path).expect("read fixture failed")
}

#[test]
fn stage1_fmt_write_updates_file() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let input_path = Path::new("tests/golden/fmt/07_call_args.wuu");
    let expected_path = input_path.with_extension("fmt.wuu");

    let input = read_fixture(input_path);
    let expected = read_fixture(&expected_path);

    let temp_path = temp_file_path("stage1_write.wuu");
    fs::write(&temp_path, input).expect("write temp file failed");

    let output = Command::new(bin)
        .args([
            "fmt",
            "--stage1",
            "--write",
            temp_path.to_str().expect("temp path utf-8"),
        ])
        .output()
        .expect("run wuu fmt --stage1 --write failed");

    assert!(output.status.success(), "stage1 write failed");
    let actual = fs::read_to_string(&temp_path).expect("read temp file failed");
    assert_eq!(actual, expected, "stage1 write mismatch");
}

#[test]
fn stage1_fmt_write_conflicts_with_check() {
    let bin = env!("CARGO_BIN_EXE_wuu");
    let input_path = Path::new("tests/golden/fmt/07_call_args.wuu");

    let output = Command::new(bin)
        .args([
            "fmt",
            "--stage1",
            "--write",
            "--check",
            input_path.to_str().expect("input path utf-8"),
        ])
        .output()
        .expect("run wuu fmt --stage1 --write --check failed");

    assert!(!output.status.success(), "expected conflict failure");
    let stderr = String::from_utf8(output.stderr).expect("stderr utf-8");
    assert!(
        stderr.contains("cannot be used with") || stderr.contains("conflicts with"),
        "unexpected stderr: {stderr}"
    );
}
