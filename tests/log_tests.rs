use wuu::log::{
    EffectCall, EffectResult, LogRecord, Outcome, StepEnd, StepStart, WorkflowEnd, WorkflowStart,
    decode_record, encode_record,
};

#[test]
fn log_roundtrip_records() {
    let records = vec![
        LogRecord::WorkflowStart(WorkflowStart {
            workflow_name: "ingest".to_string(),
            args: vec![1, 2, 3],
            run_id: "run-1".to_string(),
        }),
        LogRecord::StepStart(StepStart {
            step_id: 7,
            step_name: "fetch".to_string(),
            attempt: 2,
        }),
        LogRecord::EffectCall(EffectCall {
            call_id: 42,
            capability: "Net.Http".to_string(),
            op: "get".to_string(),
            input: vec![0x01, 0x02],
        }),
        LogRecord::EffectResult(EffectResult {
            call_id: 42,
            outcome: Outcome::Ok,
            output: vec![0x0a],
        }),
        LogRecord::StepEnd(StepEnd {
            step_id: 7,
            outcome: Outcome::Ok,
        }),
        LogRecord::WorkflowEnd(WorkflowEnd {
            outcome: Outcome::Ok,
        }),
    ];

    for record in records {
        let encoded = encode_record(&record).expect("encode failed");
        let decoded = decode_record(&encoded).expect("decode failed");
        assert_eq!(decoded, record);

        let encoded_again = encode_record(&decoded).expect("encode again failed");
        assert_eq!(encoded_again, encoded);
    }
}

#[test]
fn log_decoder_ignores_unknown_fields() {
    let record = LogRecord::StepEnd(StepEnd {
        step_id: 9,
        outcome: Outcome::Err,
    });
    let encoded = encode_record(&record).expect("encode failed");

    let mut value: serde_cbor::Value = serde_cbor::from_slice(&encoded).expect("decode to value");
    let map = match &mut value {
        serde_cbor::Value::Map(map) => map,
        _ => panic!("expected cbor map"),
    };
    map.insert(
        serde_cbor::Value::Integer(99),
        serde_cbor::Value::Text("extra".to_string()),
    );

    let encoded_extra = serde_cbor::to_vec(&value).expect("encode with extra");
    let decoded = decode_record(&encoded_extra).expect("decode failed");
    assert_eq!(decoded, record);
}
