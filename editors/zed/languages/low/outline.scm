(function_declaration
  (visibility)? @context
  "fn" @context
  name: (_) @name) @item

(struct_declaration
  (visibility)? @context
  "struct" @context
  name: (_) @name) @item

(enum_declaration
  (visibility)? @context
  "enum" @context
  name: (_) @name) @item

(field_declaration
  name: (_) @name) @item

(variant
  name: (_) @name) @item

(extern_block
  "extern" @context
  library: (_) @name) @item

(extern_function
  "fn" @context
  name: (_) @name) @item
