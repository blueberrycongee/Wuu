use std::fs;
use std::path::Path;

#[allow(dead_code)]
pub fn load_stdlib_source() -> String {
    fs::read_to_string("selfhost/stdlib.wuu").expect("read selfhost/stdlib.wuu failed")
}

#[allow(dead_code)]
pub fn load_with_stdlib(path: &Path) -> String {
    let stdlib = load_stdlib_source();
    let source = fs::read_to_string(path).expect("read selfhost source failed");
    format!("{stdlib}\n\n{source}")
}
