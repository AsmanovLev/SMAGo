import smtplib, ssl, email.utils

with open('../data/smago.key', 'r') as f:
    key = f.read()

boundary = '----=_Part_SMAGo'
parts = [
    'MIME-Version: 1.0',
    'From: sma.go@ro.ru',
    'To: delt.er@bk.ru',
    'Subject: SMAGo key exchange',
    'Chat-Version: 1.0',
    'Chat-User-Agent: SMAGo/1.0',
    'Date: ' + email.utils.formatdate(localtime=True),
    'Content-Type: multipart/mixed; boundary="' + boundary + '"',
    '',
    '--' + boundary,
    'Content-Type: text/plain; charset=utf-8',
    '',
    'Hi! I am SMAGo. This is my PGP key.',
    '',
    '--' + boundary,
    'Content-Type: application/pgp-keys; name="autocrypt.asc"',
    'Content-Disposition: attachment; filename="autocrypt.asc"',
    'Content-Transfer-Encoding: 7bit',
    '',
    key,
    '',
    '--' + boundary + '--',
]

msg = '\r\n'.join(parts)

ctx = ssl.create_default_context()
with smtplib.SMTP_SSL('smtp.rambler.ru', 465, context=ctx, timeout=30) as s:
    s.login('sma.go@ro.ru', '2ZdXFM*yot7Hzz^t')
    s.sendmail('sma.go@ro.ru', ['delt.er@bk.ru'], msg)
    print('sent key exchange email!')
