with open('../src/agent.go', 'r', encoding='utf-8') as f:
    c = f.read()
idx = c.find('case text == "/dc"')
print(f"idx={idx}")
if idx >= 0:
    print(repr(c[idx:idx+400]))
