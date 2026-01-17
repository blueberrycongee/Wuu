use std::fs;
use std::path::Path;

use wuu::lexer::{Token, TokenKind, lex};

#[test]
fn golden_lexer_files_match_expected_tokens() {
    let dir = Path::new("tests/golden/lexer");
    let mut count = 0usize;

    for entry in fs::read_dir(dir).expect("read_dir failed") {
        let entry = entry.expect("dir entry failed");
        let path = entry.path();
        if path.extension().and_then(|ext| ext.to_str()) != Some("wuu") {
            continue;
        }

        let expected_path = path.with_extension("tok");
        let expected_raw = fs::read_to_string(&expected_path).expect("read expected failed");
        let expected = normalize_newlines(&expected_raw).trim().to_string();

        let source = fs::read_to_string(&path).expect("read source failed");
        let tokens = lex(&source).expect("lex failed");
        let actual = format_tokens(&source, &tokens);

        assert_eq!(
            actual,
            expected,
            "unexpected token stream for {}",
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
