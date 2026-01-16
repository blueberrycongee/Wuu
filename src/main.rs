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
    Run {
        path: PathBuf,
        #[arg(long)]
        entry: String,
    },
    Workflow {
        #[command(subcommand)]
        cmd: WorkflowCommand,
    },
}

#[derive(Debug, Subcommand)]
enum WorkflowCommand {
    Replay {
        #[arg(long)]
        log: PathBuf,
        #[arg(long)]
        module: PathBuf,
        #[arg(long)]
        entry: String,
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
            let module = wuu::parser::parse_module_bytes(&input)?;
            wuu::effects::check_module(&module)?;
        }
        Command::Run { path, entry } => {
            let input = std::fs::read(&path)?;
            let module = wuu::parser::parse_module_bytes(&input)?;
            let value = wuu::interpreter::run_entry(&module, &entry)?;
            if !value.is_unit() {
                println!("{value}");
            }
        }
        Command::Workflow { cmd } => match cmd {
            WorkflowCommand::Replay { log, module, entry } => {
                let module_src = std::fs::read(&module)?;
                let module = wuu::parser::parse_module_bytes(&module_src)?;
                wuu::replay::replay_workflow(&module, &entry, &log)?;
            }
        },
    }

    Ok(())
}
