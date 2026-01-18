use std::fs;
use std::path::Path;
use std::sync::OnceLock;

use wuu::bytecode::{BytecodeModule, compile_module, parse_text_module};
use wuu::interpreter::Value;
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

mod selfhost_support;

const OUTPUT_SEP: &str = "\n<OUT>\n";

struct BaseToolchain {
    parser_vm: BytecodeModule,
    stage1_compiler_vm: BytecodeModule,
    stage2_compiler_vm: BytecodeModule,
}

fn base_toolchain() -> &'static BaseToolchain {
    static TOOLCHAIN: OnceLock<BaseToolchain> = OnceLock::new();
    TOOLCHAIN.get_or_init(|| {
        let parser_vm = load_stage1_vm("selfhost/parser.wuu");
        let stage1_compiler_vm = load_stage1_vm("selfhost/compiler.wuu");
        let stage2_compiler_vm = build_stage2_compiler_vm(&parser_vm, &stage1_compiler_vm);
        BaseToolchain {
            parser_vm,
            stage1_compiler_vm,
            stage2_compiler_vm,
        }
    })
}

fn slow_tests_enabled() -> bool {
    std::env::var("WUU_SLOW_TESTS").is_ok()
}

fn lexer_vms() -> &'static (BytecodeModule, BytecodeModule) {
    static LEXER_VMS: OnceLock<(BytecodeModule, BytecodeModule)> = OnceLock::new();
    LEXER_VMS.get_or_init(|| {
        let toolchain = base_toolchain();
        let stage1 = compile_tool_with_compiler(
            &toolchain.parser_vm,
            &toolchain.stage1_compiler_vm,
            "selfhost/lexer.wuu",
        );
        let stage2 = compile_tool_with_compiler(
            &toolchain.parser_vm,
            &toolchain.stage2_compiler_vm,
            "selfhost/lexer.wuu",
        );
        (stage1, stage2)
    })
}

fn parser_vms() -> &'static (BytecodeModule, BytecodeModule) {
    static PARSER_VMS: OnceLock<(BytecodeModule, BytecodeModule)> = OnceLock::new();
    PARSER_VMS.get_or_init(|| {
        let toolchain = base_toolchain();
        let stage1 = compile_tool_with_compiler(
            &toolchain.parser_vm,
            &toolchain.stage1_compiler_vm,
            "selfhost/parser.wuu",
        );
        let stage2 = compile_tool_with_compiler(
            &toolchain.parser_vm,
            &toolchain.stage2_compiler_vm,
            "selfhost/parser.wuu",
        );
        (stage1, stage2)
    })
}

fn format_vms() -> &'static (BytecodeModule, BytecodeModule) {
    static FORMAT_VMS: OnceLock<(BytecodeModule, BytecodeModule)> = OnceLock::new();
    FORMAT_VMS.get_or_init(|| {
        let toolchain = base_toolchain();
        let stage1 = compile_tool_with_compiler(
            &toolchain.parser_vm,
            &toolchain.stage1_compiler_vm,
            "selfhost/format.wuu",
        );
        let stage2 = compile_tool_with_compiler(
            &toolchain.parser_vm,
            &toolchain.stage2_compiler_vm,
            "selfhost/format.wuu",
        );
        (stage1, stage2)
    })
}

#[test]
fn stage2_compiler_matches_stage1_output() {
    let toolchain = base_toolchain();
    let parser_vm = &toolchain.parser_vm;
    let compiler_vm = &toolchain.stage1_compiler_vm;
    let stage2_compiler = &toolchain.stage2_compiler_vm;

    let input = fs::read_to_string("tests/run/01_return_int.wuu").expect("read fixture failed");
    let input_ast = run_stage1_parse_vm(parser_vm, &input);
    let stage1_text = run_stage1_compile_vm(compiler_vm, input_ast.clone());
    let stage2_text = match stage2_compiler
        .run_entry("compile_module", vec![Value::String(input_ast)])
        .expect("stage2 compile failed")
    {
        Value::String(value) => value,
        _ => panic!("stage2 compiler returned non-string value"),
    };

    assert_eq!(stage2_text, stage1_text);
}

#[test]
fn stage2_lexer_matches_stage1_output() {
    let lexer_vms = lexer_vms();
    let stage1_lexer_vm = &lexer_vms.0;
    let stage2_lexer_vm = &lexer_vms.1;

    let input =
        fs::read_to_string("tests/golden/lexer/01_basic.wuu").expect("read lexer fixture failed");
    let stage1 = run_string_entry(stage1_lexer_vm, "lex", vec![input.clone()]);
    let stage2 = run_string_entry(stage2_lexer_vm, "lex", vec![input]);
    assert_eq!(stage2, stage1);
}

#[test]
fn stage2_parser_matches_stage1_output() {
    if !slow_tests_enabled() {
        return;
    }
    let parser_vms = parser_vms();
    let stage1_parser_vm = &parser_vms.0;
    let stage2_parser_vm = &parser_vms.1;

    let input = fs::read_to_string("tests/golden/parse/03_fn_with_let.wuu")
        .expect("read parse fixture failed");
    let stage1 = run_string_entry(stage1_parser_vm, "parse", vec![input.clone()]);
    let stage2 = run_string_entry(stage2_parser_vm, "parse", vec![input]);
    assert_eq!(stage2, stage1);
}

#[test]
fn stage2_format_matches_stage1_output() {
    if !slow_tests_enabled() {
        return;
    }
    let format_vms = format_vms();
    let stage1_format_vm = &format_vms.0;
    let stage2_format_vm = &format_vms.1;

    let input =
        fs::read_to_string("tests/golden/fmt/01_simple_fn.wuu").expect("read fmt fixture failed");
    let stage1 = run_string_entry(stage1_format_vm, "format", vec![input.clone()]);
    let stage2 = run_string_entry(stage2_format_vm, "format", vec![input]);
    assert_eq!(stage2, stage1);
}

fn load_stage1_vm(path: &str) -> BytecodeModule {
    let source = selfhost_support::load_with_stdlib(Path::new(path));
    let module = parse_module(&source).expect("parse stage1 module failed");
    check_types(&module).expect("typecheck stage1 module failed");
    compile_module(&module).expect("compile stage1 module failed")
}

fn build_stage2_compiler_vm(
    parser_vm: &BytecodeModule,
    compiler_vm: &BytecodeModule,
) -> BytecodeModule {
    let compiler_source = selfhost_support::load_with_stdlib(Path::new("selfhost/compiler.wuu"));
    let compiler_ast = run_stage1_parse_vm(parser_vm, &compiler_source);
    let stage2_text = run_stage1_compile_vm(compiler_vm, compiler_ast);
    parse_text_module(&stage2_text).expect("parse stage2 compiler failed")
}

fn compile_tool_with_compiler(
    parser_vm: &BytecodeModule,
    compiler_vm: &BytecodeModule,
    path: &str,
) -> BytecodeModule {
    let source = selfhost_support::load_with_stdlib(Path::new(path));
    let ast = run_stage1_parse_vm(parser_vm, &source);
    let text = run_stage1_compile_vm(compiler_vm, ast);
    parse_text_module(&text).expect("parse tool bytecode failed")
}

fn run_stage1_parse_vm(parser_vm: &BytecodeModule, source: &str) -> String {
    let stage1_output = match parser_vm
        .run_entry("parse", vec![Value::String(source.to_string())])
        .expect("stage1 parse failed")
    {
        Value::String(value) => value,
        _ => panic!("stage1 parser returned non-string value"),
    };
    let (ast, rest) = split_output(&stage1_output).expect("stage1 parse did not return pair");
    assert!(rest.is_empty(), "stage1 parser left unconsumed tokens");
    ast
}

fn run_stage1_compile_vm(compiler_vm: &BytecodeModule, ast: String) -> String {
    match compiler_vm
        .run_entry("compile_module", vec![Value::String(ast)])
        .expect("stage1 compile failed")
    {
        Value::String(value) => value,
        _ => panic!("stage1 compiler returned non-string value"),
    }
}

fn run_string_entry(vm: &BytecodeModule, entry: &str, args: Vec<String>) -> String {
    let value_args = args.into_iter().map(Value::String).collect();
    match vm.run_entry(entry, value_args).expect("vm run failed") {
        Value::String(value) => value,
        _ => panic!("vm returned non-string value"),
    }
}

fn split_output(value: &str) -> Option<(String, String)> {
    value.find(OUTPUT_SEP).map(|index| {
        let left = value[..index].to_string();
        let right = value[index + OUTPUT_SEP.len()..].to_string();
        (left, right)
    })
}
