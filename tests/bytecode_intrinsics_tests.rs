use wuu::bytecode::compile_module;
use wuu::interpreter::{Value, run_entry};
use wuu::parser::parse_module;

fn run_vm(source: &str) -> Result<Value, String> {
    let module = parse_module(source).map_err(|err| err.to_string())?;
    let bytecode = compile_module(&module).map_err(|err| err.to_string())?;
    bytecode
        .run_entry("main", Vec::new())
        .map_err(|err| err.to_string())
}

fn run_interpreter(source: &str) -> Result<Value, String> {
    let module = parse_module(source).map_err(|err| err.to_string())?;
    run_entry(&module, "main").map_err(|err| err.to_string())
}

#[test]
fn bytecode_vm_handles_string_intrinsics() {
    let source = r#"
fn main() {
  let joined = __str_concat("hi", "!");
  let ok = __str_eq(joined, "hi!");
  if ok {
    return __str_tail(joined);
  } else {
    return "no";
  }
}
"#;

    let expected = run_interpreter(source).expect("interpreter run failed");
    let actual = run_vm(source).expect("vm run failed");
    assert_eq!(actual, expected);
}

#[test]
fn bytecode_vm_handles_lexer_intrinsic() {
    let source = r#"
fn main() {
  return __lex_tokens("let a = 1");
}
"#;

    let expected = run_interpreter(source).expect("interpreter run failed");
    let actual = run_vm(source).expect("vm run failed");
    assert_eq!(actual, expected);
}

#[test]
fn bytecode_vm_reports_intrinsic_errors() {
    let source = r#"
fn main() {
  return __str_head("");
}
"#;

    let expected = run_interpreter(source).expect_err("expected interpreter error");
    let actual = run_vm(source).expect_err("expected vm error");
    assert_eq!(actual, expected);
}
