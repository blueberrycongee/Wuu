# Evidence Gates

This file defines executable evidence blocks used by the test harness.

Format:

- `example: <name>` followed by a `wuu` code block and `expect: <value>`.
- `property: <name>` followed by a `wuu` code block and one or more
  `case: [args] => expect` lines.
- `bench: <name>` followed by a `wuu` code block and `iterations`/`max_ms`.

---

example: return-int
```wuu
fn main() -> Int {
    return 1;
}
```
expect: 1

property: bool-identity
```wuu
fn id(x: Bool) -> Bool {
    return x;
}

fn main(x: Bool) -> Bool {
    return id(x);
}
```
case: [true] => true
case: [false] => false

bench: tiny-return
```wuu
fn main() -> Int {
    return 1;
}
```
iterations: 1000
max_ms: 200
backend: interpreter
