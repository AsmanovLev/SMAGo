import os, sys

path = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'src', 'agent.go')
with open(path, 'rb') as f:
    raw = f.read()

# Normalize CRLF to LF
raw = raw.replace(b'\r\n', b'\n')
text = raw.decode('utf-8')

# 1. typing -> beginThinking before LLM
text = text.replace('a.typing(chatID)\n\t\tstepStart := time.Now()', 'thinking := a.beginThinking(chatID)\n\t\tstepStart := time.Now()')

# 2. Add thinking.stop() after stepDur
text = text.replace('stepDur := time.Since(stepStart)\n', 'stepDur := time.Since(stepStart)\n\t\tthinking.stop()\n')

# 3. playing -> beginToolCall
text = text.replace('\t\t\ta.playing(chatID)\n', '\t\t\ttoolLoop := a.beginToolCall(chatID)\n')

# 4. toolLoop.stop() before recordStep
text = text.replace(
    '\t\ta.recordStep(chatID, i+1, maxSteps, usage, stepDur, toolLines, -1, resp.Content)\n',
    '\t\ttoolLoop.stop()\n\t\ta.recordStep(chatID, i+1, maxSteps, usage, stepDur, toolLines, -1, resp.Content)\n'
)

# Verify
for needle, expected in [('beginThinking', 1), ('thinking.stop()', 1), ('beginToolCall', 1), ('toolLoop.stop()', 1)]:
    count = text.count(needle)
    if count != expected:
        print(f'FAIL: {needle} count={count}, expected={expected}')
        sys.exit(1)

with open(path, 'w', encoding='utf-8', newline='\n') as f:
    f.write(text)
print('OK')
