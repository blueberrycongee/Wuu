use std::path::Path;

use wuu::evidence::{collect_evidence, run_benches, run_examples, run_properties};

#[test]
fn evidence_blocks_execute() {
    let dir = Path::new("docs/wuu-lang");
    let evidence = collect_evidence(dir).expect("collect evidence failed");

    assert!(
        !evidence.examples.is_empty(),
        "expected at least 1 example block"
    );
    assert!(
        !evidence.properties.is_empty(),
        "expected at least 1 property block"
    );
    assert!(
        !evidence.benches.is_empty(),
        "expected at least 1 bench block"
    );

    run_examples(&evidence).expect("example blocks failed");
    run_properties(&evidence).expect("property blocks failed");
    run_benches(&evidence).expect("bench blocks failed");
}
