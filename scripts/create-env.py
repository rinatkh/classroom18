"""Локальная конфигурация создаётся один раз с независимыми случайными секретами."""
from pathlib import Path
import os,secrets
path=Path('.env')
try:
 fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
except FileExistsError:
 raise SystemExit(0)
text=Path('.env.example').read_text()
for key in ('POSTGRES_PASSWORD','JWT_SECRET'):
 lines=text.splitlines()
 text='\n'.join(key+'='+secrets.token_hex(32) if line.startswith(key+'=') else line for line in lines)+'\n'
with os.fdopen(fd,'w') as out:out.write(text)
