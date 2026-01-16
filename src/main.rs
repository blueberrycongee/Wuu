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
            let input = std::fs::read_to_string(path)?;
            let formatted = wuu::syntax::format_source(&input)?;
            if check && formatted != input {
                anyhow::bail!("file is not formatted");
            }
            print!("{formatted}");
        }
        Command::Check { path } => {
            let input = std::fs::read_to_string(path)?;
            let _ = wuu::syntax::format_source(&input)?;
        }
    }

    Ok(())
}
