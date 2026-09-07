use anyhow::Result;

/// The standalone host always runs over stdio. Wuu removed the gRPC transport
/// from the vendored Codex host.
pub const DEFAULT_LISTEN_URL: &str = "stdio";

#[derive(Debug, Clone, Eq, PartialEq)]
pub(crate) enum ListenTransport {
    Stdio,
}

pub(crate) async fn run_transport(listen_url: &str) -> Result<()> {
    parse_listen_url(listen_url)?;
    crate::run_stdio().await
}

pub(crate) fn parse_listen_url(listen_url: &str) -> Result<ListenTransport> {
    if matches!(listen_url, "stdio" | "stdio://") {
        return Ok(ListenTransport::Stdio);
    }

    anyhow::bail!(
        "unsupported --listen URL `{listen_url}`; expected `stdio` or `stdio://`"
    );
}

#[cfg(test)]
#[path = "transport_tests.rs"]
mod tests;
