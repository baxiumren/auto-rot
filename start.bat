@echo off
if not exist bot.exe (
    echo bot.exe tidak ditemukan! Jalankan build.bat dulu.
    pause
    exit /b 1
)
if not exist .env (
    echo .env tidak ditemukan! Copy .env.example ke .env dan isi dulu.
    pause
    exit /b 1
)
echo Starting bot...
bot.exe
pause
