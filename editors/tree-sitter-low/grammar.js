const PREC = {
  closure: -1,
  pipe: 0,
  or: 1,
  and: 2,
  compare: 3,
  coalesce: 4,
  additive: 5,
  multiplicative: 6,
  unary: 7,
  postfix: 8,
  block_closure: 9,
};

const ASSIGNMENT = ['=', '+=', '-=', '*=', '/=', '%=', '&=', '|=', '^=', '<<=', '>>='];

const commaSep = (rule) => optional(seq(rule, repeat(seq(',', rule)), optional(',')));

const lines = ($, rule) => optional(seq(rule, repeat(seq($._newline, rule)), optional($._newline)));

const separated = ($, rule) => optional(seq(rule, repeat(seq(choice(',', $._newline), rule)), optional(choice(',', $._newline))));

module.exports = grammar({
  name: 'low',

  word: $ => $.identifier,

  extras: $ => [$.comment, /\s/],

  externals: $ => [$._newline],

  conflicts: $ => [
    [$._expression, $._type_name],
    [$._expression, $.scoped_type_identifier],
    [$.struct_literal],
    [$.call_expression],
    [$._type_name, $.closure_parameter],
    [$.function_type_parameters, $.closure_parameters],
    [$._type, $.struct_literal],
    [$._expression, $.field_pattern],
  ],

  supertypes: $ => [$._expression, $._statement, $._type, $._declaration],

  rules: {
    source_file: $ => seq(
      repeat($.import_declaration),
      repeat($._declaration),
    ),

    comment: _ => token(seq('//', /[^\n]*/)),

    identifier: _ => /[A-Za-z_][A-Za-z0-9_]*/,

    import_declaration: $ => seq(
      'import',
      optional(seq(field('alias', $.identifier), '=')),
      field('path', $.module_path),
    ),

    module_path: $ => seq($.identifier, repeat(seq('/', $.identifier))),

    _declaration: $ => choice(
      $.function_declaration,
      $.struct_declaration,
      $.enum_declaration,
      $.extern_block,
    ),

    visibility: _ => 'pub',

    function_declaration: $ => seq(
      optional($.visibility),
      'fn',
      field('name', $.identifier),
      optional(field('type_parameters', $.type_parameters)),
      field('parameters', $.parameters),
      optional(seq('->', field('result', $._type))),
      optional(seq('!', field('error', $._type))),
      field('body', $.block),
    ),

    type_parameters: $ => seq('[', commaSep($.type_parameter), ']'),

    type_parameter: $ => seq(
      field('name', alias($.identifier, $.type_identifier)),
      optional(seq(':', field('constraints', $.constraints))),
    ),

    constraints: $ => seq(
      alias($.identifier, $.type_identifier),
      repeat(seq('+', alias($.identifier, $.type_identifier))),
    ),

    parameters: $ => seq('(', commaSep($.parameter), ')'),

    parameter: $ => seq(
      optional($.mutable),
      field('name', $.identifier),
      ':',
      field('type', $._type),
    ),

    mutable: _ => 'mut',

    extern_block: $ => seq(
      'extern',
      field('library', $.string),
      '{',
      lines($, $.extern_function),
      '}',
    ),

    extern_function: $ => seq(
      optional($.visibility),
      'fn',
      field('name', $.identifier),
      field('parameters', $.extern_parameters),
      optional(seq('->', field('result', $._type))),
      optional(seq('=', field('symbol', $.string))),
    ),

    extern_parameters: $ => seq('(', commaSep(choice($.parameter, $.variadic)), ')'),

    variadic: _ => '...',

    struct_declaration: $ => seq(
      optional($.visibility),
      optional('extern'),
      'struct',
      field('name', alias($.identifier, $.type_identifier)),
      optional(field('type_parameters', $.type_parameters)),
      field('body', $.field_declaration_list),
    ),

    field_declaration_list: $ => seq('{', lines($, choice($.field_declaration, $.drop_declaration)), '}'),

    field_declaration: $ => seq(
      optional($.visibility),
      field('name', alias($.identifier, $.field_identifier)),
      ':',
      field('type', $._type),
      optional(seq('=', field('default', $._expression))),
    ),

    drop_declaration: $ => seq('drop', field('body', $._statement)),

    enum_declaration: $ => seq(
      optional($.visibility),
      'enum',
      field('name', alias($.identifier, $.type_identifier)),
      optional(field('type_parameters', $.type_parameters)),
      field('body', $.variant_list),
    ),

    variant_list: $ => seq('{', lines($, $.variant), '}'),

    variant: $ => seq(
      field('name', $.identifier),
      optional(field('fields', $.variant_fields)),
    ),

    variant_fields: $ => seq('(', commaSep($.variant_field), ')'),

    variant_field: $ => seq(
      optional(seq(field('name', alias($.identifier, $.field_identifier)), ':')),
      field('type', $._type),
    ),

    _type: $ => choice(
      $._type_name,
      $.generic_type,
      $.scoped_type_identifier,
      $.pointer_type,
      $.slice_type,
      $.map_type,
      $.function_type,
      $.optional_type,
    ),

    _type_name: $ => alias($.identifier, $.type_identifier),

    scoped_type_identifier: $ => seq(
      field('module', $.identifier),
      '.',
      field('name', alias($.identifier, $.type_identifier)),
    ),

    generic_type: $ => seq(
      field('type', choice($._type_name, $.scoped_type_identifier)),
      field('arguments', $.type_arguments),
    ),

    type_arguments: $ => seq('[', commaSep($._type), ']'),

    pointer_type: $ => prec(2, seq('*', field('type', $._type))),

    slice_type: $ => prec(2, seq('[', ']', field('element', $._type))),

    map_type: $ => seq('[', field('key', $._type), ':', field('value', $._type), ']'),

    optional_type: $ => prec(1, seq(field('type', $._type), '?')),

    function_type: $ => prec.right(seq(
      optional('extern'),
      'fn',
      field('parameters', $.function_type_parameters),
      optional(seq('->', field('result', $._type))),
      optional(seq('!', field('error', $._type))),
    )),

    function_type_parameters: $ => seq('(', commaSep(seq(optional($.mutable), $._type)), ')'),

    block: $ => seq('{', lines($, $._statement), '}'),

    _statement: $ => choice(
      $.let_declaration,
      $.using_declaration,
      $.assignment_statement,
      $.send_statement,
      $.for_statement,
      $.for_in_statement,
      $.defer_statement,
      $.spawn_statement,
      $.expression_statement,
    ),

    expression_statement: $ => $._expression,

    let_declaration: $ => choice(
      seq(
        choice('let', 'var'),
        field('name', $.identifier),
        optional(seq(':', field('type', $._type))),
        '=',
        field('value', $._expression),
      ),
      seq(
        choice('let', 'var'),
        field('pattern', $.struct_pattern),
        '=',
        field('value', $._expression),
      ),
    ),

    struct_pattern: $ => seq('{', separated($, $.field_pattern), '}'),

    field_pattern: $ => seq(
      field('name', alias($.identifier, $.field_identifier)),
      optional(seq(':', field('binding', choice($.identifier, $.struct_pattern)))),
    ),

    using_declaration: $ => seq(
      'using',
      optional('var'),
      field('name', $.identifier),
      optional(seq(':', field('type', $._type))),
      '=',
      field('value', $._expression),
    ),

    assignment_statement: $ => seq(
      field('left', $._expression),
      field('operator', choice(...ASSIGNMENT)),
      field('right', $._expression),
    ),

    send_statement: $ => seq(
      field('channel', $._expression),
      '<-',
      field('value', $._expression),
    ),

    label: $ => seq(field('name', alias($.identifier, $.label_identifier)), ':'),

    for_statement: $ => seq(
      optional($.label),
      'for',
      optional(field('condition', $._expression)),
      field('body', $.block),
    ),

    for_in_statement: $ => seq(
      optional($.label),
      'for',
      optional(seq(field('key', $.identifier), ',')),
      field('value', choice($.identifier, $.struct_pattern)),
      'in',
      field('iterable', $._expression),
      optional(seq('..', field('end', $._expression))),
      field('body', $.block),
    ),

    break_expression: $ => prec.right(seq('break', optional(field('label', alias($.identifier, $.label_identifier))))),

    continue_expression: $ => prec.right(seq('continue', optional(field('label', alias($.identifier, $.label_identifier))))),

    return_expression: $ => prec.right(seq('return', optional(field('value', $._expression)))),

    fail_expression: $ => prec.right(seq('fail', field('value', $._expression))),

    defer_statement: $ => seq('defer', field('body', $._statement)),

    spawn_statement: $ => seq('spawn', field('call', $._expression)),

    _expression: $ => choice(
      $.identifier,
      $.integer,
      $.float,
      $.char,
      $.string,
      $.boolean,
      $.parenthesized_expression,
      $.unary_expression,
      $.binary_expression,
      $.call_expression,
      $.field_expression,
      $.index_expression,
      $.slice_expression,
      $.try_expression,
      $.catch_expression,
      $.or_expression,
      $.struct_literal,
      $.slice_literal,
      $.map_literal,
      $.closure_expression,
      $.if_expression,
      $.match_expression,
      $.select_expression,
      $.break_expression,
      $.continue_expression,
      $.return_expression,
      $.fail_expression,
      $.block,
    ),

    integer: _ => token(choice(
      /0x[0-9A-Fa-f_]+/,
      /0b[01_]+/,
      /0o[0-7_]+/,
      /[0-9][0-9_]*/,
    )),

    float: _ => token(choice(
      /[0-9][0-9_]*\.[0-9][0-9_]*([eE][+-]?[0-9_]+)?/,
      /[0-9][0-9_]*[eE][+-]?[0-9_]+/,
    )),

    escape_sequence: _ => token.immediate(/\\[ntr0\\"']/),

    char: $ => seq(
      '\'',
      choice($.escape_sequence, token.immediate(/[^'\\\n]/)),
      token.immediate('\''),
    ),

    string: $ => seq(
      '"',
      repeat(choice(
        $.string_content,
        $.escape_sequence,
        $.brace_escape,
        $.interpolation,
      )),
      token.immediate('"'),
    ),

    string_content: _ => token.immediate(prec(1, /[^"\\{}\n]+/)),

    brace_escape: _ => token.immediate(choice('{{', '}}')),

    interpolation: $ => seq(
      token.immediate('{'),
      field('value', $._expression),
      optional(seq(',', field('width', $.interpolation_width))),
      optional(seq(':', field('format', $.interpolation_format))),
      '}',
    ),

    interpolation_width: _ => /-?[0-9]+/,

    interpolation_format: _ => /[^}\s"]+/,

    boolean: _ => choice('true', 'false'),

    parenthesized_expression: $ => seq('(', $._expression, ')'),

    unary_expression: $ => prec(PREC.unary, seq(
      field('operator', choice('-', '!', '*', '~', '<-')),
      field('operand', $._expression),
    )),

    binary_expression: $ => {
      const table = [
        [PREC.pipe, '|>'],
        [PREC.or, '||'],
        [PREC.and, '&&'],
        [PREC.compare, choice('==', '!=', '<', '<=', '>', '>=')],
        [PREC.additive, choice('+', '-', '|', '^')],
        [PREC.multiplicative, choice('*', '/', '%', '&', '<<', '>>')],
      ];
      const left = table.map(([precedence, operator]) => prec.left(precedence, seq(
        field('left', $._expression),
        field('operator', operator),
        field('right', $._expression),
      )));
      const coalesce = prec.right(PREC.coalesce, seq(
        field('left', $._expression),
        field('operator', '??'),
        field('right', $._expression),
      ));
      return choice(...left, coalesce);
    },

    call_expression: $ => prec(PREC.postfix, seq(
      field('function', $._expression),
      optional(field('type_arguments', prec.dynamic(1, $.type_arguments))),
      field('arguments', $.arguments),
      optional(field('children', prec.dynamic(-1, $.children))),
    )),

    children: $ => seq('{', separated($, $._expression), '}'),

    arguments: $ => seq('(', commaSep(choice($._expression, $.mutable_argument)), ')'),

    mutable_argument: $ => seq('mut', $._expression),

    field_expression: $ => prec(PREC.postfix, seq(
      field('value', $._expression),
      '.',
      field('field', alias($.identifier, $.field_identifier)),
    )),

    index_expression: $ => prec(PREC.postfix, seq(
      field('value', $._expression),
      '[',
      field('index', $._expression),
      ']',
    )),

    slice_expression: $ => prec(PREC.postfix, seq(
      field('value', $._expression),
      '[',
      optional(field('start', $._expression)),
      '..',
      optional(field('end', $._expression)),
      ']',
    )),

    try_expression: $ => prec(PREC.postfix, seq($._expression, '?')),

    catch_expression: $ => prec.right(PREC.postfix, seq(
      field('value', $._expression),
      'catch',
      field('handler', choice($.match_block, $._expression)),
    )),

    or_expression: $ => prec.right(PREC.postfix, seq(
      field('value', $._expression),
      'or',
      field('fallback', $._expression),
    )),

    struct_literal: $ => prec(0, seq(
      field('name', choice($._type_name, $.scoped_type_identifier, $.generic_type)),
      field('body', $.field_initializer_list),
      optional(field('children', prec.dynamic(-1, $.children))),
    )),

    field_initializer_list: $ => seq('{', separated($, $.field_initializer), '}'),

    field_initializer: $ => seq(
      field('name', alias($.identifier, $.field_identifier)),
      optional(seq(':', field('value', $._expression))),
    ),

    slice_literal: $ => seq('[', separated($, $._expression), ']'),

    map_literal: $ => seq('[', choice(':', seq($.map_entry, repeat(seq(choice(',', $._newline), $.map_entry)), optional(choice(',', $._newline)))), ']'),

    map_entry: $ => seq(field('key', $._expression), ':', field('value', $._expression)),

    closure_expression: $ => choice(
      prec.dynamic(1, prec(PREC.block_closure, seq(
        'fn',
        field('parameters', $.closure_parameters),
        optional(seq('->', field('result', $._type))),
        optional(seq('!', field('error', $._type))),
        field('body', $.block),
      ))),
      prec.right(PREC.closure, seq(
        'fn',
        field('parameters', $.closure_parameters),
        field('body', $._expression),
      )),
    ),

    closure_parameters: $ => seq('(', commaSep($.closure_parameter), ')'),

    closure_parameter: $ => seq(
      optional($.mutable),
      field('name', $.identifier),
      optional(seq(':', field('type', $._type))),
    ),

    if_expression: $ => prec.right(seq(
      'if',
      field('condition', $._expression),
      field('consequence', $.block),
      optional(seq('else', field('alternative', choice($.block, $.if_expression)))),
    )),

    match_expression: $ => seq(
      'match',
      field('value', $._expression),
      field('body', $.match_block),
    ),

    match_block: $ => prec(1, seq('{', separated($, $.match_arm), '}')),

    match_arm: $ => seq(
      field('pattern', choice($.pattern, $.test_pattern)),
      '=>',
      field('value', $._expression),
    ),

    test_pattern: $ => {
      const table = [
        [PREC.compare, choice('==', '!=', '<', '<=', '>', '>=')],
        [PREC.coalesce, '??'],
        [PREC.additive, choice('+', '-', '|', '^')],
        [PREC.multiplicative, choice('*', '/', '%', '&', '<<', '>>')],
      ];
      const opening = table.map(([precedence, operator]) => prec.left(precedence, seq(
        field('operator', operator),
        field('right', $._expression),
      )));
      const extended = table.map(([precedence, operator]) => prec.left(precedence, seq(
        field('left', $.test_pattern),
        field('operator', operator),
        field('right', $._expression),
      )));
      const joined = [[PREC.or, '||'], [PREC.and, '&&']].map(([precedence, operator]) => prec.left(precedence, seq(
        field('left', $.test_pattern),
        field('operator', operator),
        field('right', $.test_pattern),
      )));
      return choice(...opening, ...extended, ...joined);
    },

    pattern: $ => prec(1, seq(
      field('variant', $.identifier),
      optional(seq('(', commaSep(field('binding', $.identifier)), ')')),
    )),

    select_expression: $ => seq('select', '{', lines($, $.select_case), '}'),

    select_case: $ => seq(
      choice(
        'else',
        $._expression,
        seq(field('name', $.identifier), '=', field('receive', $._expression)),
        seq(field('channel', $._expression), '<-', field('value', $._expression)),
      ),
      '=>',
      field('body', $._expression),
    ),
  },
});
