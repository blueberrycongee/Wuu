use wuu::lexer::{Keyword, TokenKind, lex};

#[test]
fn lex_emits_core_token_kinds() {
    let input = "effects { Net.Http } // comment\n\"ok\"";
    let tokens = lex(input).unwrap();

    assert!(
        tokens
            .iter()
            .any(|token| token.kind == TokenKind::Whitespace)
    );
    assert!(
        tokens
            .iter()
            .any(|token| token.kind == TokenKind::Punct('{'))
    );
    assert!(
        tokens
            .iter()
            .any(|token| token.kind == TokenKind::Punct('}'))
    );
    assert!(tokens.iter().any(|token| token.kind == TokenKind::Comment));
    assert!(
        tokens
            .iter()
            .any(|token| token.kind == TokenKind::StringLiteral)
    );
    assert!(
        tokens
            .iter()
            .any(|token| token.kind == TokenKind::Keyword(Keyword::Effects))
    );
}
