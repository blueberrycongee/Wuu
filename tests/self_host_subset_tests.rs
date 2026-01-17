use std::fs;
use std::path::Path;

#[test]
fn self_host_subset_doc_is_complete() {
    let path = Path::new("docs/wuu-lang/SELF_HOST_SUBSET.md");
    assert!(path.exists(), "missing SELF_HOST_SUBSET.md");

    let content = fs::read_to_string(path).expect("read SELF_HOST_SUBSET.md failed");

    for heading in [
        "# Self-Hosting Subset",
        "## Syntax Subset",
        "## Standard Library Subset",
        "## Forbidden Features",
        "## Review Checklist",
    ] {
        assert!(
            content.contains(heading),
            "missing required heading: {heading}"
        );
    }

    let mut checklist_total = 0usize;
    for line in content.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with("- [") {
            checklist_total += 1;
            assert!(
                !trimmed.starts_with("- [ ]"),
                "checklist item is not completed: {trimmed}"
            );
        }
    }

    assert!(checklist_total >= 6, "expected at least 6 checklist items");
}
