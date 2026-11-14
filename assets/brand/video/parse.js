const keywords = new Set(["fn", "let", "var", "return", "if", "else", "for", "in", "match", "enum", "struct", "import"]);
const literalTypes = { string: "str", number: "i64" };

export const irRules = [
  [/\s+/y, "operator"],
  [/"(?:[^"\\]|\\.)*"/y, "string"],
  [/#-?\d+/y, "number"],
  [/\bv\d+\b/y, "module"],
  [/\b(?:fn|ret|params|results|slots|preds|const|string|runtime|call|br|jmp|phi)\b/y, "keyword"],
  [/\bb\d+\b/y, "type"],
  [/-?\d+/y, "number"],
  [/[A-Za-z_][\w.]*/y, "function"],
  [/./y, "operator"],
];

const operandRules = [
  [/\s+/y, "operator"],
  [/\b(?:r(?:[abcd]x|[sd]i|[sb]p|\d+[dwb]?)|e(?:[abcd]x|[sd]i|[sb]p))\b/y, "type"],
  [/-?\d+/y, "number"],
  [/[A-Za-z_.][\w.]*/y, "function"],
  [/./y, "operator"],
];

export function lex(source) {
  const tokens = [];
  for (const match of source.matchAll(/("(?:[^"\\]|\\.)*")|(\d+(?:\.\d+)?)|([A-Za-z_]\w*)|(\S)/g)) {
    const [text, string, number, word] = match;
    let kind = "operator";
    if (string) {
      kind = "string";
    } else if (number) {
      kind = "number";
    } else if (word) {
      kind = keywords.has(word) ? "keyword" : "name";
    }
    tokens.push({ text, kind });
  }
  tokens.forEach((token, index) => {
    if (token.kind === "name" && tokens[index + 1]?.text === "(") {
      token.kind = "function";
    }
  });
  return tokens;
}

class Parser {
  constructor(tokens) {
    this.tokens = tokens;
    this.at = 0;
    this.nodes = [];
  }

  peek() {
    return this.tokens[this.at]?.text;
  }

  expect(text) {
    if (this.peek() !== text) {
      throw new Error(`hello.low: expected ${text} at token ${this.at}, found ${this.peek() ?? "the end"}`);
    }
    return this.at++;
  }

  identifier() {
    const token = this.tokens[this.at];
    if (token?.kind !== "function" && token?.kind !== "name") {
      throw new Error(`hello.low: expected a name at token ${this.at}`);
    }
    return this.at++;
  }

  add(kind, tokens, parent, type) {
    const node = { kind, tokens, parent, type, children: [] };
    parent?.children.push(node);
    this.nodes.push(node);
    return node;
  }

  program() {
    while (this.at < this.tokens.length) {
      this.function();
    }
  }

  function() {
    const keyword = this.expect("fn");
    const name = this.identifier();
    const declaration = this.add("fn", [keyword, name], null, "fn()");
    const open = this.expect("(");
    const close = this.expect(")");
    this.add("params", [open, close], declaration, "()");
    this.block(declaration);
  }

  block(parent) {
    const open = this.expect("{");
    const block = this.add("block", [open], parent, "void");
    while (this.peek() !== "}") {
      this.call(block);
    }
    block.tokens.push(this.expect("}"));
  }

  call(parent) {
    const name = this.identifier();
    const open = this.expect("(");
    const call = this.add("call", [name, open], parent, "void");
    while (this.peek() !== ")") {
      this.literal(call);
      if (this.peek() === ",") {
        call.tokens.push(this.expect(","));
      }
    }
    call.tokens.push(this.expect(")"));
  }

  literal(parent) {
    const token = this.tokens[this.at];
    const type = literalTypes[token?.kind];
    if (!type) {
      throw new Error(`hello.low: the video draws only string and number arguments, found ${token?.text ?? "the end"}`);
    }
    this.add(token.kind, [this.at++], parent, type);
  }
}

export function parse(tokens) {
  const parser = new Parser(tokens);
  parser.program();
  return parser.nodes;
}

export function highlight(line, rules) {
  const spans = [];
  let at = 0;
  while (at < line.length) {
    for (const [pattern, kind] of rules) {
      pattern.lastIndex = at;
      const found = pattern.exec(line);
      if (found) {
        spans.push({ text: found[0], kind });
        at += found[0].length;
        break;
      }
    }
  }
  return spans;
}

export function instruction(line) {
  const space = line.search(/\s/);
  if (space < 0) {
    return [{ text: line, kind: "keyword" }];
  }
  const operands = highlight(line.slice(space), operandRules);
  return [{ text: line.slice(0, space), kind: "keyword" }, ...operands];
}

export function mainAssembly(assembly) {
  const lines = assembly.split("\n");
  const start = lines.indexOf("fn_main:");
  if (start < 0) {
    throw new Error("low asm printed no fn_main");
  }
  const body = [];
  for (const raw of lines.slice(start + 1)) {
    const text = raw.trim();
    if (raw.startsWith("\t") && text) {
      body.push(text);
    }
    if (text === "ret") {
      return body;
    }
  }
  throw new Error("fn_main has no ret");
}
