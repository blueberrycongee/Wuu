use std::path::PathBuf;

use clap::{Parser, Subcommand};
use wuu::interpreter::{Value, run_entry_with_args};

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
        #[arg(long)]
        stage1: bool,
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
        Command::Fmt {
            path,
            check,
            stage1,
        } => {
            let input = std::fs::read(&path)?;
            let formatted = if stage1 {
                format_stage1(&input)?
            } else {
                wuu::format::format_source_bytes(&input)?
            };
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
            wuu::typeck::check_module(&module)?;
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

fn format_stage1(input: &[u8]) -> anyhow::Result<String> {
    let source = std::str::from_utf8(input).map_err(|_| anyhow::anyhow!("invalid utf-8"))?;
    let format_path = PathBuf::from("selfhost/format.wuu");
    let format_source = std::fs::read_to_string(&format_path)
        .map_err(|err| anyhow::anyhow!("failed to read {}: {err}", format_path.display()))?;
    let module = wuu::parser::parse_module(&format_source)?;
    wuu::typeck::check_module(&module)?;
    let value = run_entry_with_args(&module, "format", vec![Value::String(source.to_string())])?;
    match value {
        Value::String(output) => Ok(output),
        other => Err(anyhow::anyhow!(
            "stage1 formatter returned non-string value: {other:?}"
        )),
    }
}
