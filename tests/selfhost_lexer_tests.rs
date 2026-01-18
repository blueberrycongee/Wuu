use std::path::Path;

use wuu::ast::Item;
use wuu::effects::check_module as check_effects;
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

mod selfhost_support;

#[test]
fn selfhost_lexer_parses_and_typechecks() {
    let path = Path::new("selfhost/lexer.wuu");
    assert!(path.exists(), "missing selfhost/lexer.wuu");

    let source = selfhost_support::load_with_stdlib(path);
    let module = parse_module(&source).expect("parse failed");
    check_types(&module).expect("typecheck failed");
    check_effects(&module).expect("effect check failed");

    let has_lex = module.items.iter().any(|item| match item {
        Item::Fn(func) => func.name == "lex",
        _ => false,
    });
    assert!(has_lex, "expected a function named 'lex'");
}

#[test]
fn selfhost_parser_parses_and_typechecks() {
    let path = Path::new("selfhost/parser.wuu");
    assert!(path.exists(), "missing selfhost/parser.wuu");

    let source = selfhost_support::load_with_stdlib(path);
    let module = parse_module(&source).expect("parse failed");
    check_types(&module).expect("typecheck failed");
    check_effects(&module).expect("effect check failed");

    let has_parse = module.items.iter().any(|item| match item {
        Item::Fn(func) => func.name == "parse",
        _ => false,
    });
    assert!(has_parse, "expected a function named 'parse'");
}
