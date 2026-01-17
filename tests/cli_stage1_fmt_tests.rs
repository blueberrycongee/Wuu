use std::fs;
use std::path::Path;
use std::process::Command;

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
fn stage1_fmt_check_fails_on_unformatted() {
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

    assert!(!output.status.success(), "expected stage1 check failure");
    let stderr = String::from_utf8(output.stderr).expect("stderr utf-8");
    assert!(
        stderr.contains("file is not formatted"),
        "unexpected stderr: {stderr}"
    );
}
