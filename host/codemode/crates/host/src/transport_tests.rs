use pretty_assertions::assert_eq;

use super::ListenTransport;
use super::parse_listen_url;

#[test]
fn parse_listen_url_accepts_stdio_transports() {
    assert_eq!(
        parse_listen_url("stdio").expect("stdio listen URL should parse"),
        ListenTransport::Stdio
    );
    assert_eq!(
        parse_listen_url("stdio://").expect("stdio URL should parse"),
        ListenTransport::Stdio
    );
}

#[test]
fn parse_listen_url_rejects_non_stdio_transports() {
    let grpc = parse_listen_url("grpc://127.0.0.1:9000")
        .expect_err("gRPC transport was removed from the Wuu host");
    assert!(grpc.to_string().contains("unsupported --listen URL"));

    let http = parse_listen_url("http://127.0.0.1:9000")
        .expect_err("HTTP is not a listen transport");
    assert!(http.to_string().contains("unsupported --listen URL"));
}
