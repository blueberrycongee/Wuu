use std::path::PathBuf;

use clap::{Parser, Subcommand};

#[derive(Debug, Parser)]
#[command(name = "wuu")]
#[command(about = "Wuu toolchain prototype", long_about = None)]
struct Cli {
    #[command(subcommand)]
    cmd: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    Fmt {
        path: PathBuf,
        #[arg(long)]
        check: bool,
    },
    Check {
        path: PathBuf,
    },
}

fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();

    match cli.cmd {
        Command::Fmt { path, check } => {
            let input = std::fs::read(&path)?;
            let formatted = wuu::format::format_source_bytes(&input)?;
            if check {
                let input_str =
                    std::str::from_utf8(&input).map_err(|_| anyhow::anyhow!("invalid utf-8"))?;
                if formatted != input_str {
                    anyhow::bail!("file is not formatted");
                }
            }
            print!("{formatted}");
        }
        Command::Check { path } => {
            let input = std::fs::read(&path)?;
            let _ = wuu::parser::parse_module_bytes(&input)?;
        }
    }

    Ok(())
}
