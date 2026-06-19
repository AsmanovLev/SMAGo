import os

base = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'src')

# Fix resume.go
path = os.path.join(base, 'resume.go')
with open(path, 'r', encoding='utf-8') as f:
    c = f.read()
c = c.replace('\r\n', '\n')
c = c.replace('Version string `json:"version"`\n}', 'Version string `json:"version"`\n\tSteps   int    `json:"steps,omitempty"`\n}')
c = c.replace('func saveResumeMarker(chatID int64, version string) error {', 'func saveResumeMarker(chatID int64, version string, steps int) error {')
c = c.replace('m := ResumeMarker{ChatID: chatID, Version: version}', 'm := ResumeMarker{ChatID: chatID, Version: version, Steps: steps}')
with open(path, 'w', encoding='utf-8', newline='\n') as f:
    f.write(c)
print('resume.go OK')

# Fix resume_check.go
path2 = os.path.join(base, 'resume_check.go')
with open(path2, 'r', encoding='utf-8') as f:
    c2 = f.read()
c2 = c2.replace('\r\n', '\n')
c2 = c2.replace(
    'Upgrade to %s successful. Continue your previous task.',
    'Upgrade to %s successful. Continue your previous task. (Step %d)',
    1  # only first occurrence (in sess.Append)
)
c2 = c2.replace(
    'msg := fmt.Sprintf("Upgrade to %s successful. Continue your previous task.", m.Version)',
    'msg := fmt.Sprintf("Upgrade to %s successful. Continue your previous task. (Step %d)", m.Version, m.Steps)'
)
with open(path2, 'w', encoding='utf-8', newline='\n') as f:
    f.write(c2)
print('resume_check.go OK')
