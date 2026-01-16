use std::fmt;

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

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ParseError {
    pub message: String,
}

impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for ParseError {}

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
        return Err(ParseError {
            message: "invalid identifier".to_string(),
        });
    }
    let start = i;
    i += 1;
    while i < input.len() && is_ident_continue(input[i]) {
        i += 1;
    }
    Ok((
        std::str::from_utf8(&input[start..i])
            .map_err(|_| ParseError {
                message: "invalid identifier encoding".to_string(),
            })?
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
            return Err(ParseError {
                message: "invalid path: trailing '.'".to_string(),
            });
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
        return Err(ParseError {
            message: "expected 'effects' or 'requires'".to_string(),
        });
    };

    i = skip_ws(bytes, after_kw);
    if i >= bytes.len() || bytes[i] != b'{' {
        return Err(ParseError {
            message: "expected '{'".to_string(),
        });
    }
    i += 1;

    let mut items = Vec::new();
    loop {
        i = skip_ws(bytes, i);
        if i >= bytes.len() {
            return Err(ParseError {
                message: "unterminated declaration".to_string(),
            });
        }

        if bytes[i] == b'}' {
            i += 1;
            break;
        }

        match kind {
            DeclKind::Effects => {
                let (path, next) = parse_path(bytes, i).map_err(|e| ParseError {
                    message: format!("invalid effects path: {e}"),
                })?;
                items.push(path);
                i = next;
            }
            DeclKind::Requires => {
                let (left, next) = parse_ident(bytes, i).map_err(|e| ParseError {
                    message: format!("invalid requires pair: {e}"),
                })?;
                i = skip_ws(bytes, next);
                if i >= bytes.len() || bytes[i] != b':' {
                    return Err(ParseError {
                        message: "invalid requires pair: expected ':'".to_string(),
                    });
                }
                i += 1;
                i = skip_ws(bytes, i);
                let (right, next2) = parse_ident(bytes, i).map_err(|e| ParseError {
                    message: format!("invalid requires pair: {e}"),
                })?;
                items.push(format!("{left}:{right}"));
                i = next2;
            }
        }

        i = skip_ws(bytes, i);
        if i >= bytes.len() {
            return Err(ParseError {
                message: "unterminated declaration".to_string(),
            });
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
                return Err(ParseError {
                    message: "invalid declaration: expected ',' or '}'".to_string(),
                });
            }
        }
    }

    i = skip_ws(bytes, i);
    if i != bytes.len() {
        return Err(ParseError {
            message: "unexpected trailing input".to_string(),
        });
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

pub fn format_source(input: &str) -> Result<String, ParseError> {
    let bytes = input.as_bytes();
    let mut out = String::with_capacity(input.len());

    let mut last = 0usize;
    let mut it = input.char_indices().peekable();
    while let Some((i, ch)) = it.next() {
        if !ch.is_ascii() {
            continue;
        }

        let b = bytes[i];
        if is_ident_start(b) {
            let prev_is_ident = i > 0 && is_ident_continue(bytes[i - 1]);
            if !prev_is_ident && let Some((decl, consumed)) = try_parse_decl_at(input, i)? {
                out.push_str(&input[last..i]);
                out.push_str(&format_decl(&decl));
                let skip_to = i + consumed;
                last = skip_to;

                while let Some(&(next_i, _)) = it.peek() {
                    if next_i >= skip_to {
                        break;
                    }
                    let _ = it.next();
                }
            }
        }
    }

    out.push_str(&input[last..]);
    Ok(out)
}

fn try_parse_decl_at(input: &str, i: usize) -> Result<Option<(Decl, usize)>, ParseError> {
    let bytes = input.as_bytes();
    if parse_keyword(bytes, i, b"effects").is_none()
        && parse_keyword(bytes, i, b"requires").is_none()
    {
        return Ok(None);
    }

    // Word boundary after keyword.
    let kw_len = if parse_keyword(bytes, i, b"effects").is_some() {
        7
    } else {
        8
    };
    let after_kw = i + kw_len;
    if after_kw < bytes.len() && is_ident_continue(bytes[after_kw]) {
        return Ok(None);
    }

    // Find the end of the {...} block for this decl without trying to parse arbitrary nesting.
    let mut j = after_kw;
    j = skip_ws(bytes, j);
    if j >= bytes.len() || bytes[j] != b'{' {
        return Ok(None);
    }

    let mut depth = 0usize;
    while j < bytes.len() {
        match bytes[j] {
            b'{' => depth += 1,
            b'}' => {
                depth = depth.saturating_sub(1);
                if depth == 0 {
                    j += 1;
                    break;
                }
            }
            _ => {}
        }
        j += 1;
    }
    if depth != 0 {
        return Err(ParseError {
            message: "unterminated declaration".to_string(),
        });
    }

    let decl_str = &input[i..j];
    let decl = parse_decl(decl_str)?;
    Ok(Some((decl, j - i)))
}
