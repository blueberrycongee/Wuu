use crate::error::ParseError;
use crate::span::Span;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Keyword {
    Effects,
    Requires,
    Fn,
    Workflow,
    Type,
    Record,
    Enum,
    Let,
    If,
    Else,
    Match,
    Loop,
    Return,
    Step,
    Pre,
    Post,
    Invariant,
    Unsafe,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TokenKind {
    Ident,
    Keyword(Keyword),
    Punct(char),
    StringLiteral,
    Comment,
    Whitespace,
    Other,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Token {
    pub kind: TokenKind,
    pub span: Span,
}

fn is_ident_start(b: u8) -> bool {
    b.is_ascii_alphabetic() || b == b'_'
}

fn is_ident_continue(b: u8) -> bool {
    is_ident_start(b) || b.is_ascii_digit()
}

fn keyword_from_str(ident: &str) -> Option<Keyword> {
    match ident {
        "effects" => Some(Keyword::Effects),
        "requires" => Some(Keyword::Requires),
        "fn" => Some(Keyword::Fn),
        "workflow" => Some(Keyword::Workflow),
        "type" => Some(Keyword::Type),
        "record" => Some(Keyword::Record),
        "enum" => Some(Keyword::Enum),
        "let" => Some(Keyword::Let),
        "if" => Some(Keyword::If),
        "else" => Some(Keyword::Else),
        "match" => Some(Keyword::Match),
        "loop" => Some(Keyword::Loop),
        "return" => Some(Keyword::Return),
        "step" => Some(Keyword::Step),
        "pre" => Some(Keyword::Pre),
        "post" => Some(Keyword::Post),
        "invariant" => Some(Keyword::Invariant),
        "unsafe" => Some(Keyword::Unsafe),
        _ => None,
    }
}

pub fn lex(input: &str) -> Result<Vec<Token>, ParseError> {
    let bytes = input.as_bytes();
    let mut tokens = Vec::new();
    let mut i = 0usize;

    while i < bytes.len() {
        let b = bytes[i];

        if b.is_ascii_whitespace() {
            let start = i;
            i += 1;
            while i < bytes.len() && bytes[i].is_ascii_whitespace() {
                i += 1;
            }
            tokens.push(Token {
                kind: TokenKind::Whitespace,
                span: Span { start, end: i },
            });
            continue;
        }

        if b == b'/' {
            if i + 1 < bytes.len() && bytes[i + 1] == b'/' {
                let start = i;
                i += 2;
                while i < bytes.len() && bytes[i] != b'\n' {
                    i += 1;
                }
                tokens.push(Token {
                    kind: TokenKind::Comment,
                    span: Span { start, end: i },
                });
                continue;
            }
            if i + 1 < bytes.len() && bytes[i + 1] == b'*' {
                let start = i;
                i += 2;
                let mut closed = false;
                while i + 1 < bytes.len() {
                    if bytes[i] == b'*' && bytes[i + 1] == b'/' {
                        i += 2;
                        closed = true;
                        break;
                    }
                    i += 1;
                }
                if !closed {
                    return Err(ParseError::with_span(
                        "unterminated block comment",
                        Span { start, end: i },
                        input,
                    ));
                }
                tokens.push(Token {
                    kind: TokenKind::Comment,
                    span: Span { start, end: i },
                });
                continue;
            }
        }

        if b == b'"' {
            let start = i;
            i += 1;
            let mut closed = false;
            while i < bytes.len() {
                match bytes[i] {
                    b'\\' => {
                        i += 1;
                        if i < bytes.len() {
                            i += 1;
                        } else {
                            return Err(ParseError::with_span(
                                "unterminated string literal",
                                Span { start, end: i },
                                input,
                            ));
                        }
                    }
                    b'"' => {
                        i += 1;
                        closed = true;
                        break;
                    }
                    _ => {
                        i += 1;
                    }
                }
            }
            if !closed {
                return Err(ParseError::with_span(
                    "unterminated string literal",
                    Span { start, end: i },
                    input,
                ));
            }
            tokens.push(Token {
                kind: TokenKind::StringLiteral,
                span: Span { start, end: i },
            });
            continue;
        }

        if b.is_ascii() && is_ident_start(b) {
            let start = i;
            i += 1;
            while i < bytes.len() && is_ident_continue(bytes[i]) {
                i += 1;
            }
            let ident = &input[start..i];
            let kind = match keyword_from_str(ident) {
                Some(keyword) => TokenKind::Keyword(keyword),
                None => TokenKind::Ident,
            };
            tokens.push(Token {
                kind,
                span: Span { start, end: i },
            });
            continue;
        }

        if b.is_ascii() {
            let ch = b as char;
            tokens.push(Token {
                kind: TokenKind::Punct(ch),
                span: Span {
                    start: i,
                    end: i + 1,
                },
            });
            i += 1;
            continue;
        }

        let ch = input[i..]
            .chars()
            .next()
            .ok_or_else(|| ParseError::new("invalid utf-8"))?;
        let len = ch.len_utf8();
        tokens.push(Token {
            kind: TokenKind::Other,
            span: Span {
                start: i,
                end: i + len,
            },
        });
        i += len;
    }

    Ok(tokens)
}
