use crate::ast::{
    Block, Contract, ContractKind, EffectsDecl, Expr, FnDecl, IfStmt, Item, LetStmt, LoopStmt,
    Module, Param, Path, ReturnStmt, StepStmt, TypeRef, WorkflowDecl,
};
use crate::error::ParseError;
use crate::lexer::{Keyword, Token, TokenKind, lex};
use crate::span::Span;

pub fn parse_module(source: &str) -> Result<Module, ParseError> {
    let mut parser = Parser::new(source)?;
    parser.parse_module()
}

pub fn parse_module_bytes(input: &[u8]) -> Result<Module, ParseError> {
    let source = std::str::from_utf8(input).map_err(|_| ParseError::new("invalid utf-8"))?;
    parse_module(source)
}

struct Parser<'a> {
    source: &'a str,
    tokens: Vec<Token>,
    cursor: usize,
}

impl<'a> Parser<'a> {
    fn new(source: &'a str) -> Result<Self, ParseError> {
        let tokens = lex(source)?
            .into_iter()
            .filter(|token| !matches!(token.kind, TokenKind::Whitespace | TokenKind::Comment))
            .collect();
        Ok(Self {
            source,
            tokens,
            cursor: 0,
        })
    }

    fn parse_module(&mut self) -> Result<Module, ParseError> {
        let mut items = Vec::new();
        while !self.is_eof() {
            items.push(self.parse_item()?);
        }
        Ok(Module { items })
    }

    fn parse_item(&mut self) -> Result<Item, ParseError> {
        match self.peek_kind() {
            Some(TokenKind::Keyword(Keyword::Fn)) => Ok(Item::Fn(self.parse_fn()?)),
            Some(TokenKind::Keyword(Keyword::Workflow)) => {
                Ok(Item::Workflow(self.parse_workflow()?))
            }
            _ => Err(self.error_current("expected 'fn' or 'workflow'")),
        }
    }

    fn parse_fn(&mut self) -> Result<FnDecl, ParseError> {
        self.expect_keyword(Keyword::Fn)?;
        let name = self.expect_ident()?;
        let params = self.parse_params()?;
        let return_type = if self.consume_arrow() {
            Some(self.parse_type()?)
        } else {
            None
        };
        let effects = if self.peek_is_effects_decl() {
            Some(self.parse_effects_decl()?)
        } else {
            None
        };
        let contracts = self.parse_contracts()?;
        let body = self.parse_block(false)?;
        Ok(FnDecl {
            name,
            params,
            return_type,
            effects,
            contracts,
            body,
        })
    }

    fn parse_workflow(&mut self) -> Result<WorkflowDecl, ParseError> {
        self.expect_keyword(Keyword::Workflow)?;
        let name = self.expect_ident()?;
        let params = self.parse_params()?;
        let return_type = if self.consume_arrow() {
            Some(self.parse_type()?)
        } else {
            None
        };
        let effects = if self.peek_is_effects_decl() {
            Some(self.parse_effects_decl()?)
        } else {
            None
        };
        let contracts = self.parse_contracts()?;
        let body = self.parse_block(true)?;
        Ok(WorkflowDecl {
            name,
            params,
            return_type,
            effects,
            contracts,
            body,
        })
    }

    fn parse_params(&mut self) -> Result<Vec<Param>, ParseError> {
        self.expect_punct('(')?;
        let mut params = Vec::new();
        if self.consume_punct(')') {
            return Ok(params);
        }

        loop {
            let name = self.expect_ident()?;
            if !self.consume_punct(':') {
                return Err(self.error_current("expected ':' in parameter"));
            }
            let ty = self.parse_type()?;
            params.push(Param { name, ty: Some(ty) });

            if self.consume_punct(',') {
                if self.consume_punct(')') {
                    break;
                }
                continue;
            }
            self.expect_punct(')')?;
            break;
        }

        Ok(params)
    }

    fn parse_type(&mut self) -> Result<TypeRef, ParseError> {
        let path = self.parse_path()?;
        Ok(TypeRef::Path(path))
    }

    fn parse_effects_decl(&mut self) -> Result<EffectsDecl, ParseError> {
        if self.consume_keyword(Keyword::Effects) {
            self.expect_punct('{')?;
            let mut items = Vec::new();
            if self.consume_punct('}') {
                return Ok(EffectsDecl::Effects(items));
            }
            loop {
                let path = self.parse_path()?;
                items.push(path);
                if self.consume_punct(',') {
                    if self.consume_punct('}') {
                        break;
                    }
                    continue;
                }
                self.expect_punct('}')?;
                break;
            }
            Ok(EffectsDecl::Effects(items))
        } else if self.consume_keyword(Keyword::Requires) {
            self.expect_punct('{')?;
            let mut items = Vec::new();
            if self.consume_punct('}') {
                return Ok(EffectsDecl::Requires(items));
            }
            loop {
                let left = self.expect_ident()?;
                self.expect_punct(':')?;
                let right = self.expect_ident()?;
                items.push((left, right));
                if self.consume_punct(',') {
                    if self.consume_punct('}') {
                        break;
                    }
                    continue;
                }
                self.expect_punct('}')?;
                break;
            }
            Ok(EffectsDecl::Requires(items))
        } else {
            Err(self.error_current("expected 'effects' or 'requires'"))
        }
    }

    fn parse_contracts(&mut self) -> Result<Vec<Contract>, ParseError> {
        let mut contracts = Vec::new();
        loop {
            let kind = match self.peek_kind() {
                Some(TokenKind::Keyword(Keyword::Pre)) => ContractKind::Pre,
                Some(TokenKind::Keyword(Keyword::Post)) => ContractKind::Post,
                Some(TokenKind::Keyword(Keyword::Invariant)) => ContractKind::Invariant,
                _ => break,
            };
            self.advance();
            self.expect_punct(':')?;
            let expr = self.parse_expr()?;
            contracts.push(Contract { kind, expr });
        }
        Ok(contracts)
    }

    fn parse_block(&mut self, in_workflow: bool) -> Result<Block, ParseError> {
        self.expect_punct('{')?;
        let mut stmts = Vec::new();
        while !self.peek_is_punct('}') {
            if self.is_eof() {
                return Err(self.error_at(self.eof_span(), "unterminated block"));
            }
            stmts.push(self.parse_stmt(in_workflow)?);
        }
        self.expect_punct('}')?;
        Ok(Block { stmts })
    }

    fn parse_stmt(&mut self, in_workflow: bool) -> Result<crate::ast::Stmt, ParseError> {
        match self.peek_kind() {
            Some(TokenKind::Keyword(Keyword::Let)) => {
                self.advance();
                let name = self.expect_ident()?;
                let ty = if self.consume_punct(':') {
                    Some(self.parse_type()?)
                } else {
                    None
                };
                self.expect_punct('=')?;
                let expr = self.parse_expr()?;
                self.expect_punct(';')?;
                Ok(crate::ast::Stmt::Let(LetStmt { name, ty, expr }))
            }
            Some(TokenKind::Keyword(Keyword::Return)) => {
                self.advance();
                let expr = if self.peek_is_punct(';') {
                    None
                } else {
                    Some(self.parse_expr()?)
                };
                self.expect_punct(';')?;
                Ok(crate::ast::Stmt::Return(ReturnStmt { expr }))
            }
            Some(TokenKind::Keyword(Keyword::If)) => {
                self.advance();
                let condition = self.parse_expr()?;
                let then_block = self.parse_block(in_workflow)?;
                let else_block = if self.consume_keyword(Keyword::Else) {
                    Some(self.parse_block(in_workflow)?)
                } else {
                    None
                };
                Ok(crate::ast::Stmt::If(IfStmt {
                    condition,
                    then_block,
                    else_block,
                }))
            }
            Some(TokenKind::Keyword(Keyword::Loop)) => {
                self.advance();
                let body = self.parse_block(in_workflow)?;
                Ok(crate::ast::Stmt::Loop(LoopStmt { body }))
            }
            Some(TokenKind::Keyword(Keyword::Step)) => {
                if !in_workflow {
                    return Err(self.error_current("step is only valid inside workflow"));
                }
                self.advance();
                let label = self.expect_string_literal()?;
                let body = self.parse_block(in_workflow)?;
                Ok(crate::ast::Stmt::Step(StepStmt { label, body }))
            }
            _ => {
                let expr = self.parse_expr()?;
                self.expect_punct(';')?;
                Ok(crate::ast::Stmt::Expr(expr))
            }
        }
    }

    fn parse_expr(&mut self) -> Result<Expr, ParseError> {
        match self.peek_kind() {
            Some(TokenKind::Ident) => {
                let path = self.parse_path()?;
                if self.consume_punct('(') {
                    let args = self.parse_call_args()?;
                    Ok(Expr::Call { callee: path, args })
                } else if path.len() == 1 {
                    Ok(Expr::Ident(path[0].clone()))
                } else {
                    Ok(Expr::Path(path))
                }
            }
            Some(TokenKind::StringLiteral) => Ok(Expr::String(self.expect_string_literal()?)),
            _ => Err(self.error_current("expected expression")),
        }
    }

    fn parse_call_args(&mut self) -> Result<Vec<Expr>, ParseError> {
        let mut args = Vec::new();
        if self.consume_punct(')') {
            return Ok(args);
        }

        loop {
            args.push(self.parse_expr()?);
            if self.consume_punct(',') {
                if self.consume_punct(')') {
                    break;
                }
                continue;
            }
            self.expect_punct(')')?;
            break;
        }

        Ok(args)
    }

    fn parse_path(&mut self) -> Result<Path, ParseError> {
        let mut parts = Vec::new();
        parts.push(self.expect_ident()?);
        while self.consume_punct('.') {
            parts.push(self.expect_ident()?);
        }
        Ok(parts)
    }

    fn expect_ident(&mut self) -> Result<String, ParseError> {
        match self.peek() {
            Some(Token {
                kind: TokenKind::Ident,
                span,
            }) => {
                let text = self.token_text(*span).to_string();
                self.advance();
                Ok(text)
            }
            _ => Err(self.error_current("expected identifier")),
        }
    }

    fn expect_string_literal(&mut self) -> Result<String, ParseError> {
        match self.peek() {
            Some(Token {
                kind: TokenKind::StringLiteral,
                span,
            }) => {
                let text = self.token_text(*span).to_string();
                self.advance();
                Ok(unquote(&text))
            }
            _ => Err(self.error_current("expected string literal")),
        }
    }

    fn expect_keyword(&mut self, kw: Keyword) -> Result<(), ParseError> {
        if self.consume_keyword(kw) {
            Ok(())
        } else {
            Err(self.error_current("unexpected keyword"))
        }
    }

    fn consume_keyword(&mut self, kw: Keyword) -> bool {
        matches!(self.peek_kind(), Some(TokenKind::Keyword(found)) if found == kw) && {
            self.advance();
            true
        }
    }

    fn peek_is_effects_decl(&self) -> bool {
        matches!(
            self.peek_kind(),
            Some(TokenKind::Keyword(Keyword::Effects | Keyword::Requires))
        )
    }

    fn expect_punct(&mut self, ch: char) -> Result<(), ParseError> {
        if self.consume_punct(ch) {
            Ok(())
        } else {
            Err(self.error_current(&format!("expected '{ch}'")))
        }
    }

    fn consume_punct(&mut self, ch: char) -> bool {
        matches!(self.peek_kind(), Some(TokenKind::Punct(found)) if found == ch) && {
            self.advance();
            true
        }
    }

    fn peek_is_punct(&self, ch: char) -> bool {
        matches!(self.peek_kind(), Some(TokenKind::Punct(found)) if found == ch)
    }

    fn consume_arrow(&mut self) -> bool {
        matches!(self.peek_kind(), Some(TokenKind::Punct('-')))
            && matches!(self.peek_next_kind(), Some(TokenKind::Punct('>')))
            && {
                self.advance();
                self.advance();
                true
            }
    }

    fn peek(&self) -> Option<&Token> {
        self.tokens.get(self.cursor)
    }

    fn peek_kind(&self) -> Option<TokenKind> {
        self.peek().map(|token| token.kind.clone())
    }

    fn peek_next_kind(&self) -> Option<TokenKind> {
        self.tokens
            .get(self.cursor + 1)
            .map(|token| token.kind.clone())
    }

    fn advance(&mut self) {
        self.cursor = self.cursor.saturating_add(1);
    }

    fn is_eof(&self) -> bool {
        self.cursor >= self.tokens.len()
    }

    fn token_text(&self, span: Span) -> &str {
        &self.source[span.start..span.end]
    }

    fn eof_span(&self) -> Span {
        let end = self.source.len();
        Span { start: end, end }
    }

    fn error_current(&self, message: &str) -> ParseError {
        let span = self
            .peek()
            .map(|token| token.span)
            .unwrap_or_else(|| self.eof_span());
        self.error_at(span, message)
    }

    fn error_at(&self, span: Span, message: &str) -> ParseError {
        ParseError::with_span(message, span, self.source)
    }
}

fn unquote(text: &str) -> String {
    if text.len() >= 2 {
        text[1..text.len() - 1].to_string()
    } else {
        String::new()
    }
}
