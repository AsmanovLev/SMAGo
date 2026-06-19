import os
p = os.path.join(os.path.dirname(os.path.abspath('.')), 'src', 'tools.go')
with open(p) as f:
    c = f.read()
print(len(c), 'chars')
