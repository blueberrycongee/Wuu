use std::fs;
use std::path::Path;

use wuu::bytecode::compile_module;
use wuu::interpreter::run_entry;
use wuu::parser::parse_module;

#[test]
fn bytecode_vm_matches_interpreter_on_run_fixtures() {
    let dir = Path::new("tests/run");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }

        let source = fs::read_to_string(&path).expect("read source failed");
        let module = parse_module(&source).expect("parse failed");
        let expected = run_entry(&module, "main").expect("interpreter run failed");

        let bytecode = compile_module(&module).expect("bytecode compile failed");
        let actual = bytecode
            .run_entry("main", Vec::new())
            .expect("vm run failed");

        assert_eq!(
            actual.to_string(),
            expected.to_string(),
            "vm output mismatch for {}",
            path.display()
        );

        count += 1;
    }

    assert!(count >= 4, "expected at least 4 run fixtures");
}
