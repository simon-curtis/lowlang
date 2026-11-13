(comment) @comment

(string) @string

(char) @string

(escape_sequence) @string.escape

(brace_escape) @string.escape

(interpolation_width) @number

(interpolation_format) @string.escape

(integer) @number

(float) @number.float

(boolean) @boolean

((identifier) @constant.builtin
  (#any-of? @constant.builtin "none" "null"))

((identifier) @constructor
  (#match? @constructor "^[A-Z]"))

((type_identifier) @type
  (#not-match? @type "^(i64|i32|u64|u32|u16|u8|f64|f32|bool|str|ptr|chan|u8x16|u8x32|u16x8|u16x16|i32x4|i32x8|u32x4|u32x8|i64x2|i64x4|u64x2|u64x4|f32x4|f32x8|f64x2|f64x4)$"))

((type_identifier) @type.builtin
  (#match? @type.builtin "^(i64|i32|u64|u32|u16|u8|f64|f32|bool|str|ptr|chan|u8x16|u8x32|u16x8|u16x16|i32x4|i32x8|u32x4|u32x8|i64x2|i64x4|u64x2|u64x4|f32x4|f32x8|f64x2|f64x4)$"))

(field_identifier) @property

(function_declaration
  name: (identifier) @function)

(extern_function
  name: (identifier) @function)

(const_declaration
  name: (identifier) @constant)

(call_expression
  function: (identifier) @function.call
  (#match? @function.call "^[a-z_]")
  (#not-any-of? @function.call "print" "new" "push" "pop" "truncate" "delete" "bytes" "cstr" "view" "load" "store" "pin" "unpin" "embed" "sqrt" "floor" "exp" "close" "sleep" "wait_readable" "wait_writable"))

(call_expression
  function: (identifier) @function.builtin
  (#any-of? @function.builtin "print" "new" "push" "pop" "truncate" "delete" "bytes" "cstr" "view" "load" "store" "pin" "unpin" "embed" "sqrt" "floor" "exp" "close" "sleep" "wait_readable" "wait_writable"))

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
  "const"
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
  "or"
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
