# SQL Statement Selection Feature for DBEE

This document describes the semicolon-based SQL statement selection feature implemented for the DBEE plugin.

## Overview

The SQL statement selection feature allows users to execute individual SQL statements within a multi-statement file without manually selecting them. The parser intelligently identifies statement boundaries using semicolons while respecting SQL syntax rules for comments and quoted strings.

## Features Implemented

### 1. Run SQL Statement Under Cursor
- **Action**: `run_statement`
- **Default Mapping**: `BS` (normal mode)
- **Function**: Executes the SQL statement where the cursor is positioned

### 2. Visual Selection of SQL Statement
- **Action**: `select_statement`  
- **Default Mapping**: `SS` (normal mode)
- **Function**: Visually selects the SQL statement under cursor

### 3. SQL Text Objects
- **Inner Statement**: `is` (operator-pending and visual modes)
- **Around Statement**: `as` (operator-pending and visual modes)
- **Usage Examples**:
  - `vis` - Visually select inner SQL statement
  - `das` - Delete around SQL statement (including semicolon)
  - `cis` - Change inner SQL statement
  - `yis` - Yank inner SQL statement

## Implementation Details

### SQL Parser Features

The SQL statement parser handles the following scenarios:

#### 1. **Basic Statement Separation**
```sql
SELECT * FROM users;
SELECT * FROM orders;
-- Cursor on line 1 executes first statement
-- Cursor on line 2 executes second statement
```

#### 2. **Quoted String Handling**
```sql
SELECT 'Hello; World' FROM test;
SELECT name FROM users WHERE note = 'Contains ; semicolon';
-- Semicolons inside quotes are ignored
```

#### 3. **Line Comments**
```sql
SELECT * FROM users -- This ; is ignored
WHERE active = 1;
-- Comments are preserved in the statement
```

#### 4. **Block Comments**
```sql
SELECT * /* Comment with ; semicolon */ FROM users;
UPDATE /* Another ; comment */ users SET status = 1;
-- Semicolons in block comments are ignored
```

#### 5. **Multi-line Statements**
```sql
SELECT 
  u.id,
  u.name,
  u.email
FROM users u
WHERE u.active = 1;

-- The entire multi-line statement is executed as one unit
```

#### 6. **Statements Without Trailing Semicolons**
```sql
SELECT * FROM users
-- Last statement without semicolon is still recognized
```

### Key Implementation Files

#### 1. **lua/dbee/utils.lua**
- `get_sql_statement_at_cursor(bufnr, cursor_row)` - Core parser function
- `get_sql_statement_bounds(bufnr, cursor_row)` - Returns line boundaries
- `select_sql_statement_at_cursor()` - Visual selection function
- `select_inner_sql_statement()` - Text object for inner statement
- `select_around_sql_statement()` - Text object for around statement
- `setup_sql_text_objects()` - Initializes text objects for SQL filetypes

#### 2. **lua/dbee/ui/editor/init.lua**
- `run_statement` action - Executes statement under cursor
- `select_statement` action - Visually selects statement under cursor

#### 3. **lua/dbee/config.lua**
- Default key mappings for new actions

#### 4. **lua/dbee.lua**
- Integration of text object setup in main setup function

## Usage Examples

### Basic Usage
```lua
-- In your SQL file:
SELECT * FROM users WHERE active = 1;
SELECT COUNT(*) FROM orders;
UPDATE users SET last_login = NOW() WHERE id = 123;

-- Place cursor anywhere in first statement and press BS
-- Only "SELECT * FROM users WHERE active = 1;" will be executed
```

### Visual Selection
```lua
-- Place cursor in a statement and press SS
-- The entire statement will be visually selected
```

### Text Objects
```lua
-- With cursor in a SQL statement:
vis  -- Select inner statement (without leading/trailing whitespace)
vas  -- Select around statement (including semicolon if present)
das  -- Delete the entire statement
cis  -- Change the inner statement content
```

## Configuration

### Custom Key Mappings
Users can customize the key mappings in their DBEE configuration:

```lua
require("dbee").setup({
  editor = {
    mappings = {
      -- Default mappings
      { key = "BB", mode = "v", action = "run_selection" },
      { key = "BB", mode = "n", action = "run_file" },
      
      -- New SQL statement mappings (customizable)
      { key = "BS", mode = "n", action = "run_statement" },
      { key = "SS", mode = "n", action = "select_statement" },
      
      -- You can change these to any keys you prefer:
      -- { key = "<C-e>", mode = "n", action = "run_statement" },
      -- { key = "gs", mode = "n", action = "select_statement" },
    },
  },
})
```

### Disabling Text Objects
If you prefer not to use the SQL text objects, you can disable them by not calling the setup:

```lua
-- The text objects are automatically set up during dbee.setup()
-- To disable, you would need to modify the source or override the autocmd
```

## Benefits

### 1. **Improved Workflow**
- No need to manually select SQL statements
- Execute statements with a single keypress
- Natural vim-like text objects for SQL

### 2. **Syntax Awareness**
- Correctly handles complex SQL with comments and strings
- Respects SQL syntax rules for semicolon placement
- Works with various SQL dialects

### 3. **Vim Integration**
- Follows vim conventions for text objects
- Integrates seamlessly with existing vim motions
- Works with other vim plugins and commands

### 4. **Database Agnostic**
- Works with all databases supported by DBEE
- No database-specific parsing rules
- Consistent behavior across different SQL dialects

## Testing

The implementation includes comprehensive tests for various SQL scenarios:

```bash
# Run the test suite (when in nvim)
:luafile test_sql_parser.lua
```

Test cases cover:
- Simple single statements
- Multiple statements
- Quoted strings with semicolons
- Line and block comments
- Multi-line statements
- Statements without trailing semicolons
- Edge cases with whitespace

## Limitations and Considerations

### 1. **Parser Limitations**
- Basic SQL parsing - doesn't handle all edge cases of SQL grammar
- Focused on semicolon boundaries rather than full SQL parsing
- May not handle very complex nested structures

### 2. **Performance**
- Parser runs on entire buffer content
- Should be fast for typical SQL files
- May have performance impact on very large files

### 3. **SQL Dialect Support**
- Designed for standard SQL syntax
- Should work with most SQL dialects
- Some dialect-specific features may not be recognized

## Future Enhancements

Potential improvements that could be added:

1. **Enhanced SQL Grammar Support**
   - Better handling of stored procedures
   - Support for dialect-specific statement separators
   - Recognition of SQL blocks (BEGIN/END)

2. **Performance Optimizations**
   - Incremental parsing for large files
   - Caching of statement boundaries
   - Lazy evaluation of statements

3. **Advanced Features**
   - Statement highlighting
   - Statement folding
   - Syntax validation

4. **Configuration Options**
   - Customizable statement separators
   - File-type specific behavior
   - Performance tuning options

## Troubleshooting

### Common Issues

#### Statement Not Recognized
- **Cause**: Cursor is in whitespace between statements
- **Solution**: Place cursor inside the actual SQL statement text

#### Wrong Statement Selected
- **Cause**: Complex string literals or comments confusing the parser
- **Solution**: Use visual selection (`BB` in visual mode) as fallback

#### Text Objects Not Working
- **Cause**: Not in a SQL filetype buffer
- **Solution**: Set filetype with `:set filetype=sql`

#### Performance Issues
- **Cause**: Very large SQL files
- **Solution**: Consider breaking large files into smaller ones

### Debug Information

To debug parser behavior:
```lua
-- Check what statement is detected
local utils = require("dbee.utils")
local cursor_pos = vim.api.nvim_win_get_cursor(0)
local row = cursor_pos[1] - 1
local statement = utils.get_sql_statement_at_cursor(0, row)
print("Detected statement:", statement)
```

## Conclusion

The SQL statement selection feature significantly improves the DBEE workflow by providing intelligent, syntax-aware statement execution. The implementation follows vim conventions and integrates seamlessly with existing DBEE functionality while providing powerful new capabilities for SQL development.