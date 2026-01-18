use std::fs;
use std::path::Path;

use wuu::bytecode::{BytecodeModule, compile_module};
use wuu::format::format_source;
use wuu::interpreter::Value;
use wuu::lexer::{Token, TokenKind, lex};
use wuu::parser::parse_module;

mod selfhost_support;

const OUTPUT_SEP: &str = "\n<OUT>\n";
const AST_SEP: &str = "\n<AST>\n";

fn load_bytecode_with_stdlib(path: &Path) -> BytecodeModule {
    let source = selfhost_support::load_with_stdlib(path);
    let module = parse_module(&source).expect("parse selfhost source failed");
    compile_module(&module).expect("compile bytecode failed")
}

#[test]
fn selfhost_lexer_vm_matches_rust_tokens() {
    let lexer_path = Path::new("selfhost/lexer.wuu");
    assert!(lexer_path.exists(), "missing selfhost/lexer.wuu");
    let lexer_vm = load_bytecode_with_stdlib(lexer_path);

    let dir = Path::new("tests/golden/lexer");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }

        let source = fs::read_to_string(&path).expect("read source failed");
        let tokens = lex(&source).expect("lex failed");
        let expected = format_tokens(&source, &tokens);

        let stage1_value = lexer_vm
            .run_entry("lex", vec![Value::String(source.clone())])
            .expect("stage1 lex failed");
        let actual = match stage1_value {
            Value::String(value) => value,
            _ => panic!("stage1 lex returned non-string value"),
        };

        let expected_norm = normalize_newlines(&expected).trim().to_string();
        let actual_norm = normalize_newlines(&actual).trim().to_string();

        assert_eq!(
            actual_norm,
            expected_norm,
            "stage1 token mismatch for {}",
            path.display()
        );

        count += 1;
    }

    assert!(count >= 3, "expected at least 3 lexer fixtures");
}

#[test]
fn selfhost_parser_vm_parses_fixture() {
    let parser_path = Path::new("selfhost/parser.wuu");
    assert!(parser_path.exists(), "missing selfhost/parser.wuu");
    let parser_vm = load_bytecode_with_stdlib(parser_path);

    let input = Path::new("tests/golden/parse/02_fn_with_return.wuu");
    let source = fs::read_to_string(input).expect("read source failed");

    let stage1_value = parser_vm
        .run_entry("parse", vec![Value::String(source)])
        .expect("stage1 parse failed");
    let stage1_output = match stage1_value {
        Value::String(value) => value,
        _ => panic!("stage1 parse returned non-string value"),
    };

    let (ast, rest) = split_output(&stage1_output).expect("stage1 parse did not return pair");
    assert!(rest.is_empty(), "stage1 parser left unconsumed tokens");

    let (tag, _) = split_ast_pair(&ast).expect("stage1 parser returned invalid AST");
    assert!(tag.starts_with("Module@"), "expected Module tag with span");
}

#[test]
fn selfhost_format_vm_matches_stage0() {
    let format_path = Path::new("selfhost/format.wuu");
    assert!(format_path.exists(), "missing selfhost/format.wuu");
    let parser_path = Path::new("selfhost/parser.wuu");
    assert!(parser_path.exists(), "missing selfhost/parser.wuu");

    let format_vm = load_bytecode_with_stdlib(format_path);
    let parser_vm = load_bytecode_with_stdlib(parser_path);

    let dir = Path::new("tests/golden/fmt");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }
        if path
            .file_name()
            .and_then(|name| name.to_str())
            .map(|n| n.ends_with(".fmt.wuu"))
            == Some(true)
        {
            continue;
        }

        let source = fs::read_to_string(&path).expect("read source failed");
        let stage0 = format_source(&source).expect("stage0 format failed");

        let stage1_parse_value = parser_vm
            .run_entry("parse", vec![Value::String(source.clone())])
            .expect("stage1 parse failed");
        let stage1_output = match stage1_parse_value {
            Value::String(value) => value,
            _ => panic!("stage1 parser returned non-string value"),
        };
        let (ast, rest) =
            split_output(&stage1_output).expect("stage1 parse did not return pair output");
        assert!(
            rest.is_empty(),
            "stage1 parser left unconsumed tokens for {}",
            path.display()
        );

        let stage1_value = format_vm
            .run_entry("format_ast", vec![Value::String(ast)])
            .expect("stage1 format_ast failed");
        let actual = match stage1_value {
            Value::String(value) => value,
            _ => panic!("stage1 format_ast returned non-string value"),
        };

        assert_eq!(
            actual,
            stage0,
            "stage1 format mismatch for {}",
            path.display()
        );

        count += 1;
    }

    assert!(count >= 3, "expected at least 3 format fixtures");
}

fn split_output(value: &str) -> Option<(String, String)> {
    value.find(OUTPUT_SEP).map(|index| {
        let left = value[..index].to_string();
        let right = value[index + OUTPUT_SEP.len()..].to_string();
        (left, right)
    })
}

fn split_ast_pair(value: &str) -> Option<(String, String)> {
    value.find(AST_SEP).map(|index| {
        let left = value[..index].to_string();
        let right = value[index + AST_SEP.len()..].to_string();
        (left, right)
    })
}

fn format_tokens(source: &str, tokens: &[Token]) -> String {
    let mut lines = Vec::new();
    for token in tokens {
        match &token.kind {
            TokenKind::Whitespace | TokenKind::Comment => continue,
            TokenKind::Keyword(_) => {
                let text = escape(token_text(source, token));
                lines.push(format!("Keyword {text}"));
            }
            TokenKind::Ident => {
                let text = escape(token_text(source, token));
                lines.push(format!("Ident {text}"));
            }
            TokenKind::Number => {
                let text = escape(token_text(source, token));
                lines.push(format!("Number {text}"));
            }
            TokenKind::StringLiteral => {
                let text = escape(token_text(source, token));
                lines.push(format!("StringLiteral {text}"));
            }
            TokenKind::Punct(ch) => {
                lines.push(format!("Punct {ch}"));
            }
            TokenKind::Other => {
                let text = escape(token_text(source, token));
                lines.push(format!("Other {text}"));
            }
        }
    }

    lines.join("\n")
}

fn token_text<'a>(source: &'a str, token: &Token) -> &'a str {
    &source[token.span.start..token.span.end]
}

fn escape(text: &str) -> String {
    text.replace('\\', "\\\\")
        .replace('\n', "\\n")
        .replace('\r', "\\r")
        .replace('\t', "\\t")
}

fn normalize_newlines(input: &str) -> String {
    input.replace("\r\n", "\n").replace('\r', "\n")
}
