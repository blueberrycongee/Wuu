use wuu::ast::{EffectsDecl, Item};
use wuu::parser::parse_module;

#[test]
fn parse_fn_with_effects_and_contracts() {
    let source = r#"
fn greet(name: String) -> String
effects { Net.Http, Store.Kv, }
pre: name
post: name
{
    let result = name;
    return result;
}
"#;

    let module = parse_module(source).unwrap();
    assert_eq!(module.items.len(), 1);

    match &module.items[0] {
        Item::Fn(func) => {
            assert_eq!(func.name, "greet");
            assert_eq!(func.params.len(), 1);
            assert!(matches!(func.effects, Some(EffectsDecl::Effects(_))));
            assert_eq!(func.contracts.len(), 2);
            assert_eq!(func.body.stmts.len(), 2);
        }
        other => panic!("expected fn item, got {other:?}"),
    }
}

#[test]
fn parse_workflow_with_step_and_loop() {
    let source = r#"
workflow run() {
    step "one" {
        loop {
            return;
        }
    }
}
"#;

    let module = parse_module(source).unwrap();
    assert_eq!(module.items.len(), 1);

    match &module.items[0] {
        Item::Workflow(flow) => {
            assert_eq!(flow.name, "run");
            assert_eq!(flow.body.stmts.len(), 1);
        }
        other => panic!("expected workflow item, got {other:?}"),
    }
}

#[test]
fn parse_rejects_step_outside_workflow() {
    let source = r#"fn f() { step "x" { return; } }"#;
    let err = parse_module(source).unwrap_err();

    assert!(err.message.contains("step"));
    assert_eq!(err.line, Some(1));
    assert_eq!(err.column, Some(10));
}
