use std::fs;
use std::path::Path;

use wuu::interpreter::{Value, run_entry_with_args};
use wuu::lexer::{Token, TokenKind, lex};
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

#[test]
fn selfhost_lexer_matches_rust_tokens() {
    let lexer_path = Path::new("selfhost/lexer.wuu");
    assert!(lexer_path.exists(), "missing selfhost/lexer.wuu");

    let lexer_source = fs::read_to_string(lexer_path).expect("read lexer.wuu failed");
    let lexer_module = parse_module(&lexer_source).expect("parse lexer.wuu failed");
    check_types(&lexer_module).expect("typecheck lexer.wuu failed");

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

        let stage1_value =
            run_entry_with_args(&lexer_module, "lex", vec![Value::String(source.clone())])
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
