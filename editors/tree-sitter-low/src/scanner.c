#include "tree_sitter/parser.h"

#include <stdbool.h>

enum TokenType {
  NEWLINE,
};

void *tree_sitter_low_external_scanner_create(void) { return NULL; }

void tree_sitter_low_external_scanner_destroy(void *payload) {}

unsigned tree_sitter_low_external_scanner_serialize(void *payload, char *buffer) { return 0; }

void tree_sitter_low_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {}

static bool blank(int32_t c) {
  return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v';
}

static bool continues(TSLexer *lexer) {
  for (;;) {
    while (blank(lexer->lookahead)) {
      lexer->advance(lexer, false);
    }
    if (lexer->lookahead != '/') {
      break;
    }
    lexer->advance(lexer, false);
    if (lexer->lookahead != '/') {
      return false;
    }
    while (lexer->lookahead != '\n' && !lexer->eof(lexer)) {
      lexer->advance(lexer, false);
    }
  }
  if (lexer->lookahead == '|') {
    lexer->advance(lexer, false);
    return lexer->lookahead == '>';
  }
  if (lexer->lookahead == '.') {
    lexer->advance(lexer, false);
    return lexer->lookahead != '.';
  }
  return false;
}

bool tree_sitter_low_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
  if (!valid_symbols[NEWLINE]) {
    return false;
  }
  while (lexer->lookahead == ' ' || lexer->lookahead == '\t' || lexer->lookahead == '\r') {
    lexer->advance(lexer, true);
  }
  if (lexer->lookahead != '\n') {
    return false;
  }
  lexer->advance(lexer, false);
  lexer->mark_end(lexer);
  if (continues(lexer)) {
    return false;
  }
  lexer->result_symbol = NEWLINE;
  return true;
}
