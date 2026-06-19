import os

path = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'src', 'agent.go')
with open(path, 'r', encoding='utf-8') as f:
    lines = f.readlines()

new_lines = []
i = 0
while i < len(lines):
    line = lines[i]
    
    # Replace a.chooseSticker(chatID) with thinking := a.beginThinking(chatID)
    if 'a.chooseSticker(chatID)' in line:
        new_lines.append(line.replace('a.chooseSticker(chatID)', 'thinking := a.beginThinking(chatID)'))
        i += 1
        continue
    
    # After LLM call, add thinking.stop()
    if 'stepDur := time.Since(stepStart)' in line:
        new_lines.append(line)
        new_lines.append(T + 'thinking.stop()' + chr(10))
        i += 1
        continue
    
    # Replace a.playing(chatID) with toolLoop := a.beginToolCall(chatID)  
    if 'a.playing(chatID)' in line:
        new_lines.append(line.replace('a.playing(chatID)', 'toolLoop := a.beginToolCall(chatID)'))
        i += 1
        continue
    
    # After tool loop, add toolLoop.stop() before recordStep
    if 'a.recordStep(chatID, i+1, maxSteps, usage, stepDur, toolLines, -1, resp.Content)' in line:
        new_lines.append(T + 'toolLoop.stop()' + chr(10))
        new_lines.append(line)
        i += 1
        continue
    
    new_lines.append(line)
    i += 1

with open(path, 'w', encoding='utf-8') as f:
    f.writelines(new_lines)
print('OK')
