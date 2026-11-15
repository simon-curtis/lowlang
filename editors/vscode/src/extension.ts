import { commands, ExtensionContext, window, workspace } from 'vscode';
import { LanguageClient, LanguageClientOptions, ServerOptions } from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

function createClient(): LanguageClient {
  const command = workspace.getConfiguration('lowlang').get<string>('server.path', 'low');
  const server: ServerOptions = { command, args: ['lsp'] };
  const options: LanguageClientOptions = {
    documentSelector: [{ scheme: 'file', language: 'low' }],
  };
  return new LanguageClient('lowlang', 'Low', server, options);
}

async function start(): Promise<void> {
  const next = createClient();
  client = next;
  try {
    await next.start();
  } catch (error) {
    client = undefined;
    const reason = error instanceof Error ? error.message : String(error);
    window.showErrorMessage(`Low language server failed to start: ${reason}`);
  }
}

async function stop(): Promise<void> {
  const running = client;
  client = undefined;
  if (running) {
    await running.stop();
  }
}

export async function activate(context: ExtensionContext): Promise<void> {
  const restart = commands.registerCommand('lowlang.restartServer', async () => {
    await stop();
    await start();
  });
  const toggleMain = commands.registerCommand('lowlang.toggleMain', async () => {
    const editor = window.activeTextEditor;
    if (!client || !editor || editor.document.languageId !== 'low') {
      return;
    }
    try {
      await client.sendRequest('workspace/executeCommand', {
        command: 'low.toggleMain',
        arguments: [editor.document.uri.toString()],
      });
    } catch (error) {
      const reason = error instanceof Error ? error.message : String(error);
      window.showErrorMessage(reason);
    }
  });
  const reconfigure = workspace.onDidChangeConfiguration(async (event) => {
    if (event.affectsConfiguration('lowlang.server.path')) {
      await stop();
      await start();
    }
  });
  context.subscriptions.push(restart, toggleMain, reconfigure);
  await start();
}

export function deactivate(): Promise<void> {
  return stop();
}
