use wuu::interpreter::{Value, run_entry_with_args};
use wuu::parser::parse_module;
use wuu::typeck::check_module as check_types;

mod selfhost_support;

fn load_stdlib_module(extra: &str) -> wuu::ast::Module {
    let stdlib = selfhost_support::load_stdlib_source();
    let source = format!("{stdlib}\n\n{extra}");
    let module = parse_module(&source).expect("parse stdlib module failed");
    check_types(&module).expect("typecheck stdlib module failed");
    module
}

#[test]
fn stdlib_pair_helpers_work() {
    let module = load_stdlib_module(
        "fn pair_left_test() -> String { let value = pair(\"left\", \"right\"); return pair_left(value); }\n\
         fn pair_right_test() -> String { let value = pair(\"left\", \"right\"); return pair_right(value); }\n",
    );

    let left =
        run_entry_with_args(&module, "pair_left_test", vec![]).expect("pair_left_test failed");
    assert_eq!(left.to_string(), "left");

    let right =
        run_entry_with_args(&module, "pair_right_test", vec![]).expect("pair_right_test failed");
    assert_eq!(right.to_string(), "right");
}

#[test]
fn stdlib_split_line_and_escape_work() {
    let module = load_stdlib_module(
        "fn split_left() -> String { let pair = split_line(\"one\\ntwo\"); return pair_left(pair); }\n\
         fn split_right() -> String { let pair = split_line(\"one\\ntwo\"); return pair_right(pair); }\n\
         fn escape_tab() -> String { return escape(\"\\t\"); }\n",
    );

    let left = run_entry_with_args(&module, "split_left", vec![]).expect("split_left failed");
    assert_eq!(left.to_string(), "one");

    let right = run_entry_with_args(&module, "split_right", vec![]).expect("split_right failed");
    assert_eq!(right.to_string(), "two");

    let escaped = run_entry_with_args(&module, "escape_tab", vec![]).expect("escape_tab failed");
    assert_eq!(escaped.to_string(), "\\t");
}

#[test]
fn stdlib_list_and_option_helpers_work() {
    let module = load_stdlib_module(
        "fn list_head_test() -> String {\n\
             let list = list_cons(\"a\", list_cons(\"b\", list_nil()));\n\
             return list_head(list);\n\
         }\n\
         fn list_tail_empty() -> Bool {\n\
             let list = list_cons(\"a\", list_nil());\n\
             let tail = list_tail(list);\n\
             return list_is_empty(tail);\n\
         }\n\
         fn option_none_test() -> Bool { return option_is_none(option_none()); }\n\
         fn option_unwrap_test() -> String { return option_unwrap(option_some(\"v\")); }\n",
    );

    let head =
        run_entry_with_args(&module, "list_head_test", vec![]).expect("list_head_test failed");
    assert_eq!(head.to_string(), "a");

    let tail_empty =
        run_entry_with_args(&module, "list_tail_empty", vec![]).expect("list_tail_empty failed");
    assert_eq!(tail_empty, Value::Bool(true));

    let none =
        run_entry_with_args(&module, "option_none_test", vec![]).expect("option_none_test failed");
    assert_eq!(none, Value::Bool(true));

    let unwrap = run_entry_with_args(&module, "option_unwrap_test", vec![])
        .expect("option_unwrap_test failed");
    assert_eq!(unwrap.to_string(), "v");
}
