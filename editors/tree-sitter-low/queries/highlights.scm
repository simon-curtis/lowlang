(comment) @comment

(string) @string

(char) @string

(escape_sequence) @string.escape

(brace_escape) @string.escape

(integer) @number

(float) @number.float

(boolean) @boolean

((identifier) @constant.builtin
  (#any-of? @constant.builtin "none" "null"))

((identifier) @constructor
  (#match? @constructor "^[A-Z]"))

((type_identifier) @type
  (#not-match? @type "^(i8|i16|i32|i64|u8|u16|u32|u64|f32|f64|bool|str|ptr|chan|void)$"))

((type_identifier) @type.builtin
  (#match? @type.builtin "^(i8|i16|i32|i64|u8|u16|u32|u64|f32|f64|bool|str|ptr|chan|void)$"))

(field_identifier) @property

(function_declaration
  name: (identifier) @function)

(extern_function
  name: (identifier) @function)

(call_expression
  function: (identifier) @function.call
  (#match? @function.call "^[a-z_]")
  (#not-any-of? @function.call "print" "new" "push" "delete" "bytes" "cstr" "str" "view" "load" "store" "pin" "unpin" "embed" "sqrt" "floor" "exp" "close" "sleep" "wait_readable" "wait_writable"))

(call_expression
  function: (identifier) @function.builtin
  (#any-of? @function.builtin "print" "new" "push" "delete" "bytes" "cstr" "str" "view" "load" "store" "pin" "unpin" "embed" "sqrt" "floor" "exp" "close" "sleep" "wait_readable" "wait_writable"))

(call_expression
  function: (field_expression
    field: (field_identifier) @function.call))

(call_expression
  function: (index_expression
    value: (identifier) @function.call)
  (#match? @function.call "^[a-z_]"))

(parameter
  name: (identifier) @variable.parameter)

(closure_parameter
  name: (identifier) @variable.parameter)

(module_path
  (identifier) @module)

(import_declaration
  alias: (identifier) @module)

(scoped_type_identifier
  module: (identifier) @module)

(select_case
  (call_expression
    function: (identifier) @keyword
    (#eq? @keyword "after")))

"fn" @keyword.function

[
  "let"
  "var"
  "struct"
  "enum"
  "extern"
  "defer"
  "spawn"
  "select"
  "using"
  "drop"
] @keyword

"import" @keyword.import

[
  "if"
  "else"
  "match"
] @keyword.conditional

[
  "for"
  "in"
] @keyword.repeat

[
  "break"
  "continue"
] @keyword.repeat

(label_identifier) @label

"return" @keyword.return

[
  "fail"
  "catch"
] @keyword.exception

[
  (visibility)
  (mutable)
  "mut"
] @keyword.modifier

(variadic) @punctuation.special

[
  "="
  "+="
  "-="
  "*="
  "/="
  "%="
  "&="
  "|="
  "^="
  "<<="
  ">>="
  "+"
  "-"
  "*"
  "/"
  "%"
  "&"
  "|"
  "^"
  "~"
  "<<"
  ">>"
  "!"
  "&&"
  "||"
  "=="
  "!="
  "<"
  "<="
  ">"
  ">="
  "??"
  "?"
  "<-"
  "->"
  "=>"
  ".."
] @operator

[
  "("
  ")"
  "["
  "]"
  "{"
  "}"
] @punctuation.bracket

[
  ","
  "."
  ":"
] @punctuation.delimiter
