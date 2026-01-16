use crate::error::ParseError;
use crate::lexer::{Keyword, Token, TokenKind, lex};

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum DeclKind {
    Effects,
    Requires,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Decl {
    pub kind: DeclKind,
    pub items: Vec<String>,
}

fn is_ident_start(b: u8) -> bool {
    b.is_ascii_alphabetic() || b == b'_'
}

fn is_ident_continue(b: u8) -> bool {
    is_ident_start(b) || b.is_ascii_digit()
}

fn skip_ws(input: &[u8], mut i: usize) -> usize {
    while i < input.len() && input[i].is_ascii_whitespace() {
        i += 1;
    }
    i
}

fn parse_ident(input: &[u8], mut i: usize) -> Result<(String, usize), ParseError> {
    if i >= input.len() || !is_ident_start(input[i]) {
        return Err(ParseError::new("invalid identifier"));
    }
    let start = i;
    i += 1;
    while i < input.len() && is_ident_continue(input[i]) {
        i += 1;
    }
    Ok((
        std::str::from_utf8(&input[start..i])
            .map_err(|_| ParseError::new("invalid identifier encoding"))?
            .to_string(),
        i,
    ))
}

fn parse_path(input: &[u8], mut i: usize) -> Result<(String, usize), ParseError> {
    let (first, j) = parse_ident(input, i)?;
    let mut parts = vec![first];
    i = j;
    while i < input.len() && input[i] == b'.' {
        i += 1;
        if i >= input.len() {
            return Err(ParseError::new("invalid path: trailing '.'"));
        }
        let (seg, next) = parse_ident(input, i)?;
        parts.push(seg);
        i = next;
    }
    Ok((parts.join("."), i))
}

fn parse_keyword(input: &[u8], i: usize, kw: &[u8]) -> Option<usize> {
    let end = i.checked_add(kw.len())?;
    if end > input.len() {
        return None;
    }
    if &input[i..end] != kw {
        return None;
    }
    Some(end)
}

pub fn parse_decl(input: &str) -> Result<Decl, ParseError> {
    let bytes = input.as_bytes();
    let mut i = skip_ws(bytes, 0);

    let (kind, after_kw) = if let Some(j) = parse_keyword(bytes, i, b"effects") {
        (DeclKind::Effects, j)
    } else if let Some(j) = parse_keyword(bytes, i, b"requires") {
        (DeclKind::Requires, j)
    } else {
        return Err(ParseError::new("expected 'effects' or 'requires'"));
    };

    i = skip_ws(bytes, after_kw);
    if i >= bytes.len() || bytes[i] != b'{' {
        return Err(ParseError::new("expected '{'"));
    }
    i += 1;

    let mut items = Vec::new();
    loop {
        i = skip_ws(bytes, i);
        if i >= bytes.len() {
            return Err(ParseError::new("unterminated declaration"));
        }

        if bytes[i] == b'}' {
            i += 1;
            break;
        }

        match kind {
            DeclKind::Effects => {
                let (path, next) = parse_path(bytes, i)
                    .map_err(|e| ParseError::new(format!("invalid effects path: {e}")))?;
                items.push(path);
                i = next;
            }
            DeclKind::Requires => {
                let (left, next) = parse_ident(bytes, i)
                    .map_err(|e| ParseError::new(format!("invalid requires pair: {e}")))?;
                i = skip_ws(bytes, next);
                if i >= bytes.len() || bytes[i] != b':' {
                    return Err(ParseError::new("invalid requires pair: expected ':'"));
                }
                i += 1;
                i = skip_ws(bytes, i);
                let (right, next2) = parse_ident(bytes, i)
                    .map_err(|e| ParseError::new(format!("invalid requires pair: {e}")))?;
                items.push(format!("{left}:{right}"));
                i = next2;
            }
        }

        i = skip_ws(bytes, i);
        if i >= bytes.len() {
            return Err(ParseError::new("unterminated declaration"));
        }
        match bytes[i] {
            b',' => {
                i += 1;
                continue;
            }
            b'}' => {
                i += 1;
                break;
            }
            _ => {
                return Err(ParseError::new("invalid declaration: expected ',' or '}'"));
            }
        }
    }

    i = skip_ws(bytes, i);
    if i != bytes.len() {
        return Err(ParseError::new("unexpected trailing input"));
    }

    Ok(Decl { kind, items })
}

pub fn format_decl(decl: &Decl) -> String {
    let (kw, items) = match decl.kind {
        DeclKind::Effects => ("effects", decl.items.join(", ")),
        DeclKind::Requires => ("requires", decl.items.join(", ")),
    };
    if decl.items.is_empty() {
        format!("{kw} {{}}")
    } else {
        format!("{kw} {{ {items} }}")
    }
}

pub fn format_source_bytes(input: &[u8]) -> Result<String, ParseError> {
    let source = std::str::from_utf8(input).map_err(|_| ParseError::new("invalid utf-8"))?;
    format_source(source)
}

pub fn format_source(input: &str) -> Result<String, ParseError> {
    let tokens = lex(input)?;
    let mut out = String::with_capacity(input.len());
    let mut last = 0usize;
    let mut i = 0usize;

    while i < tokens.len() {
        let token = &tokens[i];
        if matches!(
            token.kind,
            TokenKind::Keyword(Keyword::Effects | Keyword::Requires)
        ) && let Some((decl, end)) = try_parse_decl_tokens(input, &tokens, i)?
        {
            out.push_str(&input[last..token.span.start]);
            out.push_str(&format_decl(&decl));
            last = end;

            while i < tokens.len() && tokens[i].span.end <= end {
                i += 1;
            }
            continue;
        }
        i += 1;
    }

    out.push_str(&input[last..]);
    Ok(out)
}

fn try_parse_decl_tokens(
    input: &str,
    tokens: &[Token],
    kw_index: usize,
) -> Result<Option<(Decl, usize)>, ParseError> {
    let start = tokens[kw_index].span.start;
    let mut i = kw_index + 1;

    while i < tokens.len() {
        match tokens[i].kind {
            TokenKind::Whitespace | TokenKind::Comment => {
                i += 1;
            }
            TokenKind::Punct('{') => break,
            _ => return Ok(None),
        }
    }

    if i >= tokens.len() {
        return Ok(None);
    }

    let mut depth = 0usize;
    let mut end = None;
    let mut j = i;
    while j < tokens.len() {
        match tokens[j].kind {
            TokenKind::Punct('{') => depth += 1,
            TokenKind::Punct('}') => {
                if depth == 0 {
                    return Err(ParseError::new("unterminated declaration"));
                }
                depth -= 1;
                if depth == 0 {
                    end = Some(tokens[j].span.end);
                    break;
                }
            }
            _ => {}
        }
        j += 1;
    }

    let end = match end {
        Some(end) => end,
        None => {
            return Err(ParseError::new("unterminated declaration"));
        }
    };

    let decl_str = &input[start..end];
    let decl = parse_decl(decl_str)?;
    Ok(Some((decl, end)))
}
