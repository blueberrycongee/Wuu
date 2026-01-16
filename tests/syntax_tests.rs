use pretty_assertions::assert_eq;

use wuu::syntax::{DeclKind, format_decl, format_source, format_source_bytes, parse_decl};

// Edge cases to keep stable in v0 prototype:
// - extra whitespace
// - trailing commas
// - empty effects/requires sets
// - invalid identifiers/paths are rejected

#[test]
fn parse_effects_decl_with_trailing_comma() {
    let decl = parse_decl("effects{Net.Http,Store.Kv,}").unwrap();
    assert_eq!(decl.kind, DeclKind::Effects);
    assert_eq!(decl.items, vec!["Net.Http", "Store.Kv"]);
}

#[test]
fn format_effects_decl_is_canonical() {
    let decl = parse_decl("effects{Net.Http,Store.Kv,}").unwrap();
    assert_eq!(format_decl(&decl), "effects { Net.Http, Store.Kv }");
}

#[test]
fn parse_requires_decl_is_canonicalized_as_pairs() {
    let decl = parse_decl("requires{net:http,store:kv,}").unwrap();
    assert_eq!(decl.kind, DeclKind::Requires);
    assert_eq!(decl.items, vec!["net:http", "store:kv"]);
    assert_eq!(format_decl(&decl), "requires { net:http, store:kv }");
}

#[test]
fn parse_decl_rejects_invalid_path() {
    let err = parse_decl("effects{Net..Http}").unwrap_err();
    assert!(err.to_string().contains("invalid"));
}

#[test]
fn format_source_rewrites_decls_only() {
    let input = r#"fn f()effects{Net.Http,Store.Kv,}{return;}"#;
    let out = format_source(input).unwrap();
    assert_eq!(out, r#"fn f()effects { Net.Http, Store.Kv }{return;}"#);
}

#[test]
fn format_source_ignores_effects_in_string() {
    let input = r#"let s = "effects{Net.Http,Store.Kv,}";effects{Net.Http,Store.Kv,}"#;
    let out = format_source(input).unwrap();
    assert_eq!(
        out,
        r#"let s = "effects{Net.Http,Store.Kv,}";effects { Net.Http, Store.Kv }"#
    );
}

#[test]
fn format_source_ignores_effects_in_comment() {
    let input = "// effects{Net.Http,Store.Kv,}\neffects{Net.Http,Store.Kv,}";
    let out = format_source(input).unwrap();
    assert_eq!(
        out,
        "// effects{Net.Http,Store.Kv,}\neffects { Net.Http, Store.Kv }"
    );
}

#[test]
fn format_source_rejects_invalid_utf8() {
    let input = vec![0xff, 0xfe, b'e'];
    let err = format_source_bytes(&input).unwrap_err();
    assert!(err.to_string().contains("invalid utf-8"));
}
