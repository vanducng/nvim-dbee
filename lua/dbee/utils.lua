local M = {}

-- private variable with registered onces
---@type table<string, boolean>
local used_onces = {}

---@param id string unique id of this singleton bool
---@return boolean
function M.once(id)
  id = id or ""

  if used_onces[id] then
    return false
  end

  used_onces[id] = true

  return true
end

-- Get cursor range of current selection
---@return integer start row
---@return integer start column
---@return integer end row
---@return integer end column
function M.visual_selection()
  -- return to normal mode ('< and '> become available only after you exit visual mode)
  local key = vim.api.nvim_replace_termcodes("<esc>", true, false, true)
  vim.api.nvim_feedkeys(key, "x", false)

  local _, srow, scol, _ = unpack(vim.fn.getpos("'<"))
  local _, erow, ecol, _ = unpack(vim.fn.getpos("'>"))
  if ecol > 200000 then
    ecol = 20000
  end
  if srow < erow or (srow == erow and scol <= ecol) then
    return srow - 1, scol - 1, erow - 1, ecol
  else
    return erow - 1, ecol - 1, srow - 1, scol
  end
end

---@param level "info"|"warn"|"error"
---@param message string
---@param subtitle? string
function M.log(level, message, subtitle)
  -- log level
  local l = vim.log.levels.OFF
  if level == "info" then
    l = vim.log.levels.INFO
  elseif level == "warn" then
    l = vim.log.levels.WARN
  elseif level == "error" then
    l = vim.log.levels.ERROR
  end

  -- subtitle
  if subtitle then
    subtitle = "[" .. subtitle .. "]:"
  else
    subtitle = ""
  end
  vim.notify(subtitle .. " " .. message, l, { title = "nvim-dbee" })
end

-- Gets keys of a map and sorts them by name
---@param obj table<string, any> map-like table
---@return string[]
function M.sorted_keys(obj)
  local keys = {}
  for k, _ in pairs(obj) do
    table.insert(keys, k)
  end
  table.sort(keys)
  return keys
end

-- create an autocmd that is associated with a window rather than a buffer.
---@param events string[]
---@param winid integer
---@param opts table<string, any>
local function create_window_autocmd(events, winid, opts)
  opts = opts or {}
  if not events or not winid or not opts.callback then
    return
  end

  local cb = opts.callback

  opts.callback = function(event)
    -- remove autocmd if window is closed
    if not vim.api.nvim_win_is_valid(winid) then
      vim.api.nvim_del_autocmd(event.id)
      return
    end

    local wid = vim.fn.bufwinid(event.buf or -1)
    if wid ~= winid then
      return
    end
    cb(event)
  end

  vim.api.nvim_create_autocmd(events, opts)
end

-- create an autocmd just once in a single place in code.
-- If opts hold a "window" key, autocmd is defined per window rather than a buffer.
-- If window and buffer are provided, this results in an error.
---@param events string[] events list as defined in nvim api
---@param opts table<string, any> options as in api
function M.create_singleton_autocmd(events, opts)
  if opts.window and opts.buffer then
    error("cannot register autocmd for buffer and window at the same time")
  end

  local caller_info = debug.getinfo(2)
  if not caller_info or not caller_info.name or not caller_info.currentline then
    error("could not determine function caller")
  end

  if
    not M.once(
      "autocmd_singleton_"
        .. caller_info.name
        .. caller_info.currentline
        .. tostring(opts.window)
        .. tostring(opts.buffer)
    )
  then
    -- already configured
    return
  end

  if opts.window then
    local window = opts.window
    opts.window = nil
    create_window_autocmd(events, window, opts)
    return
  end

  vim.api.nvim_create_autocmd(events, opts)
end

local random_charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

--- Generate a random string
---@return string _ random string of 10 characters
function M.random_string()
  local function r(length)
    if length < 1 then
      return ""
    end

    local i = math.random(1, #random_charset)
    return r(length - 1) .. random_charset:sub(i, i)
  end

  return r(10)
end

---Get SQL statement at cursor position by analyzing semicolon boundaries
---@param bufnr integer buffer number
---@param cursor_row integer 0-indexed cursor row
---@return string|nil query the SQL statement at cursor, or nil if not found
function M.get_sql_statement_at_cursor(bufnr, cursor_row)
  local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
  if #lines == 0 then
    return nil
  end

  local all_text = table.concat(lines, "\n")
  if all_text == "" then
    return nil
  end

  -- Calculate cursor position in the full text
  local cursor_pos = 0
  for i = 0, cursor_row - 1 do
    if lines[i + 1] then
      cursor_pos = cursor_pos + #lines[i + 1] + 1 -- +1 for newline
    end
  end

  -- Add current line position up to cursor
  local current_line = lines[cursor_row + 1] or ""
  local cursor_col = vim.api.nvim_win_get_cursor(0)[2]
  cursor_pos = cursor_pos + math.min(cursor_col, #current_line)

  -- Find statement boundaries (semicolons) respecting quoted strings and comments
  local statements = {}
  local start_pos = 1
  local in_string = false
  local quote_char = nil
  local in_line_comment = false
  local in_block_comment = false
  local i = 1

  while i <= #all_text do
    local char = all_text:sub(i, i)
    local next_char = all_text:sub(i + 1, i + 1)
    local prev_char = all_text:sub(i - 1, i - 1)

    -- Handle newlines (reset line comments)
    if char == "\n" then
      in_line_comment = false
    end

    -- Handle line comments (-- style)
    if not in_string and not in_block_comment and char == "-" and next_char == "-" then
      in_line_comment = true
      i = i + 1 -- skip next char
    end

    -- Handle block comments (/* */ style)
    if not in_string and not in_line_comment and char == "/" and next_char == "*" then
      in_block_comment = true
      i = i + 1 -- skip next char
    elseif in_block_comment and char == "*" and next_char == "/" then
      in_block_comment = false
      i = i + 1 -- skip next char
    end

    -- Handle string literals (only if not in comments)
    if not in_line_comment and not in_block_comment then
      if (char == "'" or char == '"') and prev_char ~= "\\" then
        if not in_string then
          in_string = true
          quote_char = char
        elseif char == quote_char then
          in_string = false
          quote_char = nil
        end
      end
    end

    -- Found semicolon outside of string and comments
    if char == ";" and not in_string and not in_line_comment and not in_block_comment then
      local statement = all_text:sub(start_pos, i - 1)
      if statement:match("%S") then -- Has non-whitespace content
        table.insert(statements, {
          text = statement,
          start_pos = start_pos,
          end_pos = i - 1,
        })
      end
      start_pos = i + 1
    end

    i = i + 1
  end

  -- Add last statement if no trailing semicolon
  if start_pos <= #all_text then
    local statement = all_text:sub(start_pos)
    if statement:match("%S") then -- Has non-whitespace
      table.insert(statements, {
        text = statement,
        start_pos = start_pos,
        end_pos = #all_text,
      })
    end
  end

  -- Find which statement contains the cursor
  for _, stmt in ipairs(statements) do
    if cursor_pos >= stmt.start_pos and cursor_pos <= stmt.end_pos then
      return vim.trim(stmt.text)
    end
  end

  -- If no statement found, return the entire buffer as fallback
  if #statements == 0 then
    return vim.trim(all_text)
  end

  return nil
end

---Get the line boundaries of SQL statement at cursor position
---@param bufnr integer buffer number
---@param cursor_row integer 0-indexed cursor row
---@return integer|nil start_line 1-indexed start line
---@return integer|nil end_line 1-indexed end line
function M.get_sql_statement_bounds(bufnr, cursor_row)
  local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
  if #lines == 0 then
    return nil, nil
  end

  local all_text = table.concat(lines, "\n")
  if all_text == "" then
    return nil, nil
  end

  -- Calculate cursor position in the full text
  local cursor_pos = 0
  for i = 0, cursor_row - 1 do
    if lines[i + 1] then
      cursor_pos = cursor_pos + #lines[i + 1] + 1
    end
  end
  local current_line = lines[cursor_row + 1] or ""
  local cursor_col = vim.api.nvim_win_get_cursor(0)[2]
  cursor_pos = cursor_pos + math.min(cursor_col, #current_line)

  -- Find statement boundaries in character positions
  local statements = {}
  local start_pos = 1
  local in_string = false
  local quote_char = nil
  local in_line_comment = false
  local in_block_comment = false
  local i = 1

  while i <= #all_text do
    local char = all_text:sub(i, i)
    local next_char = all_text:sub(i + 1, i + 1)
    local prev_char = all_text:sub(i - 1, i - 1)

    if char == "\n" then
      in_line_comment = false
    end

    if not in_string and not in_block_comment and char == "-" and next_char == "-" then
      in_line_comment = true
      i = i + 1
    end

    if not in_string and not in_line_comment and char == "/" and next_char == "*" then
      in_block_comment = true
      i = i + 1
    elseif in_block_comment and char == "*" and next_char == "/" then
      in_block_comment = false
      i = i + 1
    end

    if not in_line_comment and not in_block_comment then
      if (char == "'" or char == '"') and prev_char ~= "\\" then
        if not in_string then
          in_string = true
          quote_char = char
        elseif char == quote_char then
          in_string = false
          quote_char = nil
        end
      end
    end

    if char == ";" and not in_string and not in_line_comment and not in_block_comment then
      local statement = all_text:sub(start_pos, i - 1)
      if statement:match("%S") then
        table.insert(statements, {
          start_pos = start_pos,
          end_pos = i - 1,
        })
      end
      start_pos = i + 1
    end

    i = i + 1
  end

  if start_pos <= #all_text then
    local statement = all_text:sub(start_pos)
    if statement:match("%S") then
      table.insert(statements, {
        start_pos = start_pos,
        end_pos = #all_text,
      })
    end
  end

  -- Find which statement contains the cursor and convert to line numbers
  for _, stmt in ipairs(statements) do
    if cursor_pos >= stmt.start_pos and cursor_pos <= stmt.end_pos then
      -- Convert character positions to line numbers
      local start_line = 1
      local end_line = 1
      local char_count = 0

      for line_num, line in ipairs(lines) do
        local line_start = char_count + 1
        local line_end = char_count + #line

        if stmt.start_pos >= line_start and stmt.start_pos <= line_end + 1 then
          start_line = line_num
        end
        if stmt.end_pos >= line_start and stmt.end_pos <= line_end + 1 then
          end_line = line_num
          break
        end

        char_count = char_count + #line + 1 -- +1 for newline
      end

      return start_line, end_line
    end
  end

  return nil, nil
end

---Visually select the SQL statement at cursor position
function M.select_sql_statement_at_cursor()
  local bufnr = vim.api.nvim_get_current_buf()
  local cursor_pos = vim.api.nvim_win_get_cursor(0)
  local row = cursor_pos[1] - 1 -- Convert to 0-indexed

  local start_line, end_line = M.get_sql_statement_bounds(bufnr, row)

  if not start_line or not end_line then
    vim.notify("No SQL statement found at cursor", vim.log.levels.WARN)
    return
  end

  -- Move to start of statement and select it
  vim.api.nvim_win_set_cursor(0, { start_line, 0 })
  vim.cmd("normal! V")
  vim.api.nvim_win_set_cursor(0, { end_line, 0 })
  vim.cmd("normal! $")
end

---Select inner SQL statement text object (for vim text objects)
function M.select_inner_sql_statement()
  local bufnr = vim.api.nvim_get_current_buf()
  local cursor_pos = vim.api.nvim_win_get_cursor(0)
  local row = cursor_pos[1] - 1

  local statement = M.get_sql_statement_at_cursor(bufnr, row)
  if not statement then
    return
  end

  local start_line, end_line = M.get_sql_statement_bounds(bufnr, row)
  if not start_line or not end_line then
    return
  end

  -- For text objects, we select character-wise from first non-whitespace to last non-whitespace
  local lines = vim.api.nvim_buf_get_lines(bufnr, start_line - 1, end_line, false)
  local first_non_ws = nil
  local last_non_ws = nil

  -- Find first non-whitespace character
  for line_idx, line in ipairs(lines) do
    local match_start = line:find("%S")
    if match_start then
      first_non_ws = { start_line - 1 + line_idx - 1, match_start - 1 }
      break
    end
  end

  -- Find last non-whitespace character
  for line_idx = #lines, 1, -1 do
    local line = lines[line_idx]
    local match_end = line:find("%S[^%S]*$")
    if match_end then
      last_non_ws = { start_line - 1 + line_idx - 1, line:len() - 1 }
      break
    end
  end

  if first_non_ws and last_non_ws then
    vim.api.nvim_win_set_cursor(0, { first_non_ws[1] + 1, first_non_ws[2] })
    vim.cmd("normal! v")
    vim.api.nvim_win_set_cursor(0, { last_non_ws[1] + 1, last_non_ws[2] })
  end
end

---Select around SQL statement text object (for vim text objects)
function M.select_around_sql_statement()
  local bufnr = vim.api.nvim_get_current_buf()
  local cursor_pos = vim.api.nvim_win_get_cursor(0)
  local row = cursor_pos[1] - 1

  local start_line, end_line = M.get_sql_statement_bounds(bufnr, row)
  if not start_line or not end_line then
    return
  end

  -- For "around", include the semicolon and any trailing whitespace
  local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
  local end_col = 0

  -- Check if there's a semicolon after the statement
  if end_line <= #lines then
    local end_line_text = lines[end_line]
    local semicolon_pos = end_line_text:find(";")
    if semicolon_pos then
      end_col = semicolon_pos
    else
      end_col = end_line_text:len()
    end
  end

  vim.api.nvim_win_set_cursor(0, { start_line, 0 })
  vim.cmd("normal! v")
  vim.api.nvim_win_set_cursor(0, { end_line, end_col })
end

---Setup SQL text objects for DBEE buffers
function M.setup_sql_text_objects()
  vim.api.nvim_create_autocmd("FileType", {
    pattern = { "sql", "mysql", "plsql" },
    callback = function(ev)
      local opts = { buffer = ev.buf, silent = true }

      -- Inner SQL statement text object
      vim.keymap.set({ "o", "x" }, "is", function()
        M.select_inner_sql_statement()
      end, vim.tbl_extend("force", opts, { desc = "Inner SQL statement" }))

      -- Around SQL statement text object
      vim.keymap.set({ "o", "x" }, "as", function()
        M.select_around_sql_statement()
      end, vim.tbl_extend("force", opts, { desc = "Around SQL statement" }))
    end,
  })
end

return M
