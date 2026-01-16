use std::fmt;

use crate::span::Span;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ParseError {
    pub message: String,
    pub span: Option<Span>,
    pub line: Option<usize>,
    pub column: Option<usize>,
}

impl ParseError {
    pub fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
            span: None,
            line: None,
            column: None,
        }
    }

    pub fn with_span(message: impl Into<String>, span: Span, source: &str) -> Self {
        let (line, column) = line_col(source, span.start);
        Self {
            message: message.into(),
            span: Some(span),
            line: Some(line),
            column: Some(column),
        }
    }
}

impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match (self.line, self.column) {
            (Some(line), Some(column)) => write!(f, "{line}:{column}: {}", self.message),
            _ => write!(f, "{}", self.message),
        }
    }
}

impl std::error::Error for ParseError {}

fn line_col(source: &str, offset: usize) -> (usize, usize) {
    let mut line = 1usize;
    let mut column = 1usize;
    let mut last_index = 0usize;

    for (index, ch) in source.char_indices() {
        if index >= offset {
            break;
        }
        if ch == '\n' {
            line += 1;
            column = 1;
        } else {
            column += 1;
        }
        last_index = index;
    }

    if offset > last_index && offset >= source.len() {
        column = source
            .chars()
            .rev()
            .take_while(|&ch| ch != '\n')
            .count()
            .saturating_add(1);
    }

    (line, column)
}
