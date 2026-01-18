use std::collections::BTreeSet;

use wuu::typeck::intrinsic_names;

fn load_allowed_intrinsics() -> BTreeSet<String> {
    let content =
        std::fs::read_to_string("docs/wuu-lang/HOST_INTRINSICS.md").expect("read inventory");
    let mut allowed = BTreeSet::new();
    for line in content.lines() {
        let trimmed = line.trim();
        if let Some(name) = trimmed
            .strip_prefix("- `")
            .and_then(|rest| rest.strip_suffix('`'))
            && name.starts_with("__")
        {
            allowed.insert(name.to_string());
        }
    }
    allowed
}

#[test]
fn host_intrinsics_inventory_is_exhaustive() {
    let allowed = load_allowed_intrinsics();
    let actual: BTreeSet<_> = intrinsic_names().into_iter().collect();
    assert_eq!(
        allowed, actual,
        "HOST_INTRINSICS.md must list all intrinsics"
    );
}
