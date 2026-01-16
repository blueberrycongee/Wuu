use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};

use wuu::log::{
    EffectCall, EffectResult, LogRecord, Outcome, StepEnd, StepStart, WorkflowEnd, WorkflowStart,
    encode_log,
};
use wuu::parser::parse_module;
use wuu::replay::replay_workflow;

fn write_log(records: &[LogRecord]) -> PathBuf {
    static COUNTER: AtomicUsize = AtomicUsize::new(0);
    let dir = std::env::temp_dir().join("wuu-replay-tests");
    let _ = fs::create_dir_all(&dir);
    let id = COUNTER.fetch_add(1, Ordering::SeqCst);
    let path = dir.join(format!("log-{id}.cbor"));
    let bytes = encode_log(records).expect("encode log failed");
    fs::write(&path, bytes).expect("write log failed");
    path
}

#[test]
fn replay_ok() {
    let module_path = PathBuf::from("tests/replay/ok.wuu");
    let source = fs::read_to_string(&module_path).expect("read module failed");
    let module = parse_module(&source).expect("parse failed");

    let log = vec![
        LogRecord::WorkflowStart(WorkflowStart {
            workflow_name: "run".to_string(),
            args: vec![],
            run_id: "run-1".to_string(),
        }),
        LogRecord::StepStart(StepStart {
            step_id: 1,
            step_name: "fetch".to_string(),
            attempt: 1,
        }),
        LogRecord::EffectCall(EffectCall {
            call_id: 10,
            capability: "Net.Http".to_string(),
            op: "get".to_string(),
            input: vec![0x80],
        }),
        LogRecord::EffectResult(EffectResult {
            call_id: 10,
            outcome: Outcome::Ok,
            output: vec![],
        }),
        LogRecord::StepEnd(StepEnd {
            step_id: 1,
            outcome: Outcome::Ok,
        }),
        LogRecord::WorkflowEnd(WorkflowEnd {
            outcome: Outcome::Ok,
        }),
    ];

    let log_path = write_log(&log);
    replay_workflow(&module, "run", &log_path).expect("replay failed");
}

#[test]
fn replay_detects_mismatch() {
    let module_path = PathBuf::from("tests/replay/ok.wuu");
    let source = fs::read_to_string(&module_path).expect("read module failed");
    let module = parse_module(&source).expect("parse failed");

    let log = vec![
        LogRecord::WorkflowStart(WorkflowStart {
            workflow_name: "run".to_string(),
            args: vec![],
            run_id: "run-1".to_string(),
        }),
        LogRecord::StepStart(StepStart {
            step_id: 1,
            step_name: "fetch".to_string(),
            attempt: 1,
        }),
        LogRecord::EffectCall(EffectCall {
            call_id: 10,
            capability: "Net.Http".to_string(),
            op: "post".to_string(),
            input: vec![0x80],
        }),
        LogRecord::EffectResult(EffectResult {
            call_id: 10,
            outcome: Outcome::Ok,
            output: vec![],
        }),
        LogRecord::StepEnd(StepEnd {
            step_id: 1,
            outcome: Outcome::Ok,
        }),
        LogRecord::WorkflowEnd(WorkflowEnd {
            outcome: Outcome::Ok,
        }),
    ];

    let log_path = write_log(&log);
    let err = replay_workflow(&module, "run", &log_path).unwrap_err();
    assert!(err.to_string().contains("effect call mismatch"));
}
