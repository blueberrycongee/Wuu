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
        #[arg(long, conflicts_with = "check")]
        write: bool,
    },
    Lex {
        path: PathBuf,
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
            write,
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
            } else if write {
                std::fs::write(&path, formatted)?;
            } else {
                print!("{formatted}");
            }
        }
        Command::Lex { path, stage1 } => {
            let input = std::fs::read(&path)?;
            let output = if stage1 {
                lex_stage1(&input)?
            } else {
                lex_stage0(&input)?
            };
            print!("{output}");
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

fn lex_stage0(input: &[u8]) -> anyhow::Result<String> {
    let source = std::str::from_utf8(input).map_err(|_| anyhow::anyhow!("invalid utf-8"))?;
    let tokens = wuu::lexer::lex(source)?;
    Ok(format_tokens(source, &tokens))
}

fn lex_stage1(input: &[u8]) -> anyhow::Result<String> {
    let source = std::str::from_utf8(input).map_err(|_| anyhow::anyhow!("invalid utf-8"))?;
    let lexer_path = PathBuf::from("selfhost/lexer.wuu");
    let lexer_source = std::fs::read_to_string(&lexer_path)
        .map_err(|err| anyhow::anyhow!("failed to read {}: {err}", lexer_path.display()))?;
    let module = wuu::parser::parse_module(&lexer_source)?;
    wuu::typeck::check_module(&module)?;
    let value = run_entry_with_args(&module, "lex", vec![Value::String(source.to_string())])?;
    match value {
        Value::String(output) => Ok(output),
        other => Err(anyhow::anyhow!(
            "stage1 lexer returned non-string value: {other:?}"
        )),
    }
}

fn format_tokens(source: &str, tokens: &[wuu::lexer::Token]) -> String {
    let mut lines = Vec::new();
    for token in tokens {
        match token.kind {
            wuu::lexer::TokenKind::Whitespace | wuu::lexer::TokenKind::Comment => continue,
            wuu::lexer::TokenKind::Keyword(_) => {
                let text = escape_token(token_text(source, token));
                lines.push(format!("Keyword {text}"));
            }
            wuu::lexer::TokenKind::Ident => {
                let text = escape_token(token_text(source, token));
                lines.push(format!("Ident {text}"));
            }
            wuu::lexer::TokenKind::Number => {
                let text = escape_token(token_text(source, token));
                lines.push(format!("Number {text}"));
            }
            wuu::lexer::TokenKind::StringLiteral => {
                let text = escape_token(token_text(source, token));
                lines.push(format!("StringLiteral {text}"));
            }
            wuu::lexer::TokenKind::Punct(ch) => {
                lines.push(format!("Punct {ch}"));
            }
            wuu::lexer::TokenKind::Other => {
                let text = escape_token(token_text(source, token));
                lines.push(format!("Other {text}"));
            }
        }
    }
    lines.join("\n")
}

fn token_text<'a>(source: &'a str, token: &wuu::lexer::Token) -> &'a str {
    &source[token.span.start..token.span.end]
}

fn escape_token(text: &str) -> String {
    text.replace('\\', "\\\\")
        .replace('\n', "\\n")
        .replace('\r', "\\r")
        .replace('\t', "\\t")
}
