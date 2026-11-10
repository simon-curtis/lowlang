use zed_extension_api::{self as zed, LanguageServerId, Result};

struct LowLang;

impl zed::Extension for LowLang {
    fn new() -> Self {
        LowLang
    }

    fn language_server_command(
        &mut self,
        _language_server_id: &LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<zed::Command> {
        let command = worktree
            .which("low")
            .ok_or_else(|| "low was not found on PATH".to_string())?;
        Ok(zed::Command {
            command,
            args: vec!["lsp".to_string()],
            env: worktree.shell_env(),
        })
    }
}

zed::register_extension!(LowLang);
