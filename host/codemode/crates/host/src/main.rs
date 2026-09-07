use anyhow::Context;
use clap::Parser;

#[derive(Debug, Parser)]
struct Cli {
    /// Transport endpoint: `stdio` or `stdio://`.
    #[arg(
        long,
        value_name = "URL",
        default_value = codex_code_mode_host::DEFAULT_LISTEN_URL
    )]
    listen: String,
}

#[tokio::main(flavor = "current_thread")]
async fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();
    tracing_subscriber::fmt()
        .with_writer(std::io::stderr)
        .with_ansi(false)
        .with_max_level(tracing::Level::INFO)
        .init();
    codex_code_mode_host::run_main(&cli.listen)
        .await
        .context("code-mode host failed")
}
