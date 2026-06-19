import os

path = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'src', 'agent.go')
with open(path, 'r', encoding='utf-8') as f:
    c = f.read()

# 1. Add helper methods
c = c.replace(
    'func (a *Agent) typing(chatID int64) { _ = a.tg.Typing(chatID) }',
    'func (a *Agent) typing(chatID int64)       { _ = a.tg.Typing(chatID) }\nfunc (a *Agent) chooseSticker(chatID int64) { _ = a.tg.SendChatAction(chatID, "choose_sticker") }\nfunc (a *Agent) playing(chatID int64)       { _ = a.tg.SendChatAction(chatID, "playing") }'
)

# 2. Before LLM call -> choose_sticker (thinking)
c = c.replace(
    'a.typing(chatID)\n\t\tstepStart := time.Now()',
    'a.chooseSticker(chatID)\n\t\tstepStart := time.Now()'
)

# 3. In tool execution loop -> playing (tool call)
lines = c.split('\n')
new_lines = []
in_tool_loop = False
brace_depth = 0
for line in lines:
    if 'for _, tc := range resp.ToolCalls {' in line:
        in_tool_loop = True
        brace_depth = 0
    if in_tool_loop:
        brace_depth += line.count('{') - line.count('}')
        if 'a.typing(chatID)' in line and brace_depth > 0:
            line = line.replace('a.typing(chatID)', 'a.playing(chatID)')
        if brace_depth <= 0 and line.strip() == '}':
            in_tool_loop = False
    new_lines.append(line)

c = '\n'.join(new_lines)

with open(path, 'w', encoding='utf-8') as f:
    f.write(c)
print('OK')
